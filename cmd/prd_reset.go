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

	branchName := buildPRDBranchName(taskID)
	worktreePath := ""

	report, _ := prd.ReadCodeResultReport(task.TaskDir)
	if report != nil && report.Branch != "" {
		branchName = report.Branch
	}
	if report != nil {
		worktreePath = report.Worktree
	}

	worktreeDeleted := false
	if worktreePath != "" {
		if err := prd.CleanupCodeWorktree(repoRoot, worktreePath); err != nil {
			color.Yellow("   ⚠ 删除 worktree 失败: %v", err)
		} else {
			worktreeDeleted = true
			color.Green("   ✓ 已删除 worktree %s", worktreePath)
		}
	}

	// 直接删除分支（强制，丢弃所有改动）
	branchDeleted := false
	checkCmd := exec.Command("git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = repoRoot
	if checkCmd.Run() == nil {
		delCmd := exec.Command("git", "branch", "-D", branchName)
		delCmd.Dir = repoRoot
		if delCmd.Run() == nil {
			branchDeleted = true
			color.Green("   ✓ 已删除分支 %s（改动已丢弃）", branchName)
		}
	} else {
		branchDeleted = true
		color.Green("   ✓ 分支 %s 不存在，无需清理", branchName)
	}

	// 删除 code-result.json
	_ = os.Remove(filepath.Join(task.TaskDir, "code-result.json"))

	// 回退状态到 planned
	if err := prd.ResetTaskToPlanned(task.TaskDir, time.Now()); err != nil {
		return err
	}
	color.Green("   ✓ 状态已回退为 planned")

	result := map[string]any{
		"status":           "reset",
		"task_id":          taskID,
		"branch":           branchName,
		"worktree":         worktreePath,
		"worktree_deleted": worktreeDeleted,
		"branch_deleted":   branchDeleted,
		"message":          "task 已重置为 planned 状态，可重新执行 prd code。",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	return nil
}
