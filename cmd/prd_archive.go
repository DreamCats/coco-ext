package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdArchiveTaskID string

var prdArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "归档已完成的 task：清理分支，更新状态",
	Long:  "清理 prd code 产生的分支，将 task 状态标记为 archived。",
	RunE:  runPRDArchive,
}

func init() {
	prdCmd.AddCommand(prdArchiveCmd)
	prdArchiveCmd.Flags().StringVar(&prdArchiveTaskID, "task", "", "指定 task id；默认读取最近一个 task")
}

func runPRDArchive(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	taskID, err := prd.ResolveTaskID(repoRoot, prdArchiveTaskID)
	if err != nil {
		return err
	}

	task, err := prd.LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return err
	}

	color.Cyan("📦 PRD Archive")
	color.Cyan("   task_id: %s", taskID)

	branchName := "prd/" + taskID
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

	// 清理分支
	branchDeleted := false
	checkCmd := exec.Command("git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = repoRoot
	if checkCmd.Run() == nil {
		delCmd := exec.Command("git", "branch", "-D", branchName)
		delCmd.Dir = repoRoot
		if delCmd.Run() == nil {
			branchDeleted = true
			color.Green("   ✓ 已删除分支 %s", branchName)
		}
	} else {
		branchDeleted = true
	}

	// 更新状态
	if err := prd.ArchiveTask(task.TaskDir, time.Now()); err != nil {
		return err
	}
	color.Green("   ✓ 状态已更新为 archived")

	result := map[string]any{
		"status":           "archived",
		"task_id":          taskID,
		"branch":           branchName,
		"worktree":         worktreePath,
		"worktree_deleted": worktreeDeleted,
		"branch_deleted":   branchDeleted,
		"message":          "task 已归档，分支已清理。",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	return nil
}
