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

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdResetTaskID string
var prdResetRepoID string

var prdResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "重置 task 的 code 状态，回退分支改动，可重新执行 prd code",
	Long:  "将 coded 状态的 task 回退到 planned，回退分支上的 auto-commit，删除 code-result.json。",
	RunE:  runPRDReset,
}

func init() {
	prdCmd.AddCommand(prdResetCmd)
	prdResetCmd.Flags().StringVar(&prdResetTaskID, "task", "", "指定 task id；默认读取最近一个 task")
	prdResetCmd.Flags().StringVar(&prdResetRepoID, "repo", "", "仅重置指定 repo_id；不传则重置整个 task")
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

	switch task.Metadata.Status {
	case prd.TaskStatusCoded, prd.TaskStatusCoding, prd.TaskStatusPartiallyCoded, prd.TaskStatusFailed:
	default:
		return fmt.Errorf("task 状态为 %s，仅 coded / coding / partially_coded / failed 状态可重置", task.Metadata.Status)
	}

	color.Cyan("🔄 PRD Reset")
	color.Cyan("   task_id: %s", taskID)
	if prdResetRepoID != "" {
		color.Cyan("   repo: %s", prdResetRepoID)
	}

	branchDeleted := false
	worktreeDeleted := false
	branchName := ""
	worktreePath := ""

	if prdResetRepoID == "" {
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
					color.Green("   ✓ 已删除分支 %s（改动已丢弃）", repo.Branch)
				}
			}
		}
		_ = os.Remove(filepath.Join(task.TaskDir, "code-result.json"))
		_ = os.RemoveAll(filepath.Join(task.TaskDir, "code-results"))
		if err := prd.ResetTaskToPlanned(task.TaskDir, time.Now()); err != nil {
			return err
		}
		color.Green("   ✓ 状态已回退为 planned")
	} else {
		repo, err := prd.ResolveTaskRepo(task.TaskDir, repoRoot, prdResetRepoID)
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
				color.Green("   ✓ 已删除分支 %s（改动已丢弃）", repo.Branch)
			}
		}
		_ = prd.RemoveRepoCodeResult(task.TaskDir, repo.ID)
		if err := prd.ResetRepoBinding(task.TaskDir, repo.ID); err != nil {
			return err
		}
		color.Green("   ✓ repo %s 状态已回退为 planned", repo.ID)
	}

	result := map[string]any{
		"status":           "reset",
		"task_id":          taskID,
		"repo":             prdResetRepoID,
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

func deleteBranchQuiet(repoRoot, branchName string) bool {
	if strings.TrimSpace(branchName) == "" {
		return false
	}
	checkCmd := exec.Command("git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = repoRoot
	if checkCmd.Run() != nil {
		return true
	}
	delCmd := exec.Command("git", "branch", "-D", branchName)
	delCmd.Dir = repoRoot
	return delCmd.Run() == nil
}
