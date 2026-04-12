package prd

import "fmt"

type ArchiveCodeResult struct {
	TaskID          string
	RepoID          string
	Branch          string
	Worktree        string
	WorktreeDeleted bool
	BranchDeleted   bool
}

func ArchiveCodeForRepo(repoRoot, taskID, repoID string) (*ArchiveCodeResult, error) {
	task, err := LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	if task.Metadata.Status != TaskStatusCoded {
		return nil, fmt.Errorf("task 状态为 %s，仅 coded 状态可归档", task.Metadata.Status)
	}

	repo, err := ResolveTaskRepo(task.TaskDir, repoRoot, repoID)
	if err != nil {
		return nil, err
	}

	result := &ArchiveCodeResult{
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

	if err := ArchiveRepoBinding(task.TaskDir, repo.ID); err != nil {
		return nil, err
	}

	return result, nil
}
