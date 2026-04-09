package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdResetTaskID string

var prdResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "重置 task 的 code 状态，清理 worktree 和分支，可重新执行 prd code",
	Long:  "将 coded/build_failed 状态的 task 回退到 planned，清理 worktree 目录和分支，删除 code-result.json。",
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

	// 从 code-result.json 读取 worktree 和 branch 信息
	var worktreePath, branchName string
	var worktreeRemoved, branchDeleted bool

	report, _ := prd.ReadCodeResultReport(task.TaskDir)
	if report != nil {
		worktreePath = report.Worktree
		branchName = report.Branch
	}
	if branchName == "" {
		branchName = "prd/" + taskID
	}

	// 清理 worktree
	if worktreePath != "" {
		if _, err := os.Stat(worktreePath); err == nil {
			rmCmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
			rmCmd.Dir = repoRoot
			if err := rmCmd.Run(); err == nil {
				worktreeRemoved = true
			}
		} else {
			worktreeRemoved = true
		}
	}

	// 清理分支
	checkCmd := exec.Command("git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = repoRoot
	if err := checkCmd.Run(); err == nil {
		delCmd := exec.Command("git", "branch", "-D", branchName)
		delCmd.Dir = repoRoot
		if err := delCmd.Run(); err == nil {
			branchDeleted = true
		}
	} else {
		branchDeleted = true
	}

	// 删除 code-result.json
	_ = os.Remove(filepath.Join(task.TaskDir, "code-result.json"))

	// 回退状态到 planned
	if err := prd.ResetTaskToPlanned(task.TaskDir, time.Now()); err != nil {
		return err
	}

	result := map[string]any{
		"status":           "reset",
		"task_id":          taskID,
		"branch":           branchName,
		"branch_deleted":   branchDeleted,
		"worktree":         worktreePath,
		"worktree_removed": worktreeRemoved,
		"message":          "task 已重置为 planned 状态，可重新执行 prd code。",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	return nil
}
