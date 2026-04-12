package prd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ResetCodeResult struct {
	TaskID          string
	RepoID          string
	Branch          string
	Worktree        string
	WorktreeDeleted bool
	BranchDeleted   bool
}

func ResetCodeForRepo(repoRoot, taskID, repoID string) (*ResetCodeResult, error) {
	task, err := LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	repo, err := ResolveTaskRepo(task.TaskDir, repoRoot, repoID)
	if err != nil {
		return nil, err
	}

	switch repo.Status {
	case TaskStatusCoded, TaskStatusFailed:
	default:
		return nil, fmt.Errorf("repo %s 当前状态为 %s，仅 coded / failed 状态可重置", repo.ID, repo.Status)
	}

	result := &ResetCodeResult{
		TaskID:   taskID,
		RepoID:   repo.ID,
		Branch:   repo.Branch,
		Worktree: repo.Worktree,
	}

	if repo.Worktree != "" {
		if err := CleanupCodeWorktree(repoRoot, repo.Worktree); err == nil {
			result.WorktreeDeleted = true
		}
	}

	if repo.Branch != "" {
		result.BranchDeleted = DeleteBranchQuiet(repoRoot, repo.Branch)
	}

	_ = RemoveRepoCodeResult(task.TaskDir, repo.ID)
	_ = RemoveRepoDiffArtifacts(task.TaskDir, repo.ID)
	_ = os.Remove(filepath.Join(task.TaskDir, "code-result.json"))
	_ = os.Remove(filepath.Join(task.TaskDir, "code.log"))

	if err := ResetRepoBinding(task.TaskDir, repo.ID); err != nil {
		return nil, err
	}

	return result, nil
}

func DeleteBranchQuiet(repoRoot, branchName string) bool {
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
