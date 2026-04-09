package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdArchiveTaskID string

var prdArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "归档已完成的 task：清理 worktree 和分支，更新状态",
	Long:  "清理 prd code 产生的 worktree 目录和分支，将 task 状态标记为 archived。输出 JSON 供 LLM 消费。",
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
			worktreeRemoved = true // 已不存在，视为已清理
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
		branchDeleted = true // 已不存在，视为已清理
	}

	// 更新状态
	if err := prd.ArchiveTask(task.TaskDir, time.Now()); err != nil {
		return err
	}

	// 输出 JSON
	result := map[string]any{
		"status":           "archived",
		"task_id":          taskID,
		"branch":           branchName,
		"branch_deleted":   branchDeleted,
		"worktree":         worktreePath,
		"worktree_removed": worktreeRemoved,
		"message":          "task 已归档，worktree 和分支已清理。",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	return nil
}
