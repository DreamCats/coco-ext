package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var (
	prdCodeTaskID   string
	prdCodeBranch   string
	prdCodeRepoID   string
	prdCodeAllRepos bool
	prdCodeMaxRetry int
)

var prdCodeCmd = &cobra.Command{
	Use:   "code",
	Short: "基于 plan.md 自动生成实现代码",
	Long:  "读取 plan.md 和 design.md，创建隔离 worktree，启动 yolo agent 在 worktree 中自主读写文件和编译，成功后自动 commit 到 prd_<task_id> 分支。",
	RunE:  runPRDCode,
}

func init() {
	prdCmd.AddCommand(prdCodeCmd)
	prdCodeCmd.Flags().StringVar(&prdCodeTaskID, "task", "", "指定 task id；默认读取最近一个 task")
	prdCodeCmd.Flags().StringVar(&prdCodeBranch, "branch", "", "自定义分支名；默认 prd_<task_id>")
	prdCodeCmd.Flags().StringVar(&prdCodeRepoID, "repo", "", "指定当前要执行 code 的 repo_id；默认使用当前仓库目录名")
	prdCodeCmd.Flags().BoolVar(&prdCodeAllRepos, "all-repos", false, "按 task 绑定顺序依次执行所有 repo 的 code；失败即停")
	prdCodeCmd.Flags().IntVar(&prdCodeMaxRetry, "max-retries", 2, "编译失败时最大重试次数")
}

func runPRDCode(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	taskID, err := prd.ResolveTaskID(repoRoot, prdCodeTaskID)
	if err != nil {
		return err
	}
	if prdCodeAllRepos && strings.TrimSpace(prdCodeRepoID) != "" {
		return fmt.Errorf("--repo 与 --all-repos 互斥，请选择其中一种模式")
	}

	branchName := prdCodeBranch
	if branchName == "" {
		branchName = buildPRDBranchName(taskID)
	}

	startedAt := time.Now()
	color.Cyan("🤖 PRD Code")
	color.Cyan("   task_id: %s", taskID)
	if prdCodeAllRepos {
		color.Cyan("   repos: all")
	} else if prdCodeRepoID != "" {
		color.Cyan("   repo: %s", prdCodeRepoID)
	}
	color.Cyan("   branch: %s", branchName)
	if prdCodeAllRepos {
		return runPRDCodeAllRepos(repoRoot, taskID, branchName, startedAt)
	}
	report, err := executePRDCodeForRepo(repoRoot, taskID, branchName, prdCodeRepoID, prdCodeMaxRetry, func(chunk string) {
		fmt.Print(chunk)
	}, func(event generator.ToolEvent) {
		renderToolEvent(event)
	})
	fmt.Println()
	if err != nil {
		return err
	}

	renderSingleCodeReport(report, branchName, startedAt)
	return nil
}

func runPRDCodeAllRepos(repoRoot, taskID, branchName string, startedAt time.Time) error {
	task, err := prd.LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return err
	}
	if task.Repos == nil || len(task.Repos.Repos) == 0 {
		return fmt.Errorf("task 未绑定任何 repo")
	}

	reports := make([]*prd.CodeResultReport, 0, len(task.Repos.Repos))
	for idx, repo := range task.Repos.Repos {
		color.Cyan("   [%d/%d] repo: %s", idx+1, len(task.Repos.Repos), repo.ID)
		report, execErr := executePRDCodeForRepo(repo.Path, taskID, branchName, repo.ID, prdCodeMaxRetry, func(chunk string) {
			fmt.Print(chunk)
		}, func(event generator.ToolEvent) {
			renderToolEvent(event)
		})
		fmt.Println()
		if execErr != nil {
			renderCodeBatchSummary(reports, branchName, startedAt)
			return fmt.Errorf("repo %s 执行失败: %w", repo.ID, execErr)
		}
		reports = append(reports, report)
		renderCodeRepoProgress(report)
	}

	renderCodeBatchSummary(reports, branchName, startedAt)
	return nil
}

func executePRDCode(repoRoot, taskID, branchName string, maxRetries int, onChunk func(string), onTool func(generator.ToolEvent)) (*prd.CodeResultReport, error) {
	return executePRDCodeForRepo(repoRoot, taskID, branchName, prdCodeRepoID, maxRetries, onChunk, onTool)
}

func executePRDCodeForRepo(repoRoot, taskID, branchName, repoID string, maxRetries int, onChunk func(string), onTool func(generator.ToolEvent)) (_ *prd.CodeResultReport, retErr error) {
	startedAt := time.Now()
	var logBuffer strings.Builder
	taskDir := ""

	defer func() {
		if strings.TrimSpace(taskDir) == "" {
			return
		}
		codeAppendLogLine(&logBuffer, "=== CODE LOG ===")
		codeAppendLogLine(&logBuffer, fmt.Sprintf("task_id: %s", taskID))
		codeAppendLogLine(&logBuffer, fmt.Sprintf("branch: %s", branchName))
		codeAppendLogLine(&logBuffer, fmt.Sprintf("started_at: %s", startedAt.Format(time.RFC3339)))
		if retErr != nil {
			codeAppendLogLine(&logBuffer, fmt.Sprintf("result: error (%v)", retErr))
		} else {
			codeAppendLogLine(&logBuffer, "result: ok")
		}
		codeAppendLogLine(&logBuffer, fmt.Sprintf("finished_at: %s", time.Now().Format(time.RFC3339)))
		codeAppendLogLine(&logBuffer, "=== END ===")

		logPath := filepath.Join(taskDir, "code.log")
		if err := os.WriteFile(logPath, []byte(logBuffer.String()), 0644); err != nil && retErr == nil {
			retErr = fmt.Errorf("写入 code.log 失败: %w", err)
		}
	}()

	codeAppendLogLine(&logBuffer, "=== SETUP ===")
	codeAppendLogLine(&logBuffer, fmt.Sprintf("repo_root: %s", repoRoot))

	var err error
	taskDir, err = prd.PrepareAgentCode(repoRoot, taskID)
	if err != nil {
		codeAppendLogLine(&logBuffer, fmt.Sprintf("prepare_agent_code_error: %v", err))
		return nil, err
	}
	codeAppendLogLine(&logBuffer, fmt.Sprintf("task_dir: %s", taskDir))
	repoBinding, err := prd.ResolveTaskRepo(taskDir, repoRoot, repoID)
	if err != nil {
		codeAppendLogLine(&logBuffer, fmt.Sprintf("resolve_task_repo_error: %v", err))
		return nil, err
	}
	codeAppendLogLine(&logBuffer, fmt.Sprintf("repo_id: %s", repoBinding.ID))
	codeAppendLogLine(&logBuffer, fmt.Sprintf("repo_path: %s", repoBinding.Path))

	workspace, err := prd.PrepareCodeWorkspace(repoRoot, taskID, branchName)
	if err != nil {
		codeAppendLogLine(&logBuffer, fmt.Sprintf("prepare_code_workspace_error: %v", err))
		return nil, err
	}
	codeAppendLogLine(&logBuffer, fmt.Sprintf("worktree: %s", workspace.WorktreeDir))

	agent, err := generator.NewAgent(workspace.WorktreeDir)
	if err != nil {
		codeAppendLogLine(&logBuffer, fmt.Sprintf("start_agent_error: %v", err))
		return nil, fmt.Errorf("启动 AI agent 失败: %w", err)
	}
	defer agent.Close()

	codeAppendLogLine(&logBuffer, "=== AGENT OUTPUT ===")
	wrappedOnChunk := func(chunk string) {
		logBuffer.WriteString(chunk)
		if onChunk != nil {
			onChunk(chunk)
		}
	}
	wrappedOnTool := func(event generator.ToolEvent) {
		codeAppendLogLine(&logBuffer, codeFormatToolEvent(event))
		if onTool != nil {
			onTool(event)
		}
	}

	result, err := prd.GenerateCodeWithAgent(agent, workspace.WorktreeTaskDir, taskDir, time.Now(), wrappedOnChunk, wrappedOnTool)
	if err != nil {
		codeAppendLogLine(&logBuffer, fmt.Sprintf("generate_code_with_agent_error: %v", err))
		return nil, err
	}
	codeAppendLogLine(&logBuffer, "")

	if !result.BuildOK && maxRetries > 0 {
		changedPkgs := codeExtractChangedPackages(workspace.WorktreeDir)
		if len(changedPkgs) == 0 {
			codeAppendLogLine(&logBuffer, "retry: 未检测到改动文件，跳过重试")
			color.Yellow("      未检测到改动文件，跳过重试")
		} else {
			for attempt := 1; attempt <= maxRetries; attempt++ {
				buildOutput, buildErr := codeRunBuildPackages(workspace.WorktreeDir, changedPkgs)
				if buildErr == nil {
					codeAppendLogLine(&logBuffer, fmt.Sprintf("retry_%d: build passed", attempt))
					result.BuildOK = true
					color.Green("      重试 %d/%d: 编译通过 ✓", attempt, maxRetries)
					break
				}

				codeAppendLogLine(&logBuffer, fmt.Sprintf("=== RETRY %d/%d BUILD OUTPUT ===", attempt, maxRetries))
				logBuffer.WriteString(buildOutput)
				if !strings.HasSuffix(buildOutput, "\n") {
					logBuffer.WriteString("\n")
				}
				color.Yellow("      重试 %d/%d: 编译失败，正在修复...", attempt, maxRetries)
				followUp := fmt.Sprintf("编译失败，请修复以下错误：\n%s\n修复后重新运行 go build 验证，然后输出 === CODE RESULT ===", buildOutput)
				codeAppendLogLine(&logBuffer, fmt.Sprintf("=== RETRY %d/%d AGENT OUTPUT ===", attempt, maxRetries))
				reply, retryErr := agent.PromptWithTools(followUp, config.CodePromptTimeout, wrappedOnChunk, wrappedOnTool)
				if onChunk != nil {
					onChunk("\n")
				}
				if retryErr != nil && reply == "" {
					codeAppendLogLine(&logBuffer, fmt.Sprintf("retry_%d_error: %v", attempt, retryErr))
					color.Yellow("      重试失败: %v", retryErr)
					break
				}

				cr := prd.ParseCodeResult(reply)
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

				if _, buildErr2 := codeRunBuildPackages(workspace.WorktreeDir, changedPkgs); buildErr2 == nil {
					codeAppendLogLine(&logBuffer, fmt.Sprintf("retry_%d_postcheck: build passed", attempt))
					result.BuildOK = true
				}
			}
		}
	}

	changedFiles := result.FilesChanged
	if len(changedFiles) == 0 {
		changedFiles = codeCollectChanges(workspace.WorktreeDir)
	} else {
		gitChanged := codeCollectChanges(workspace.WorktreeDir)
		if len(gitChanged) > 0 {
			changedFiles = codeMergeFileLists(changedFiles, gitChanged)
		}
	}

	commitHash := ""
	if result.BuildOK && len(changedFiles) > 0 {
		hash, commitErr := codeCommitOnBranch(workspace.WorktreeDir, taskID, changedFiles, result.Summary)
		if commitErr != nil {
			codeAppendLogLine(&logBuffer, fmt.Sprintf("auto_commit_error: %v", commitErr))
			color.Yellow("⚠ auto-commit 失败: %v", commitErr)
		} else {
			codeAppendLogLine(&logBuffer, fmt.Sprintf("auto_commit: %s", hash))
			commitHash = hash
			patch, diffErr := codeReadCommitPatch(workspace.WorktreeDir)
			if diffErr != nil {
				codeAppendLogLine(&logBuffer, fmt.Sprintf("write_diff_patch_error: %v", diffErr))
			} else if writeErr := prd.WriteRepoDiffArtifacts(taskDir, repoBinding.ID, branchName, hash, changedFiles, patch); writeErr != nil {
				codeAppendLogLine(&logBuffer, fmt.Sprintf("write_diff_patch_error: %v", writeErr))
			} else {
				codeAppendLogLine(&logBuffer, fmt.Sprintf("diff_patch: %s", filepath.Join(taskDir, "diffs", repoBinding.ID+".patch")))
			}
		}
	}

	report := &prd.CodeResultReport{
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

	if err := prd.WriteCodeResultReport(taskDir, *report); err != nil {
		codeAppendLogLine(&logBuffer, fmt.Sprintf("write_code_result_error: %v", err))
		return nil, fmt.Errorf("写入 code-result.json 失败: %w", err)
	}
	codeAppendLogLine(&logBuffer, "=== RESULT ===")
	codeAppendLogLine(&logBuffer, fmt.Sprintf("build_ok: %t", report.BuildOK))
	codeAppendLogLine(&logBuffer, fmt.Sprintf("files_written: %s", strings.Join(report.FilesWritten, ", ")))
	codeAppendLogLine(&logBuffer, fmt.Sprintf("commit: %s", report.Commit))
	codeAppendLogLine(&logBuffer, fmt.Sprintf("code_result_json: %s", filepath.Join(taskDir, "code-result.json")))

	return report, nil
}

func renderSingleCodeReport(report *prd.CodeResultReport, branchName string, startedAt time.Time) {
	color.Cyan("   worktree: %s", report.Worktree)
	color.Cyan("   [3/3] 结果检查...")
	if report.BuildOK {
		color.Green("   [3/3] 编译通过 ✓")
	} else {
		color.Yellow("⚠ agent 未确认编译通过，改动未 commit")
	}

	if len(report.FilesWritten) > 0 {
		codeShowDiffSummary(report.Worktree, report.FilesWritten)
	} else {
		color.Yellow("⚠ 未检测到文件改动")
	}

	if report.Commit != "" {
		color.Green("   已自动 commit: %s", report.Commit)
	}

	color.Green("\n✓ prd code 完成")
	color.Green("  分支: %s", branchName)
	if report.Commit != "" {
		color.Green("  commit: %s", report.Commit)
	}
	color.Green("  worktree: %s", report.Worktree)
	color.Green("  改动文件: %d 个", len(report.FilesWritten))
	color.Green("⏱ 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))
}

func renderCodeRepoProgress(report *prd.CodeResultReport) {
	color.Cyan("   worktree: %s", report.Worktree)
	if report.BuildOK {
		color.Green("   编译通过 ✓")
	} else {
		color.Yellow("   编译状态未知")
	}
	if report.Commit != "" {
		color.Green("   commit: %s", report.Commit)
	}
	if len(report.FilesWritten) > 0 {
		color.Cyan("   改动文件: %d 个", len(report.FilesWritten))
	}
}

func renderCodeBatchSummary(reports []*prd.CodeResultReport, branchName string, startedAt time.Time) {
	color.Green("\n✓ prd code 完成")
	color.Green("  分支: %s", branchName)
	color.Cyan("  repo summary:")
	for _, report := range reports {
		status := "build_unknown"
		if report.BuildOK {
			status = "coded"
		}
		line := fmt.Sprintf("  - %s [%s]", report.RepoID, status)
		if report.Commit != "" {
			line += fmt.Sprintf(" commit=%s", report.Commit)
		}
		line += fmt.Sprintf(" files=%d", len(report.FilesWritten))
		color.Cyan(line)
	}
	color.Green("⏱ 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))
}

func codeAppendLogLine(b *strings.Builder, line string) {
	b.WriteString(line)
	b.WriteString("\n")
}

func codeFormatToolEvent(event generator.ToolEvent) string {
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

func codeReadCommitPatch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "show", "--format=medium", "--stat=0", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取 commit patch 失败: %s\n%s", err, string(output))
	}
	return string(output), nil
}

func buildPRDBranchName(taskID string) string {
	return "prd_" + taskID
}

// ---------- 2. 进度渲染 ----------

type todoItem struct {
	Content string `json:"Content"`
	ID      string `json:"ID"`
	Status  string `json:"Status"`
}

type todoRawInput struct {
	Todos []todoItem `json:"Todos"`
}

// renderToolEvent 渲染 agent 工具调用事件。
func renderToolEvent(event generator.ToolEvent) {
	// todo_write 特殊渲染
	if event.Title == "todo_write" && len(event.RawInput) > 0 {
		if event.Status == "in_progress" {
			var input todoRawInput
			if err := json.Unmarshal(event.RawInput, &input); err == nil && len(input.Todos) > 0 {
				color.Cyan("      📋 待办事项:")
				for _, item := range input.Todos {
					icon := "☐"
					if item.Status == "completed" {
						icon = "✓"
					} else if item.Status == "in_progress" {
						icon = "▶"
					}
					color.Cyan("         %s %s", icon, item.Content)
				}
			}
		}
		return
	}

	// 跳过无 kind 的工具
	if event.Kind == "" {
		return
	}

	// done 事件：简洁标记
	if event.Status == "done" {
		if event.Kind == "bash" {
			fmt.Println()
		}
		return
	}

	// in_progress 事件：按类型渲染
	switch event.Kind {
	case "edit":
		file := extractFileFromTitle(event.Title)
		color.Cyan("      ✏️  编辑 %s", file)
	case "bash":
		cmd := extractCmdFromTitle(event.Title)
		color.Cyan("      ⚡ 执行 %s", cmd)
	case "read":
		file := extractFileFromTitle(event.Title)
		color.Cyan("      📖 读取 %s", file)
	case "write":
		file := extractFileFromTitle(event.Title)
		color.Cyan("      📝 写入 %s", file)
	default:
		color.Cyan("      🔧 [%s] %s", event.Kind, event.Title)
	}
}

// extractFileFromTitle 从 title 中提取文件路径。
func extractFileFromTitle(title string) string {
	parts := strings.SplitN(title, " ", 2)
	if len(parts) == 2 && strings.HasPrefix(parts[1], "/") {
		return filepath.Base(parts[1])
	}
	return title
}

// extractCmdFromTitle 从 title 中提取命令。
func extractCmdFromTitle(title string) string {
	if after, ok := strings.CutPrefix(title, "Run command: "); ok {
		return after
	}
	return title
}

// ---------- 3. 结果汇总 ----------

// codeShowDiffSummary 展示改动文件的 diff 摘要。
func codeShowDiffSummary(repoRoot string, files []string) {
	color.Cyan("   改动内容:")
	for _, file := range files {
		cmd := exec.Command("git", "diff", "--numstat", "HEAD", "--", file)
		cmd.Dir = repoRoot
		output, _ := cmd.Output()
		added, deleted := parseNumstat(string(output))
		if added > 0 || deleted > 0 {
			color.Cyan("      - %s (+%d/-%d)", file, added, deleted)
		} else {
			color.Cyan("      - %s (新文件)", file)
		}
	}
}

// parseNumstat 解析 git diff --numstat 输出。
func parseNumstat(output string) (added, deleted int) {
	line := strings.TrimSpace(output)
	if line == "" {
		return 0, 0
	}
	parts := strings.Split(line, "\t")
	if len(parts) >= 2 {
		fmt.Sscanf(parts[0], "%d", &added)
		fmt.Sscanf(parts[1], "%d", &deleted)
	}
	return added, deleted
}

// ---------- Git 操作 ----------

// codeCollectChanges 获取工作区中的改动文件列表。
func codeCollectChanges(repoRoot string) []string {
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

// codeMergeFileLists 合并 agent 报告的文件和 git 实际改动文件，去重。
func codeMergeFileLists(agentFiles, gitFiles []string) []string {
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

// codeExtractChangedPackages 从 git 改动文件中提取涉及的 package 目录（去重）。
func codeExtractChangedPackages(repoRoot string) []string {
	files := codeCollectChanges(repoRoot)
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

// codeRunBuildPackages 只编译指定的 package 列表，避免全量编译。
func codeRunBuildPackages(repoRoot string, pkgs []string) (string, error) {
	args := append([]string{"build"}, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// codeCommitOnBranch 在当前分支 add + commit。
func codeCommitOnBranch(repoRoot, taskID string, files []string, summary string) (string, error) {
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
