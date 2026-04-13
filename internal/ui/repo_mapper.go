package ui

import (
	"path/filepath"
	"strings"

	"github.com/DreamCats/coco-ext/internal/prd"
)

func buildRepoViews(taskDir string, repos *prd.ReposMetadata) []RepoView {
	if repos == nil {
		return nil
	}

	result := make([]RepoView, 0, len(repos.Repos))
	for _, repo := range repos.Repos {
		view := RepoView{
			ID:          repo.ID,
			DisplayName: repo.ID,
			Path:        repo.Path,
			Status:      repo.Status,
			Branch:      repo.Branch,
			Worktree:    repo.Worktree,
			Commit:      repo.Commit,
			Build:       "n/a",
		}

		if report, err := prd.ReadRepoCodeResultReport(taskDir, repo.ID); err == nil {
			if report.BuildOK {
				view.Build = "passed"
			} else {
				view.Build = "failed"
			}
			view.FailureHint = summarizeRepoFailure(report, taskDir, repo.ID)
			view.FilesWritten = cleanFilesWritten(report.FilesWritten, repo.Path, repo.Worktree)
			if view.Branch == "" {
				view.Branch = report.Branch
			}
			if view.Worktree == "" {
				view.Worktree = report.Worktree
			}
			if view.Commit == "" {
				view.Commit = report.Commit
			}
			if view.Status == "" {
				view.Status = report.Status
			}
		}

		if diffSummary, err := prd.ReadRepoDiffSummary(taskDir, repo.ID); err == nil {
			patch, _ := prd.ReadRepoDiffPatch(taskDir, repo.ID)
			view.DiffSummary = &DiffView{
				RepoID:    diffSummary.RepoID,
				Commit:    diffSummary.Commit,
				Branch:    diffSummary.Branch,
				Files:     diffSummary.Files,
				Additions: diffSummary.Additions,
				Deletions: diffSummary.Deletions,
				Patch:     patch,
			}
		}

		result = append(result, view)
	}
	return result
}

func summarizeRepoFailure(report *prd.CodeResultReport, taskDir, repoID string) string {
	if report == nil {
		return ""
	}
	if hint := strings.TrimSpace(report.Error); hint != "" {
		return hint
	}
	if report.BuildOK {
		return ""
	}

	logContent, err := prd.ReadRepoCodeLog(taskDir, repoID)
	if err != nil {
		return ""
	}

	lines := strings.Split(logContent, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "result: error") {
			return trimFailureLogLine(line)
		}
		if strings.Contains(lower, "build failed") || strings.Contains(lower, "auto_commit_error") || strings.Contains(lower, "generate_code_with_agent_error") || strings.Contains(lower, "retry_") {
			return trimFailureLogLine(line)
		}
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
			return trimFailureLogLine(line)
		}
	}
	return ""
}

func trimFailureLogLine(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > 140 {
		return line[:140] + "..."
	}
	return line
}

func collectRepoIDs(repos *prd.ReposMetadata) []string {
	if repos == nil {
		return nil
	}
	ids := make([]string, 0, len(repos.Repos))
	for _, repo := range repos.Repos {
		if strings.TrimSpace(repo.ID) != "" {
			ids = append(ids, repo.ID)
		}
	}
	return ids
}

func cleanFilesWritten(files []string, repoPath, worktree string) []string {
	if len(files) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(files))
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		file = strings.TrimPrefix(file, worktree+string(filepath.Separator))
		file = strings.TrimPrefix(file, filepath.Clean(worktree)+string(filepath.Separator))
		file = strings.TrimPrefix(file, filepath.Clean(repoPath)+string(filepath.Separator))
		file = strings.TrimPrefix(file, repoPath+string(filepath.Separator))
		file = strings.TrimSpace(file)
		if file == "" || file == filepath.Base(repoPath) || file == "." {
			continue
		}
		if !strings.Contains(file, "/") && !strings.Contains(file, ".") {
			continue
		}
		if seen[file] {
			continue
		}
		seen[file] = true
		result = append(result, file)
	}
	return result
}
