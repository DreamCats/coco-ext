package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdArchiveTaskID string
var prdArchiveRepoID string

var prdArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "归档已完成的 task：清理分支，更新状态",
	Long:  "清理 prd code 产生的分支，将 task 状态标记为 archived。",
	RunE:  runPRDArchive,
}

func init() {
	prdCmd.AddCommand(prdArchiveCmd)
	prdArchiveCmd.Flags().StringVar(&prdArchiveTaskID, "task", "", "指定 task id；默认读取最近一个 task")
	prdArchiveCmd.Flags().StringVar(&prdArchiveRepoID, "repo", "", "仅归档指定 repo_id；不传则归档整个 task")
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
	if prdArchiveRepoID == "" && task.Metadata.Status != prd.TaskStatusCoded && task.Metadata.Status != prd.TaskStatusArchived {
		return fmt.Errorf("task 状态为 %s，仅 coded / archived 状态可归档", task.Metadata.Status)
	}

	color.Cyan("📦 PRD Archive")
	color.Cyan("   task_id: %s", taskID)
	if prdArchiveRepoID != "" {
		color.Cyan("   repo: %s", prdArchiveRepoID)
	}

	branchName := ""
	worktreePath := ""
	worktreeDeleted := false
	branchDeleted := false

	if prdArchiveRepoID == "" {
		for _, repo := range task.Repos.Repos {
			if repo.Worktree != "" {
				if err := prd.CleanupCodeWorktree(repoRoot, repo.Worktree); err != nil {
					color.Yellow("   ⚠ 删除 worktree 失败: %v", err)
				} else {
					worktreeDeleted = true
					color.Green("   ✓ 已删除 worktree %s", repo.Worktree)
				}
			}
			if repo.Branch != "" {
				deleted := deleteBranchQuiet(repoRoot, repo.Branch)
				branchDeleted = branchDeleted || deleted
				if deleted {
					color.Green("   ✓ 已删除分支 %s", repo.Branch)
				}
			}
		}
		if err := prd.ArchiveTask(task.TaskDir, time.Now()); err != nil {
			return err
		}
		color.Green("   ✓ 状态已更新为 archived")
	} else {
		repo, err := prd.ResolveTaskRepo(task.TaskDir, repoRoot, prdArchiveRepoID)
		if err != nil {
			return err
		}
		branchName = repo.Branch
		worktreePath = repo.Worktree
		if repo.Worktree != "" {
			if err := prd.CleanupCodeWorktree(repoRoot, repo.Worktree); err != nil {
				color.Yellow("   ⚠ 删除 worktree 失败: %v", err)
			} else {
				worktreeDeleted = true
				color.Green("   ✓ 已删除 worktree %s", repo.Worktree)
			}
		}
		if repo.Branch != "" {
			branchDeleted = deleteBranchQuiet(repoRoot, repo.Branch)
			if branchDeleted {
				color.Green("   ✓ 已删除分支 %s", repo.Branch)
			}
		}
		if err := prd.ArchiveRepoBinding(task.TaskDir, repo.ID); err != nil {
			return err
		}
		color.Green("   ✓ repo %s 状态已更新为 archived", repo.ID)
	}

	result := map[string]any{
		"status":           "archived",
		"task_id":          taskID,
		"repo":             prdArchiveRepoID,
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
