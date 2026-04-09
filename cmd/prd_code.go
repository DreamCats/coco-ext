package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/generator"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var (
	prdCodeTaskID   string
	prdCodeBranch   string
	prdCodeSync     bool
	prdCodeDryRun   bool
	prdCodeBG       bool   // hidden: 后台进程模式
	prdCodeWTPath   string // hidden: worktree 绝对路径
	prdCodeRepoRoot string // hidden: 主仓库绝对路径
)

var prdCodeCmd = &cobra.Command{
	Use:   "code",
	Short: "基于 plan.md 自动生成实现代码",
	Long:  "读取 plan.md 和 design.md，结合 context 和源文件，AI 一次性生成所有改动。默认在 worktree 中异步执行，立即返回 JSON 供 LLM 消费。",
	RunE:  runPRDCode,
}

func init() {
	prdCmd.AddCommand(prdCodeCmd)
	prdCodeCmd.Flags().StringVar(&prdCodeTaskID, "task", "", "指定 task id；默认读取最近一个 task")
	prdCodeCmd.Flags().StringVar(&prdCodeBranch, "branch", "", "自定义分支名；默认 prd/{task_id}")
	prdCodeCmd.Flags().BoolVar(&prdCodeSync, "sync", false, "同步执行（阻塞等待完成）")
	prdCodeCmd.Flags().BoolVar(&prdCodeDryRun, "dry-run", false, "仅输出 prompt，不连接 daemon")
	// 后台进程专用隐藏 flag
	prdCodeCmd.Flags().BoolVar(&prdCodeBG, "_bg", false, "")
	prdCodeCmd.Flags().StringVar(&prdCodeWTPath, "_wt", "", "")
	prdCodeCmd.Flags().StringVar(&prdCodeRepoRoot, "_repo", "", "")
	_ = prdCodeCmd.Flags().MarkHidden("_bg")
	_ = prdCodeCmd.Flags().MarkHidden("_wt")
	_ = prdCodeCmd.Flags().MarkHidden("_repo")
}

// ---------- 入口 ----------

func runPRDCode(cmd *cobra.Command, args []string) error {
	if prdCodeBG {
		return runPRDCodeBackground()
	}

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

	// 校验 task 状态和复杂度（在主仓库读取，不依赖 worktree）
	build, err := prd.PrepareCodeBuild(repoRoot, taskID)
	if err != nil {
		return err
	}

	// dry-run 模式：只输出 prompt，不连接 daemon
	if prdCodeDryRun {
		prompt := prd.BuildCodePrompt(build)
		dumpPath := filepath.Join(build.Task.TaskDir, "code-prompt-debug.txt")
		_ = os.WriteFile(dumpPath, []byte(prompt), 0644)
		fmt.Printf("prompt 大小: %d 字符 (%.1f KB)\n", len([]rune(prompt)), float64(len(prompt))/1024)
		fmt.Printf("plan 标识符: %v\n", build.PlanIdents)
		fmt.Printf("候选文件: %v\n", build.CandidateFiles)
		fmt.Printf("prompt 已写入: %s\n", dumpPath)
		return nil
	}

	branchName := prdCodeBranch
	if branchName == "" {
		branchName = "prd/" + taskID
	}

	// worktree 路径：主仓库同级目录
	repoName := filepath.Base(repoRoot)
	worktreePath := filepath.Join(filepath.Dir(repoRoot), repoName+"--prd--"+taskID)

	// 创建 worktree
	if err := codeCreateWorktree(repoRoot, worktreePath, branchName); err != nil {
		return err
	}

	if prdCodeSync {
		return runPRDCodeForeground(repoRoot, build, branchName, worktreePath)
	}
	return runPRDCodeAsync(repoRoot, build, branchName, worktreePath)
}

// ---------- 异步模式（默认）----------

func runPRDCodeAsync(repoRoot string, build *prd.CodeBuild, branchName, worktreePath string) error {
	taskID := build.TaskID
	taskDir := build.Task.TaskDir
	logPath := filepath.Join(taskDir, "code.log")
	resultPath := filepath.Join(taskDir, "code-result.json")

	// 写入初始 result
	startedAt := time.Now().Format(time.RFC3339)
	_ = prd.WriteCodeResultReport(taskDir, prd.CodeResultReport{
		Status:    "generating",
		TaskID:    taskID,
		Branch:    branchName,
		Worktree:  worktreePath,
		Log:       logPath,
		StartedAt: startedAt,
	})

	// 拉起后台进程
	exe, err := os.Executable()
	if err != nil {
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return fmt.Errorf("创建日志文件失败: %w", err)
	}
	defer logFile.Close()

	bgCmd := exec.Command(exe, "prd", "code",
		"--task", taskID,
		"--branch", branchName,
		"--_bg",
		"--_wt", worktreePath,
		"--_repo", repoRoot,
	)
	bgCmd.Dir = repoRoot
	bgCmd.Stdin = nil
	bgCmd.Stdout = logFile
	bgCmd.Stderr = logFile
	bgCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := bgCmd.Start(); err != nil {
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return fmt.Errorf("启动后台进程失败: %w", err)
	}
	_ = bgCmd.Process.Release()

	// 立即返回结构化 JSON
	response := map[string]any{
		"status":      "started",
		"task_id":     taskID,
		"branch":      branchName,
		"worktree":    worktreePath,
		"result_file": resultPath,
		"log_file":    logPath,
		"candidate_files": build.CandidateFiles,
		"message":     "代码生成已在后台启动。完成后请读取 result_file 查看结果。",
	}
	data, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(data))

	return nil
}

// ---------- 后台进程 ----------

func runPRDCodeBackground() error {
	repoRoot := prdCodeRepoRoot
	if repoRoot == "" {
		var err error
		repoRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	worktreePath := prdCodeWTPath
	taskID := prdCodeTaskID
	branchName := prdCodeBranch
	startedAt := time.Now()

	build, err := prd.PrepareCodeBuild(repoRoot, taskID)
	if err != nil {
		codeWriteFailResult(build, repoRoot, worktreePath, branchName, taskID, startedAt, err)
		return err
	}
	taskDir := build.Task.TaskDir

	gen, err := generator.New(repoRoot)
	if err != nil {
		codeWriteFailResult(build, repoRoot, worktreePath, branchName, taskID, startedAt,
			fmt.Errorf("连接 coco daemon 失败: %w", err))
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return err
	}
	defer gen.Close()

	fmt.Printf("[prd code] task_id=%s, worktree=%s\n", taskID, worktreePath)

	fmt.Println("[prd code] AI 代码生成中...")

	result, err := prd.GenerateCode(gen, build, worktreePath, time.Now(), nil)
	if err != nil {
		codeWriteFailResult(build, repoRoot, worktreePath, branchName, taskID, startedAt, err)
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return err
	}

	var writtenFiles []string
	for _, f := range result.Files {
		if f.Written {
			fmt.Printf("[prd code] ✓ %s\n", f.Path)
			writtenFiles = append(writtenFiles, f.Path)
		} else {
			fmt.Printf("[prd code] ✗ %s: %s\n", f.Path, f.Error)
		}
	}

	if len(writtenFiles) == 0 {
		codeWriteFailResult(build, repoRoot, worktreePath, branchName, taskID, startedAt,
			fmt.Errorf("没有文件成功写入"))
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return fmt.Errorf("没有文件成功写入")
	}

	commitHash := ""
	if result.BuildOK {
		fmt.Println("[prd code] 编译通过，正在 commit...")
		hash, commitErr := codeCommitInWorktree(worktreePath, build.Task.Metadata.Title, taskID, writtenFiles)
		if commitErr != nil {
			fmt.Printf("[prd code] commit 失败: %v\n", commitErr)
		} else {
			commitHash = hash
			fmt.Printf("[prd code] 已 commit: %s\n", hash)
		}
	} else {
		fmt.Printf("[prd code] 编译失败:\n%s\n", result.BuildOutput)
	}

	report := prd.CodeResultReport{
		Status:       "success",
		TaskID:       taskID,
		Branch:       branchName,
		Worktree:     worktreePath,
		Commit:       commitHash,
		BuildOK:      result.BuildOK,
		FilesWritten: writtenFiles,
		Log:          filepath.Join(taskDir, "code.log"),
		StartedAt:    startedAt.Format(time.RFC3339),
		FinishedAt:   time.Now().Format(time.RFC3339),
	}
	if !result.BuildOK {
		report.Status = "build_failed"
		report.Error = result.BuildOutput
	}
	_ = prd.WriteCodeResultReport(taskDir, report)

	fmt.Printf("[prd code] 完成，耗时 %s\n", formatDurationSeconds(time.Since(startedAt)))
	return nil
}

func codeWriteFailResult(build *prd.CodeBuild, repoRoot, worktreePath, branchName, taskID string, startedAt time.Time, cause error) {
	taskDir := ""
	if build != nil && build.Task != nil {
		taskDir = build.Task.TaskDir
	} else {
		taskDir = filepath.Join(repoRoot, ".livecoding", "tasks", taskID)
	}
	_ = prd.WriteCodeResultReport(taskDir, prd.CodeResultReport{
		Status:     "failed",
		TaskID:     taskID,
		Branch:     branchName,
		Worktree:   worktreePath,
		Error:      cause.Error(),
		Log:        filepath.Join(taskDir, "code.log"),
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: time.Now().Format(time.RFC3339),
	})
}

// ---------- 同步模式 ----------

func runPRDCodeForeground(repoRoot string, build *prd.CodeBuild, branchName, worktreePath string) error {
	startedAt := time.Now()
	taskID := build.TaskID

	color.Cyan("🔨 PRD Code (sync)")
	color.Cyan("   task_id: %s", taskID)
	color.Cyan("   worktree: %s", worktreePath)
	color.Cyan("   branch: %s", branchName)
	color.Green("   [1/4] 校验通过 ✓ (候选文件: %d 个)", len(build.CandidateFiles))

	color.Cyan("   [2/4] 正在连接 coco daemon...")
	connectStartedAt := time.Now()
	gen, err := generator.New(repoRoot)
	if err != nil {
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return fmt.Errorf("连接 coco daemon 失败: %w\n建议：先执行 coco-ext doctor --fix", err)
	}
	defer gen.Close()
	color.Green("   [2/4] coco daemon 已连接 ✓")
	color.Cyan("      连接耗时: %s", formatDurationSeconds(time.Since(connectStartedAt)))

	color.Cyan("   [3/4] 正在生成代码...")
	generateStartedAt := time.Now()

	now := time.Now()
	result, err := prd.GenerateCode(gen, build, worktreePath, now, func(chunk string) {
		fmt.Print(chunk)
	})
	fmt.Println()

	if err != nil {
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return err
	}
	color.Cyan("      生成耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))

	writtenCount := 0
	var writtenFiles []string
	for _, f := range result.Files {
		if f.Written {
			color.Green("      ✓ %s", f.Path)
			writtenCount++
			writtenFiles = append(writtenFiles, f.Path)
		} else {
			color.Red("      ✗ %s: %s", f.Path, f.Error)
		}
	}

	if writtenCount == 0 {
		codeRemoveWorktree(repoRoot, worktreePath, branchName)
		return fmt.Errorf("没有文件成功写入")
	}

	color.Cyan("   [4/4] 编译检查...")
	if result.BuildOK {
		color.Green("   [4/4] 编译通过 ✓")
		commitHash, commitErr := codeCommitInWorktree(worktreePath, build.Task.Metadata.Title, taskID, writtenFiles)
		if commitErr != nil {
			color.Yellow("⚠ auto-commit 失败: %v", commitErr)
		} else {
			color.Green("   已自动 commit: %s", commitHash)
		}
	} else {
		color.Yellow("⚠ 编译失败，未自动 commit")
		color.Yellow("  %s", result.BuildOutput)
	}

	// 写入 result file
	report := prd.CodeResultReport{
		Status:       "success",
		TaskID:       taskID,
		Branch:       branchName,
		Worktree:     worktreePath,
		BuildOK:      result.BuildOK,
		FilesWritten: writtenFiles,
		Log:          filepath.Join(build.Task.TaskDir, "code.log"),
		StartedAt:    startedAt.Format(time.RFC3339),
		FinishedAt:   time.Now().Format(time.RFC3339),
	}
	if !result.BuildOK {
		report.Status = "build_failed"
		report.Error = result.BuildOutput
	}
	_ = prd.WriteCodeResultReport(build.Task.TaskDir, report)

	color.Green("\n✓ prd code 完成")
	color.Green("  分支: %s", branchName)
	color.Green("  worktree: %s", worktreePath)
	color.Green("  写入文件: %d 个", writtenCount)
	color.Green("⏱ 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))

	return nil
}

// ---------- Worktree 管理 ----------

// codeCreateWorktree 创建 worktree。如果已存在则复用。
func codeCreateWorktree(repoRoot, worktreePath, branchName string) error {
	// worktree 已存在 → 复用（幂等重试）
	if _, err := os.Stat(filepath.Join(worktreePath, ".git")); err == nil {
		return nil
	}

	// 清理已删除但未注销的 worktree 记录（手动删目录后残留）
	pruneCmd := exec.Command("git", "worktree", "prune")
	pruneCmd.Dir = repoRoot
	_ = pruneCmd.Run()

	fetchCmd := exec.Command("git", "fetch", "origin")
	fetchCmd.Dir = repoRoot
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch origin 失败: %s\n%s", err, string(output))
	}

	defaultBranch := codeDetectRemoteDefaultBranch(repoRoot)

	// 检查分支是否已存在
	checkCmd := exec.Command("git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = repoRoot
	branchExists := checkCmd.Run() == nil

	if branchExists {
		// 分支存在但 worktree 不在 → 在已有分支上创建 worktree
		wtCmd := exec.Command("git", "worktree", "add", worktreePath, branchName)
		wtCmd.Dir = repoRoot
		if output, err := wtCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("创建 worktree 失败（已有分支）: %s\n%s", err, string(output))
		}
	} else {
		// 新建分支 + worktree
		wtCmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, "origin/"+defaultBranch)
		wtCmd.Dir = repoRoot
		if output, err := wtCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("创建 worktree 失败: %s\n%s", err, string(output))
		}
	}

	return nil
}

// codeRemoveWorktree 清理 worktree 和分支。
func codeRemoveWorktree(repoRoot, worktreePath, branchName string) {
	rmCmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	rmCmd.Dir = repoRoot
	_ = rmCmd.Run()

	delCmd := exec.Command("git", "branch", "-D", branchName)
	delCmd.Dir = repoRoot
	_ = delCmd.Run()
}

// codeDetectRemoteDefaultBranch 检测远程默认分支名。
func codeDetectRemoteDefaultBranch(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--verify", "origin/main")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return "main"
	}
	cmd = exec.Command("git", "rev-parse", "--verify", "origin/master")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return "master"
	}
	return "main"
}

// codeCommitInWorktree 在 worktree 中 add + commit。
func codeCommitInWorktree(worktreePath, title, taskID string, files []string) (string, error) {
	args := append([]string{"add"}, files...)
	addCmd := exec.Command("git", args...)
	addCmd.Dir = worktreePath
	if output, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add 失败: %s\n%s", err, string(output))
	}

	msg := fmt.Sprintf("feat(prd): %s\n\ntask_id: %s\ngenerated by: coco-ext prd code", title, taskID)
	commitCmd := exec.Command("git", "commit", "-m", msg)
	commitCmd.Dir = worktreePath
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit 失败: %s\n%s", err, string(output))
	}

	hashCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	hashCmd.Dir = worktreePath
	hashOutput, err := hashCmd.Output()
	if err != nil {
		return "unknown", nil
	}
	return strings.TrimSpace(string(hashOutput)), nil
}

func clearCodeProgressLine() {
	fmt.Print("\r\033[K")
}
