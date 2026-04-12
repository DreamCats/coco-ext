package prd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
)

// ExecuteCodeForRepo 在指定 repo 中执行单仓 prd code 主流程。
func ExecuteCodeForRepo(repoRoot, taskID, branchName, repoID string, maxRetries int, onChunk func(string), onTool func(generator.ToolEvent)) (_ *CodeResultReport, retErr error) {
	startedAt := time.Now()
	var logBuffer strings.Builder
	taskDir := ""
	repoBindingID := ""
	worktreeDir := ""
	startedCoding := false

	defer func() {
		if strings.TrimSpace(taskDir) == "" {
			return
		}
		if startedCoding && retErr != nil && strings.TrimSpace(repoBindingID) != "" {
			_ = MarkRepoBindingFailed(taskDir, repoBindingID, branchName, worktreeDir)
		}
		appendCodeLogLine(&logBuffer, "=== CODE LOG ===")
		appendCodeLogLine(&logBuffer, fmt.Sprintf("task_id: %s", taskID))
		appendCodeLogLine(&logBuffer, fmt.Sprintf("branch: %s", branchName))
		appendCodeLogLine(&logBuffer, fmt.Sprintf("started_at: %s", startedAt.Format(time.RFC3339)))
		if retErr != nil {
			appendCodeLogLine(&logBuffer, fmt.Sprintf("result: error (%v)", retErr))
		} else {
			appendCodeLogLine(&logBuffer, "result: ok")
		}
		appendCodeLogLine(&logBuffer, fmt.Sprintf("finished_at: %s", time.Now().Format(time.RFC3339)))
		appendCodeLogLine(&logBuffer, "=== END ===")

		logPath := filepath.Join(taskDir, "code.log")
		if err := os.WriteFile(logPath, []byte(logBuffer.String()), 0644); err != nil && retErr == nil {
			retErr = fmt.Errorf("写入 code.log 失败: %w", err)
		}
		if err := WriteRepoCodeLog(taskDir, repoBindingID, logBuffer.String()); err != nil && retErr == nil {
			retErr = err
		}
	}()

	appendCodeLogLine(&logBuffer, "=== SETUP ===")
	appendCodeLogLine(&logBuffer, fmt.Sprintf("repo_root: %s", repoRoot))

	var err error
	taskDir, err = PrepareAgentCode(repoRoot, taskID)
	if err != nil {
		appendCodeLogLine(&logBuffer, fmt.Sprintf("prepare_agent_code_error: %v", err))
		return nil, err
	}
	appendCodeLogLine(&logBuffer, fmt.Sprintf("task_dir: %s", taskDir))

	repoBinding, err := ResolveTaskRepo(taskDir, repoRoot, repoID)
	if err != nil {
		appendCodeLogLine(&logBuffer, fmt.Sprintf("resolve_task_repo_error: %v", err))
		return nil, err
	}
	repoBindingID = repoBinding.ID
	appendCodeLogLine(&logBuffer, fmt.Sprintf("repo_id: %s", repoBinding.ID))
	appendCodeLogLine(&logBuffer, fmt.Sprintf("repo_path: %s", repoBinding.Path))

	workspace, err := PrepareCodeWorkspace(repoRoot, taskID, branchName)
	if err != nil {
		appendCodeLogLine(&logBuffer, fmt.Sprintf("prepare_code_workspace_error: %v", err))
		return nil, err
	}
	worktreeDir = workspace.WorktreeDir
	appendCodeLogLine(&logBuffer, fmt.Sprintf("worktree: %s", workspace.WorktreeDir))

	if err := StartCodingRepoBinding(taskDir, repoBinding.ID, branchName, workspace.WorktreeDir); err != nil {
		appendCodeLogLine(&logBuffer, fmt.Sprintf("start_coding_status_error: %v", err))
		return nil, err
	}
	startedCoding = true

	agent, err := generator.NewAgent(workspace.WorktreeDir)
	if err != nil {
		appendCodeLogLine(&logBuffer, fmt.Sprintf("start_agent_error: %v", err))
		return nil, fmt.Errorf("启动 AI agent 失败: %w", err)
	}
	defer agent.Close()

	appendCodeLogLine(&logBuffer, "=== AGENT OUTPUT ===")
	wrappedOnChunk := func(chunk string) {
		logBuffer.WriteString(chunk)
		if onChunk != nil {
			onChunk(chunk)
		}
	}
	wrappedOnTool := func(event generator.ToolEvent) {
		appendCodeLogLine(&logBuffer, formatCodeToolEvent(event))
		if onTool != nil {
			onTool(event)
		}
	}

	result, err := GenerateCodeWithAgent(agent, workspace.WorktreeTaskDir, taskDir, time.Now(), wrappedOnChunk, wrappedOnTool)
	if err != nil {
		appendCodeLogLine(&logBuffer, fmt.Sprintf("generate_code_with_agent_error: %v", err))
		return nil, err
	}
	appendCodeLogLine(&logBuffer, "")

	if !result.BuildOK && maxRetries > 0 {
		changedPkgs := extractChangedPackages(workspace.WorktreeDir)
		if len(changedPkgs) == 0 {
			appendCodeLogLine(&logBuffer, "retry: 未检测到改动文件，跳过重试")
		} else {
			for attempt := 1; attempt <= maxRetries; attempt++ {
				buildOutput, buildErr := runBuildPackages(workspace.WorktreeDir, changedPkgs)
				if buildErr == nil {
					appendCodeLogLine(&logBuffer, fmt.Sprintf("retry_%d: build passed", attempt))
					result.BuildOK = true
					break
				}

				appendCodeLogLine(&logBuffer, fmt.Sprintf("=== RETRY %d/%d BUILD OUTPUT ===", attempt, maxRetries))
				logBuffer.WriteString(buildOutput)
				if !strings.HasSuffix(buildOutput, "\n") {
					logBuffer.WriteString("\n")
				}

				followUp := fmt.Sprintf("编译失败，请修复以下错误：\n%s\n修复后重新运行 go build 验证，然后输出 === CODE RESULT ===", buildOutput)
				appendCodeLogLine(&logBuffer, fmt.Sprintf("=== RETRY %d/%d AGENT OUTPUT ===", attempt, maxRetries))
				reply, retryErr := agent.PromptWithTools(followUp, config.CodePromptTimeout, wrappedOnChunk, wrappedOnTool)
				if onChunk != nil {
					onChunk("\n")
				}
				if retryErr != nil && reply == "" {
					appendCodeLogLine(&logBuffer, fmt.Sprintf("retry_%d_error: %v", attempt, retryErr))
					break
				}

				cr := ParseCodeResult(reply)
				if cr != nil {
					result.AgentReply += "\n" + reply
					result.BuildOK = cr.BuildOK
					if len(cr.Files) > 0 {
						result.FilesChanged = cr.Files
					}
					if cr.Summary != "" {
						result.Summary = cr.Summary
					}
					continue
				}

				if _, buildErr2 := runBuildPackages(workspace.WorktreeDir, changedPkgs); buildErr2 == nil {
					appendCodeLogLine(&logBuffer, fmt.Sprintf("retry_%d_postcheck: build passed", attempt))
					result.BuildOK = true
				}
			}
		}
	}

	changedFiles := result.FilesChanged
	if len(changedFiles) == 0 {
		changedFiles = collectCodeChanges(workspace.WorktreeDir)
	} else {
		gitChanged := collectCodeChanges(workspace.WorktreeDir)
		if len(gitChanged) > 0 {
			changedFiles = mergeCodeFileLists(changedFiles, gitChanged)
		}
	}

	commitHash := ""
	if result.BuildOK && len(changedFiles) > 0 {
		hash, commitErr := commitCodeChanges(workspace.WorktreeDir, taskID, changedFiles, result.Summary)
		if commitErr != nil {
			appendCodeLogLine(&logBuffer, fmt.Sprintf("auto_commit_error: %v", commitErr))
		} else {
			appendCodeLogLine(&logBuffer, fmt.Sprintf("auto_commit: %s", hash))
			commitHash = hash
			patch, diffErr := readCommitPatch(workspace.WorktreeDir)
			if diffErr != nil {
				appendCodeLogLine(&logBuffer, fmt.Sprintf("write_diff_patch_error: %v", diffErr))
			} else if writeErr := WriteRepoDiffArtifacts(taskDir, repoBinding.ID, branchName, hash, changedFiles, patch); writeErr != nil {
				appendCodeLogLine(&logBuffer, fmt.Sprintf("write_diff_patch_error: %v", writeErr))
			} else {
				appendCodeLogLine(&logBuffer, fmt.Sprintf("diff_patch: %s", filepath.Join(taskDir, "diffs", repoBinding.ID+".patch")))
			}
		}
	}

	report := &CodeResultReport{
		Status:       "success",
		TaskID:       taskID,
		RepoID:       repoBinding.ID,
		RepoPath:     repoRoot,
		Branch:       branchName,
		Worktree:     workspace.WorktreeDir,
		Commit:       commitHash,
		BuildOK:      result.BuildOK,
		FilesWritten: changedFiles,
		Log:          filepath.Join(taskDir, "code.log"),
		StartedAt:    startedAt.Format(time.RFC3339),
		FinishedAt:   time.Now().Format(time.RFC3339),
	}
	if !result.BuildOK {
		report.Status = "build_unknown"
	}

	if err := WriteCodeResultReport(taskDir, *report); err != nil {
		appendCodeLogLine(&logBuffer, fmt.Sprintf("write_code_result_error: %v", err))
		return nil, fmt.Errorf("写入 code-result.json 失败: %w", err)
	}
	appendCodeLogLine(&logBuffer, "=== RESULT ===")
	appendCodeLogLine(&logBuffer, fmt.Sprintf("build_ok: %t", report.BuildOK))
	appendCodeLogLine(&logBuffer, fmt.Sprintf("files_written: %s", strings.Join(report.FilesWritten, ", ")))
	appendCodeLogLine(&logBuffer, fmt.Sprintf("commit: %s", report.Commit))
	appendCodeLogLine(&logBuffer, fmt.Sprintf("code_result_json: %s", filepath.Join(taskDir, "code-result.json")))

	return report, nil
}

func appendCodeLogLine(b *strings.Builder, line string) {
	b.WriteString(line)
	b.WriteString("\n")
}

func formatCodeToolEvent(event generator.ToolEvent) string {
	base := fmt.Sprintf("[tool] status=%s kind=%s title=%s", event.Status, event.Kind, event.Title)
	if len(event.RawInput) == 0 {
		return base
	}
	input := strings.TrimSpace(string(event.RawInput))
	if input == "" {
		return base
	}
	return base + " input=" + input
}

func readCommitPatch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "show", "--format=medium", "--patch", "--stat=0", "--no-ext-diff", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取 commit patch 失败: %s\n%s", err, string(output))
	}
	return string(output), nil
}

func collectCodeChanges(repoRoot string) []string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 3 {
			continue
		}
		file := strings.TrimSpace(line[2:])
		if strings.HasPrefix(file, ".livecoding/") {
			continue
		}
		if file != "" {
			files = append(files, file)
		}
	}
	return files
}

func mergeCodeFileLists(agentFiles, gitFiles []string) []string {
	seen := make(map[string]bool)
	var merged []string
	for _, f := range agentFiles {
		if !seen[f] {
			seen[f] = true
			merged = append(merged, f)
		}
	}
	for _, f := range gitFiles {
		if !seen[f] {
			seen[f] = true
			merged = append(merged, f)
		}
	}
	return merged
}

func extractChangedPackages(repoRoot string) []string {
	files := collectCodeChanges(repoRoot)
	seen := make(map[string]bool)
	var pkgs []string
	for _, f := range files {
		dir := filepath.Dir(f)
		if dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		pkgs = append(pkgs, "./"+dir+"/...")
	}
	return pkgs
}

func runBuildPackages(repoRoot string, pkgs []string) (string, error) {
	args := append([]string{"build"}, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func commitCodeChanges(repoRoot, taskID string, files []string, summary string) (string, error) {
	args := append([]string{"add"}, files...)
	addCmd := exec.Command("git", args...)
	addCmd.Dir = repoRoot
	if output, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add 失败: %s\n%s", err, string(output))
	}

	msg := fmt.Sprintf("feat(prd): auto-generated code\n\ntask_id: %s\ngenerated by: coco-ext prd code", taskID)
	if summary != "" {
		msg = fmt.Sprintf("feat(prd): %s\n\ntask_id: %s\ngenerated by: coco-ext prd code", summary, taskID)
	}
	commitCmd := exec.Command("git", "commit", "-m", msg)
	commitCmd.Dir = repoRoot
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit 失败: %s\n%s", err, string(output))
	}

	hashCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	hashCmd.Dir = repoRoot
	hashOutput, err := hashCmd.Output()
	if err != nil {
		return "unknown", nil
	}
	return strings.TrimSpace(string(hashOutput)), nil
}
