package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
	"github.com/DreamCats/coco-ext/internal/prd"
)

type createTaskRequest struct {
	Input string   `json:"input"`
	Title string   `json:"title"`
	Repos []string `json:"repos"`
}

type createTaskResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.Input = strings.TrimSpace(req.Input)
	req.Title = strings.TrimSpace(req.Title)
	req.Repos = normalizeRepoPaths(req.Repos)
	if req.Input == "" {
		writeJSONError(w, http.StatusBadRequest, "input 不能为空")
		return
	}
	if len(req.Repos) == 0 {
		writeJSONError(w, http.StatusBadRequest, "web ui 创建 task 时必须至少指定一个 repo")
		return
	}

	task, err := prd.PrepareRefineTask(s.repoRoot, prd.RefineInput{
		RawInput:    req.Input,
		Title:       req.Title,
		RepoPaths:   req.Repos,
		AutoAddRepo: false,
		Now:         time.Now(),
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	go s.runRefineTask(s.repoRoot, task)

	writeJSON(w, http.StatusAccepted, createTaskResponse{
		TaskID: task.TaskID,
		Status: prd.TaskStatusInitialized,
	})
}

func normalizeRepoPaths(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		normalized = append(normalized, path)
	}
	return normalized
}

func (s *Server) runRefineTask(repoRoot string, task *prd.RefineTask) {
	now := time.Now()
	if !task.SupportsRefine {
		_ = prd.WriteRefinedContent(task, prd.BuildPendingRefinedContent(task), now, prd.TaskStatusInitialized)
		return
	}

	gen, err := generator.New(repoRoot)
	if err != nil {
		fallback := prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, err)
		_ = prd.WriteRefinedContent(task, fallback, time.Now(), prd.TaskStatusRefined)
		return
	}
	defer gen.Close()

	refinedContent, promptErr := gen.PromptWithTimeout(
		prd.BuildRefinedPrompt(task.Title, task.Source.Content),
		config.ReviewPromptTimeout,
		nil,
	)
	if promptErr != nil {
		refinedContent = prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, promptErr)
	} else {
		refinedContent = prd.ExtractRefinedContent(refinedContent)
		if validateErr := prd.ValidateRefinedContent(refinedContent); validateErr != nil {
			refinedContent = prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, validateErr)
		}
	}
	_ = prd.WriteRefinedContent(task, refinedContent, time.Now(), prd.TaskStatusRefined)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request, taskID string) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	report, err := prd.LoadTaskStatus(s.repoRoot, taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	switch report.Metadata.Status {
	case prd.TaskStatusInitialized, prd.TaskStatusRefined, prd.TaskStatusPlanned, prd.TaskStatusFailed:
	default:
		writeJSONError(w, http.StatusConflict, fmt.Sprintf("当前状态为 %s，仅允许删除未进入 code 的 task", report.Metadata.Status))
		return
	}

	if err := os.RemoveAll(report.TaskDir); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"task_id": taskID,
		"status":  "deleted",
	})
}

func parseTaskIDFromPath(path string) string {
	return strings.Trim(strings.TrimPrefix(path, "/api/tasks/"), "/")
}
