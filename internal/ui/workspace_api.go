package ui

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/prd"
)

func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	tasks, err := prd.ListTasks(s.repoRoot, "")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	repoSet := map[string]bool{}
	for _, task := range tasks {
		report, err := prd.LoadTaskStatus(s.repoRoot, task.TaskID)
		if err != nil || report.Repos == nil {
			continue
		}
		for _, repo := range report.Repos.Repos {
			if strings.TrimSpace(repo.ID) != "" {
				repoSet[repo.ID] = true
			}
		}
	}

	repos := make([]string, 0, len(repoSet))
	for repoID := range repoSet {
		repos = append(repos, repoID)
	}
	sort.Strings(repos)

	writeJSON(w, http.StatusOK, WorkspaceSummary{
		RepoRoot:      s.repoRoot,
		TasksRoot:     filepath.Join(config.DefaultConfigDir(), "tasks"),
		ContextRoot:   filepath.Join(s.repoRoot, config.ContextDir),
		WorktreeRoot:  filepath.Join(filepath.Dir(s.repoRoot), ".coco-ext-worktree"),
		ReposInvolved: repos,
		TaskCount:     len(tasks),
	})
}
