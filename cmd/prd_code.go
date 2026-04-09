package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/generator"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdCodeTaskID string
var prdCodeBranch string

var prdCodeCmd = &cobra.Command{
	Use:   "code",
	Short: "基于 plan.md 自动生成实现代码",
	Long:  "读取 plan.md 和 design.md，结合 context 和源文件，AI 一次性生成所有改动并写入新分支。仅支持简单/中等复杂度需求。",
	RunE:  runPRDCode,
}

func init() {
	prdCmd.AddCommand(prdCodeCmd)
	prdCodeCmd.Flags().StringVar(&prdCodeTaskID, "task", "", "指定 task id；默认读取最近一个 task")
	prdCodeCmd.Flags().StringVar(&prdCodeBranch, "branch", "", "自定义分支名；默认 prd/{task_id}")
}

func runPRDCode(cmd *cobra.Command, args []string) error {
	startedAt := time.Now()
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

	color.Cyan("🔨 PRD Code")
	color.Cyan("   task_id: %s", taskID)

	// [1/5] 校验 task 状态和复杂度
	color.Cyan("   [1/5] 正在校验 task 状态和复杂度...")
	build, err := prd.PrepareCodeBuild(repoRoot, taskID)
	if err != nil {
		return err
	}
	color.Green("   [1/5] 校验通过 ✓ (候选文件: %d 个)", len(build.CandidateFiles))

	// 记录当前分支，用于失败回滚
	origBranch := codeGetCurrentBranch(repoRoot)

	// [2/5] 创建分支
	branchName := prdCodeBranch
	if branchName == "" {
		branchName = "prd/" + taskID
	}
	color.Cyan("   [2/5] 正在创建分支 %s ...", branchName)
	if err := codeCreateBranch(repoRoot, branchName); err != nil {
		return err
	}
	color.Green("   [2/5] 分支 %s 已创建 ✓", branchName)

	// [3/5] 连接 daemon
	color.Cyan("   [3/5] 正在连接 coco daemon...")
	connectStartedAt := time.Now()
	gen, err := generator.New(repoRoot)
	if err != nil {
		codeRollbackBranch(repoRoot, branchName, origBranch)
		return fmt.Errorf("连接 coco daemon 失败: %w\n建议：先执行 coco-ext doctor --fix", err)
	}
	defer gen.Close()
	color.Green("   [3/5] coco daemon 已连接 ✓")
	color.Cyan("      连接耗时: %s", formatDurationSeconds(time.Since(connectStartedAt)))

	// [4/5] AI 生成代码
	color.Cyan("   [4/5] 正在生成代码...")
	generateStartedAt := time.Now()
	var streamBuffer strings.Builder
	streamStarted := false
	firstChunkShown := make(chan struct{})
	stopTicker := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopTicker:
				return
			case <-firstChunkShown:
				return
			case <-ticker.C:
				fmt.Printf("\r\033[K   生成中，已耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))
			}
		}
	}()

	now := time.Now()
	result, err := prd.GenerateCode(gen, build, now, func(chunk string) {
		streamBuffer.WriteString(chunk)
		if streamStarted {
			return
		}
		if strings.Contains(streamBuffer.String(), "=== FILE:") {
			streamStarted = true
			close(firstChunkShown)
			clearCodeProgressLine()
			color.Cyan("      AI 正在输出代码...")
		}
	})
	close(stopTicker)
	clearCodeProgressLine()

	if err != nil {
		codeRollbackBranch(repoRoot, branchName, origBranch)
		return err
	}
	color.Cyan("      生成耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))

	// 显示写入结果
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
		codeRollbackBranch(repoRoot, branchName, origBranch)
		return fmt.Errorf("没有文件成功写入")
	}

	// [5/5] 编译检查 + auto commit
	color.Cyan("   [5/5] 编译检查...")
	if result.BuildOK {
		color.Green("   [5/5] 编译通过 ✓")

		commitHash, commitErr := codeCommitChanges(repoRoot, build.Task, taskID, writtenFiles)
		if commitErr != nil {
			color.Yellow("⚠ auto-commit 失败: %v", commitErr)
			color.Yellow("  文件已写入，请手动 commit。")
		} else {
			result.CommitHash = commitHash
			result.Branch = branchName
			color.Green("   已自动 commit: %s", commitHash)
		}
	} else {
		color.Yellow("⚠ 编译失败，未自动 commit")
		color.Yellow("  %s", result.BuildOutput)
		color.Yellow("  文件已写入分支 %s，请手动修复后 commit。", branchName)
	}

	color.Green("\n✓ prd code 完成")
	color.Green("  分支: %s", branchName)
	color.Green("  写入文件: %d 个", writtenCount)
	color.Green("⏱ 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))
	color.Green("\n  next: git diff main  # 查看所有改动")

	return nil
}

// codeGetCurrentBranch 获取当前分支名，失败时返回 "main"。
func codeGetCurrentBranch(repoRoot string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "main"
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "main"
	}
	return branch
}

// codeCreateBranch 从最新的 origin/main 创建并切换到新分支。
func codeCreateBranch(repoRoot, branchName string) error {
	currentBranch := codeGetCurrentBranch(repoRoot)

	// 检查工作区是否干净（已在目标分支上时跳过，允许重试）
	if currentBranch != branchName {
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = repoRoot
		statusOutput, err := statusCmd.Output()
		if err != nil {
			return fmt.Errorf("检查工作区状态失败: %w", err)
		}
		if strings.TrimSpace(string(statusOutput)) != "" {
			return fmt.Errorf("工作区有未提交的改动，请先 commit 或 stash 后再执行 prd code")
		}
	}

	// 检查分支是否已存在
	checkCmd := exec.Command("git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = repoRoot
	branchExists := checkCmd.Run() == nil

	if branchExists && currentBranch == branchName {
		// 已在目标分支上（上次编译失败后重试），跳过创建
		return nil
	}
	if branchExists {
		return fmt.Errorf("分支 %s 已存在，请先切换到该分支重试，或删除后重新执行", branchName)
	}

	// fetch 最新
	fetchCmd := exec.Command("git", "fetch", "origin")
	fetchCmd.Dir = repoRoot
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch origin 失败: %s\n%s", err, string(output))
	}

	// 检测远程默认分支
	defaultBranch := codeDetectRemoteDefaultBranch(repoRoot)

	// 从 origin/{default} 创建新分支
	checkoutCmd := exec.Command("git", "checkout", "-b", branchName, "origin/"+defaultBranch)
	checkoutCmd.Dir = repoRoot
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("创建分支 %s 失败: %s\n%s", branchName, err, string(output))
	}

	return nil
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

// codeRollbackBranch 回滚分支：丢弃改动 → 切回原分支 → 删除新分支。
// 如果原分支就是新分支（重试场景），只丢弃改动，不删除分支。
func codeRollbackBranch(repoRoot, branchName, origBranch string) {
	if origBranch == branchName {
		// 重试场景：只丢弃本次改动
		resetCmd := exec.Command("git", "checkout", "--", ".")
		resetCmd.Dir = repoRoot
		_ = resetCmd.Run()
		return
	}

	checkoutCmd := exec.Command("git", "checkout", "-f", origBranch)
	checkoutCmd.Dir = repoRoot
	_ = checkoutCmd.Run()

	deleteCmd := exec.Command("git", "branch", "-D", branchName)
	deleteCmd.Dir = repoRoot
	_ = deleteCmd.Run()
}

// codeCommitChanges 将生成的文件 add + commit。
func codeCommitChanges(repoRoot string, task *prd.TaskStatusReport, taskID string, files []string) (string, error) {
	args := append([]string{"add"}, files...)
	addCmd := exec.Command("git", args...)
	addCmd.Dir = repoRoot
	if output, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add 失败: %s\n%s", err, string(output))
	}

	msg := fmt.Sprintf("feat(prd): %s\n\ntask_id: %s\ngenerated by: coco-ext prd code", task.Metadata.Title, taskID)
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

func clearCodeProgressLine() {
	fmt.Print("\r\033[K")
}
