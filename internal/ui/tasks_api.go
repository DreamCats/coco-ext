package ui

import (
	"net/http"
	"os"
	"strings"

	"github.com/DreamCats/coco-ext/internal/prd"
)

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	tasks, err := prd.ListTasks(s.repoRoot, "")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]TaskListItem, 0, len(tasks))
	for _, task := range tasks {
		report, err := prd.LoadTaskStatus(s.repoRoot, task.TaskID)
		if err != nil {
			continue
		}
		items = append(items, TaskListItem{
			ID:         task.TaskID,
			Title:      task.Title,
			Status:     task.Status,
			SourceType: string(task.SourceType),
			UpdatedAt:  task.UpdatedAt.Format("2006-01-02 15:04"),
			RepoCount:  task.RepoCount,
			RepoIDs:    collectRepoIDs(report.Repos),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"tasks": items})
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	taskID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/tasks/"), "/")
	if taskID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing task id")
		return
	}

	report, err := prd.LoadTaskStatus(s.repoRoot, taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	complexity, _, err := prd.ReadTaskComplexity(report.TaskDir)
	if err != nil && !os.IsNotExist(err) {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if complexity == "" {
		complexity = "未评估"
	}

	writeJSON(w, http.StatusOK, TaskDetail{
		ID:         report.TaskID,
		Title:      report.Metadata.Title,
		Status:     report.Metadata.Status,
		SourceType: string(report.Metadata.SourceType),
		UpdatedAt:  report.Metadata.UpdatedAt.Format("2006-01-02 15:04"),
		Owner:      localOwner(),
		Complexity: complexity,
		NextAction: report.NextCommand,
		RepoNext:   report.RepoNext,
		Repos:      buildRepoViews(report.TaskDir, report.Repos),
		Timeline:   buildTimeline(report.Metadata.Status),
		Artifacts:  readTaskArtifacts(report.TaskDir),
	})
}
