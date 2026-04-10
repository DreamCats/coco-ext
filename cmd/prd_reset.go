package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdResetTaskID string

var prdResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "重置 task 的 code 状态，回退分支改动，可重新执行 prd code",
	Long:  "将 coded 状态的 task 回退到 planned，回退分支上的 auto-commit，删除 code-result.json。",
	RunE:  runPRDReset,
}

func init() {
	prdCmd.AddCommand(prdResetCmd)
	prdResetCmd.Flags().StringVar(&prdResetTaskID, "task", "", "指定 task id；默认读取最近一个 task")
}

func runPRDReset(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	taskID, err := prd.ResolveTaskID(repoRoot, prdResetTaskID)
	if err != nil {
		return err
	}

	task, err := prd.LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return err
	}

	if task.Metadata.Status != prd.TaskStatusCoded && task.Metadata.Status != "build_failed" {
		return fmt.Errorf("task 状态为 %s，仅 coded / build_failed 状态可重置", task.Metadata.Status)
	}

	color.Cyan("🔄 PRD Reset")
	color.Cyan("   task_id: %s", taskID)

	branchName := "prd/" + taskID

	// 读取 code-result.json 获取 commit 信息
	report, _ := prd.ReadCodeResultReport(task.TaskDir)
	if report != nil && report.Branch != "" {
		branchName = report.Branch
	}

	// 回退分支上的 auto-commit
	commitReverted := false
	checkCmd := exec.Command("git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = repoRoot
	if checkCmd.Run() == nil {
		// 切到分支，回退最后一个 commit（保留改动在工作区）
		checkoutCmd := exec.Command("git", "checkout", branchName)
		checkoutCmd.Dir = repoRoot
		if checkoutCmd.Run() == nil {
			resetCmd := exec.Command("git", "reset", "--soft", "HEAD~1")
			resetCmd.Dir = repoRoot
			if resetCmd.Run() == nil {
				commitReverted = true
				color.Green("   ✓ 已回退 auto-commit（改动保留在工作区）")
			}

			// 切回原分支
			origBranch := codeCurrentBranch(repoRoot)
			_ = origBranch // 已经在 branchName 上了
			// 回退完把改动带回来：先 stash，切回，再 pop
			stashCmd := exec.Command("git", "stash")
			stashCmd.Dir = repoRoot
			_ = stashCmd.Run()

			// 找到主分支
			mainBranch := "main"
			if b, err := exec.Command("git", "rev-parse", "--verify", "master").CombinedOutput(); err == nil && len(b) > 0 {
				mainBranch = "master"
			}
			exec.Command("git", "checkout", mainBranch).Dir = repoRoot

			exec.Command("git", "stash", "pop").Dir = repoRoot
		}
	}

	// 删除分支
	branchDeleted := false
	delCmd := exec.Command("git", "branch", "-D", branchName)
	delCmd.Dir = repoRoot
	if delCmd.Run() == nil {
		branchDeleted = true
		color.Green("   ✓ 已删除分支 %s", branchName)
	}

	// 删除 code-result.json
	_ = os.Remove(filepath.Join(task.TaskDir, "code-result.json"))

	// 回退状态到 planned
	if err := prd.ResetTaskToPlanned(task.TaskDir, time.Now()); err != nil {
		return err
	}
	color.Green("   ✓ 状态已回退为 planned")

	result := map[string]any{
		"status":          "reset",
		"task_id":         taskID,
		"branch":          branchName,
		"branch_deleted":  branchDeleted,
		"commit_reverted": commitReverted,
		"message":         "task 已重置为 planned 状态，可重新执行 prd code。",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	return nil
}
