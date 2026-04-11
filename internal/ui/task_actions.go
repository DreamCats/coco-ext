package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

func appendRefineLogLine(taskDir, line string) {
	logPath := filepath.Join(taskDir, "refine.log")
	logLine := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(logLine)
}

func (s *Server) runRefineTask(repoRoot string, task *prd.RefineTask) {
	startedAt := time.Now()
	appendRefineLogLine(task.TaskDir, "=== REFINE START ===")
	appendRefineLogLine(task.TaskDir, fmt.Sprintf("task_id: %s", task.TaskID))
	appendRefineLogLine(task.TaskDir, fmt.Sprintf("task_dir: %s", task.TaskDir))
	appendRefineLogLine(task.TaskDir, fmt.Sprintf("source_type: %s", task.Source.Type))
	appendRefineLogLine(task.TaskDir, fmt.Sprintf("supports_refine: %t", task.SupportsRefine))
	if !task.SupportsRefine {
		appendRefineLogLine(task.TaskDir, "source_content_missing: skip AI refine and write pending refined content")
		if err := prd.WriteRefinedContent(task, prd.BuildPendingRefinedContent(task), time.Now(), prd.TaskStatusInitialized); err != nil {
			appendRefineLogLine(task.TaskDir, fmt.Sprintf("write_pending_refined_error: %v", err))
			return
		}
		appendRefineLogLine(task.TaskDir, "status: initialized")
		appendRefineLogLine(task.TaskDir, fmt.Sprintf("duration: %s", time.Since(startedAt).Round(time.Millisecond)))
		return
	}

	appendRefineLogLine(task.TaskDir, fmt.Sprintf("generator_init: begin (session timeout=%s)", config.ContextPromptTimeout))
	gen, err := generator.NewPromptOnly(repoRoot)
	if err != nil {
		appendRefineLogLine(task.TaskDir, fmt.Sprintf("generator_init_error: %v", err))
		fallback := prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, err)
		if writeErr := prd.WriteRefinedContent(task, fallback, time.Now(), prd.TaskStatusRefined); writeErr != nil {
			appendRefineLogLine(task.TaskDir, fmt.Sprintf("write_fallback_refined_error: %v", writeErr))
			return
		}
		appendRefineLogLine(task.TaskDir, "fallback_written: true")
		appendRefineLogLine(task.TaskDir, "status: refined")
		appendRefineLogLine(task.TaskDir, fmt.Sprintf("duration: %s", time.Since(startedAt).Round(time.Millisecond)))
		return
	}
	defer gen.Close()
	appendRefineLogLine(task.TaskDir, "generator_mode: prompt_only+yolo")

	appendRefineLogLine(task.TaskDir, fmt.Sprintf("prompt_start: total_timeout=%s idle_timeout=%s", config.RefinePromptTimeout, config.RefineChunkIdleTimeout))
	firstChunkLogged := false
	refinedContent, promptErr := gen.PromptWithIdleTimeout(
		prd.BuildRefinedPrompt(task.Title, task.Source.Content),
		config.RefinePromptTimeout,
		config.RefineChunkIdleTimeout,
		func(chunk string) {
			if firstChunkLogged {
				return
			}
			firstChunkLogged = true
			appendRefineLogLine(task.TaskDir, fmt.Sprintf("first_chunk_received: %d bytes", len(chunk)))
		},
	)
	if promptErr != nil {
		appendRefineLogLine(task.TaskDir, fmt.Sprintf("prompt_error: %v", promptErr))
		appendRefineLogLine(task.TaskDir, fmt.Sprintf("prompt_partial_bytes: %d", len(refinedContent)))
	} else {
		appendRefineLogLine(task.TaskDir, fmt.Sprintf("prompt_ok: %d bytes", len(refinedContent)))
	}
	finalContent, usedFallback, note := prd.ResolveRefinedContent(task.Title, task.Source.Content, refinedContent, promptErr)
	refinedContent = finalContent
	if usedFallback {
		appendRefineLogLine(task.TaskDir, "fallback_written: true")
		if note != nil {
			appendRefineLogLine(task.TaskDir, fmt.Sprintf("fallback_reason: %v", note))
		}
	} else {
		appendRefineLogLine(task.TaskDir, "validate_ok: true")
		if note != nil {
			appendRefineLogLine(task.TaskDir, fmt.Sprintf("partial_output_preserved_after_error: %v", note))
		}
	}
	if err := prd.WriteRefinedContent(task, refinedContent, time.Now(), prd.TaskStatusRefined); err != nil {
		appendRefineLogLine(task.TaskDir, fmt.Sprintf("write_refined_error: %v", err))
		return
	}
	appendRefineLogLine(task.TaskDir, "status: refined")
	appendRefineLogLine(task.TaskDir, fmt.Sprintf("duration: %s", time.Since(startedAt).Round(time.Millisecond)))
	appendRefineLogLine(task.TaskDir, "=== REFINE END ===")
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
