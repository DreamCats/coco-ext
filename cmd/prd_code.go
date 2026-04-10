package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
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
	prdCodeTaskID string
	prdCodeBranch string
)

var prdCodeCmd = &cobra.Command{
	Use:   "code",
	Short: "基于 plan.md 自动生成实现代码",
	Long:  "读取 plan.md 和 design.md，启动 yolo agent 自主读写文件和编译。在主仓库分支上工作，完成后自动 commit 并切回原分支。",
	RunE:  runPRDCode,
}

func init() {
	prdCmd.AddCommand(prdCodeCmd)
	prdCodeCmd.Flags().StringVar(&prdCodeTaskID, "task", "", "指定 task id；默认读取最近一个 task")
	prdCodeCmd.Flags().StringVar(&prdCodeBranch, "branch", "", "自定义分支名；默认 prd/{task_id}")
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

	taskDir, err := prd.PrepareAgentCode(repoRoot, taskID)
	if err != nil {
		return err
	}

	branchName := prdCodeBranch
	if branchName == "" {
		branchName = "prd/" + taskID
	}

	startedAt := time.Now()
	origBranch := codeCurrentBranch(repoRoot)

	// ---- 1. 前置检查：未提交改动时 stash ----
	stashed := false
	if codeHasUncommittedChanges(repoRoot) {
		color.Cyan("   检测到未提交的改动，正在 stash...")
		if err := codeStash(repoRoot); err != nil {
			return fmt.Errorf("stash 失败: %w", err)
		}
		stashed = true
		color.Green("   已 stash ✓")
	}

	// ---- 5. 中断处理：Ctrl+C 时恢复 ----
	cleanup := func() {
		agent, _ := cmd.Flags().GetString("_agent_ref")
		_ = agent // agent cleanup handled by defer
		codeCheckoutBranchQuiet(repoRoot, origBranch)
		if stashed {
			codeStashPop(repoRoot)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		color.Yellow("⚠ 收到中断信号，正在恢复...")
		cleanup()
		os.Exit(130)
	}()
	defer signal.Stop(sigCh)

	color.Cyan("🤖 PRD Code")
	color.Cyan("   task_id: %s", taskID)
	color.Cyan("   branch: %s", branchName)

	// 创建并切换到工作分支
	if err := codeCheckoutBranch(repoRoot, branchName); err != nil {
		if stashed {
			codeStashPop(repoRoot)
		}
		return err
	}

	// ---- 4. 错误恢复：任何失败都恢复状态 ----
	restored := false
	defer func() {
		if restored {
			return
		}
		codeCheckoutBranchQuiet(repoRoot, origBranch)
		if stashed {
			codeStashPop(repoRoot)
		}
	}()

	color.Cyan("   [1/3] 正在启动 AI agent（yolo 模式）...")
	connectStartedAt := time.Now()
	agent, err := generator.NewAgent(repoRoot)
	if err != nil {
		return fmt.Errorf("启动 AI agent 失败: %w", err)
	}
	defer agent.Close()
	color.Green("   [1/3] AI agent 已就绪 ✓")
	color.Cyan("      启动耗时: %s", formatDurationSeconds(time.Since(connectStartedAt)))

	color.Cyan("   [2/3] agent 正在自主实现代码...")
	generateStartedAt := time.Now()

	result, err := prd.GenerateCodeWithAgent(agent, taskDir, time.Now(),
		func(chunk string) {
			fmt.Print(chunk)
		},
		func(event generator.ToolEvent) {
			renderToolEvent(event)
		},
	)
	fmt.Println()

	if err != nil {
		return err
	}
	color.Cyan("      生成耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))

	// 统计工具调用
	toolCounts := make(map[string]int)
	for _, e := range result.ToolEvents {
		if e.Status == "done" {
			toolCounts[e.Kind]++
		}
	}
	if len(toolCounts) > 0 {
		color.Cyan("      工具调用: %v", toolCounts)
	}

	// ---- 3. 结果汇总：收集改动 + diff 预览 ----
	changedFiles := codeCollectChanges(repoRoot)

	color.Cyan("   [3/3] 结果检查...")
	commitHash := ""
	if result.BuildOK && len(changedFiles) > 0 {
		color.Green("   [3/3] 编译通过 ✓")

		// diff 摘要
		codeShowDiffSummary(repoRoot, changedFiles)

		hash, commitErr := codeCommitOnBranch(repoRoot, taskID, changedFiles)
		if commitErr != nil {
			color.Yellow("⚠ auto-commit 失败: %v", commitErr)
		} else {
			commitHash = hash
			color.Green("   已自动 commit: %s", commitHash)
		}
	} else if len(changedFiles) == 0 {
		color.Yellow("⚠ 未检测到文件改动")
	} else {
		color.Yellow("⚠ agent 未确认编译通过，改动未 commit")
		// 即使未通过也展示改动
		if len(changedFiles) > 0 {
			color.Cyan("   改动文件（未 commit）:")
			for _, f := range changedFiles {
				color.Cyan("      - %s", f)
			}
		}
	}

	// 写入 result file
	report := prd.CodeResultReport{
		Status:       "success",
		TaskID:       taskID,
		Branch:       branchName,
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
	_ = prd.WriteCodeResultReport(taskDir, report)

	// 标记已恢复（由后续逻辑完成），防止 defer 重复恢复
	restored = true
	codeCheckoutBranchQuiet(repoRoot, origBranch)

	// 恢复 stash
	if stashed {
		if popErr := codeStashPop(repoRoot); popErr != nil {
			color.Yellow("⚠ stash pop 失败（改动可通过 git stash list 找回）: %v", popErr)
		} else {
			color.Green("   已恢复 stash ✓")
		}
	}

	color.Green("\n✓ prd code 完成")
	color.Green("  分支: %s", branchName)
	if commitHash != "" {
		color.Green("  commit: %s", commitHash)
	}
	color.Green("  改动文件: %d 个", len(changedFiles))
	color.Green("⏱ 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))

	return nil
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
			// bash 完成时只显示 ✓
			fmt.Println()
		}
		return
	}

	// in_progress 事件：按类型渲染
	switch event.Kind {
	case "edit":
		// 提取文件名
		file := extractFileFromTitle(event.Title)
		color.Cyan("      ✏️  编辑 %s", file)
	case "bash":
		// 提取命令
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
	// title 格式: "Read /path/to/file" 或 "Edit /path/to/file"
	parts := strings.SplitN(title, " ", 2)
	if len(parts) == 2 && strings.HasPrefix(parts[1], "/") {
		return filepath.Base(parts[1])
	}
	return title
}

// extractCmdFromTitle 从 title 中提取命令。
func extractCmdFromTitle(title string) string {
	// title 格式: "Run command: go build ./..."
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
		// 统计增删行数
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
	// 格式: "3\t2\tfile.go"
	parts := strings.Split(line, "\t")
	if len(parts) >= 2 {
		fmt.Sscanf(parts[0], "%d", &added)
		fmt.Sscanf(parts[1], "%d", &deleted)
	}
	return added, deleted
}

// ---------- Git 操作 ----------

// codeCurrentBranch 获取当前分支名。
func codeCurrentBranch(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(string(output))
}

// codeHasUncommittedChanges 检查是否有未提交的改动。
func codeHasUncommittedChanges(repoRoot string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// codeStash 执行 git stash。
func codeStash(repoRoot string) error {
	cmd := exec.Command("git", "stash", "push", "-m", "coco-ext prd code auto-stash")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// codeStashPop 执行 git stash pop。
func codeStashPop(repoRoot string) error {
	cmd := exec.Command("git", "stash", "pop")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// codeCheckoutBranch 创建（如不存在）并切换到分支。
func codeCheckoutBranch(repoRoot, branchName string) error {
	checkCmd := exec.Command("git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = repoRoot
	if checkCmd.Run() == nil {
		cmd := exec.Command("git", "checkout", branchName)
		cmd.Dir = repoRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("切换到分支 %s 失败: %s\n%s", branchName, err, string(output))
		}
		return nil
	}
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("创建分支 %s 失败: %s\n%s", branchName, err, string(output))
	}
	return nil
}

// codeCheckoutBranchQuiet 静默切回分支。
func codeCheckoutBranchQuiet(repoRoot, branchName string) {
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = repoRoot
	_ = cmd.Run()
}

// codeCollectChanges 获取主仓库中的改动文件列表。
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

// codeCommitOnBranch 在当前分支 add + commit。
func codeCommitOnBranch(repoRoot, taskID string, files []string) (string, error) {
	args := append([]string{"add"}, files...)
	addCmd := exec.Command("git", args...)
	addCmd.Dir = repoRoot
	if output, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add 失败: %s\n%s", err, string(output))
	}

	msg := fmt.Sprintf("feat(prd): auto-generated code\n\ntask_id: %s\ngenerated by: coco-ext prd code", taskID)
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
