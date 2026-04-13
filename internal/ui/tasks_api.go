package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/prd"
)

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		s.handleCreateTask(w, r)
		return
	default:
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
	taskID, action := parseTaskPath(r.URL.Path)
	if taskID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing task id")
		return
	}
	if action != "" {
		if action == "artifact" {
			s.handleTaskArtifact(w, r, taskID)
			return
		}
		s.handleTaskAction(w, r, taskID, action)
		return
	}
	if r.Method == http.MethodDelete {
		s.handleDeleteTask(w, r, taskID)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
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

func parseTaskPath(path string) (taskID string, action string) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/tasks/"), "/")
	if trimmed == "" {
		return "", ""
	}
	parts := strings.Split(trimmed, "/")
	taskID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}
	return taskID, action
}

func (s *Server) handleTaskArtifact(w http.ResponseWriter, r *http.Request, taskID string) {
	switch r.Method {
	case http.MethodGet:
	case http.MethodPut:
		s.handleUpdateTaskArtifact(w, r, taskID)
		return
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	report, err := prd.LoadTaskStatus(s.repoRoot, taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "missing artifact name")
		return
	}

	repoID := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repoID == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"task_id": taskID,
			"name":    name,
			"content": readTaskArtifact(report.TaskDir, name),
		})
		return
	}

	if _, err := resolveActionRepo(report, repoID); err != nil {
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}

	content, err := readRepoArtifact(report.TaskDir, repoID, name)
	if err != nil {
		if os.IsNotExist(err) {
			content = emptyRepoArtifactPlaceholder(name, repoID)
		} else {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"task_id": taskID,
		"repo_id": repoID,
		"name":    name,
		"content": content,
	})
}

type updateTaskArtifactRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleUpdateTaskArtifact(w http.ResponseWriter, r *http.Request, taskID string) {
	var req updateTaskArtifactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "missing artifact name")
		return
	}

	if err := prd.UpdateTaskArtifact(s.repoRoot, taskID, name, req.Content, time.Now()); err != nil {
		switch {
		case os.IsNotExist(err), strings.Contains(err.Error(), "task 不存在"):
			writeJSONError(w, http.StatusNotFound, err.Error())
		case strings.Contains(err.Error(), "不支持编辑"), strings.Contains(err.Error(), "不能编辑"), strings.Contains(err.Error(), "不能为空"):
			writeJSONError(w, http.StatusConflict, err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	report, err := prd.LoadTaskStatus(s.repoRoot, taskID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"task_id": taskID,
		"name":    name,
		"status":  report.Metadata.Status,
		"content": readTaskArtifact(report.TaskDir, name),
	})
}
