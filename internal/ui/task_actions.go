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

type taskActionResponse struct {
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

func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request, taskID, action string) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	switch action {
	case "plan":
		s.handleStartPlan(w, taskID)
	case "code":
		s.handleStartCode(w, taskID)
	default:
		writeJSONError(w, http.StatusNotFound, "unknown task action")
	}
}

func (s *Server) handleStartPlan(w http.ResponseWriter, taskID string) {
	report, err := prd.LoadTaskStatus(s.repoRoot, taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	if !canStartPlan(report) {
		writeJSONError(w, http.StatusConflict, fmt.Sprintf("当前状态为 %s，不能开始 plan", report.Metadata.Status))
		return
	}

	if err := prd.StartPlanningTask(report.TaskDir, time.Now()); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	repoRoot, err := resolvePlanRepoRoot(report)
	if err != nil {
		_ = prd.ResetTaskToPlanned(report.TaskDir, time.Now())
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}

	go s.runPlanTask(taskID, report.TaskDir, repoRoot)

	writeJSON(w, http.StatusAccepted, taskActionResponse{
		TaskID: taskID,
		Status: prd.TaskStatusPlanning,
	})
}

func canStartPlan(report *prd.TaskStatusReport) bool {
	switch report.Metadata.Status {
	case prd.TaskStatusRefined:
		return true
	case prd.TaskStatusPlanned:
		return true
	case prd.TaskStatusFailed:
		return taskHasArtifact(report.Artifacts, "prd-refined.md") && (!taskHasArtifact(report.Artifacts, "design.md") || !taskHasArtifact(report.Artifacts, "plan.md"))
	default:
		return false
	}
}

func taskHasArtifact(artifacts []prd.ArtifactStatus, name string) bool {
	for _, artifact := range artifacts {
		if artifact.Name == name {
			return artifact.Exists
		}
	}
	return false
}

func appendPlanLogLine(taskDir, line string) {
	logPath := filepath.Join(taskDir, "plan.log")
	logLine := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(logLine)
}

func resolvePlanRepoRoot(report *prd.TaskStatusReport) (string, error) {
	repo, err := resolveSingleRepo(report)
	if err != nil {
		return "", err
	}
	return repo.Path, nil
}

func resolveSingleRepo(report *prd.TaskStatusReport) (*prd.RepoBinding, error) {
	if report == nil || report.Repos == nil || len(report.Repos.Repos) == 0 {
		return nil, fmt.Errorf("task 未绑定 repo，无法执行当前操作")
	}

	if len(report.Repos.Repos) > 1 {
		repoIDs := make([]string, 0, len(report.Repos.Repos))
		for _, repo := range report.Repos.Repos {
			repoIDs = append(repoIDs, repo.ID)
		}
		return nil, fmt.Errorf("当前 Web 操作暂不支持多 repo task；请先在目标仓库目录执行对应 CLI。关联 repo: %s", strings.Join(repoIDs, ", "))
	}

	repo := report.Repos.Repos[0]
	repo.Path = strings.TrimSpace(repo.Path)
	if repo.Path == "" {
		return nil, fmt.Errorf("task 绑定的 repo path 为空，无法执行当前操作")
	}
	return &repo, nil
}

func (s *Server) runPlanTask(taskID, taskDir, repoRoot string) {
	startedAt := time.Now()
	appendPlanLogLine(taskDir, "=== PLAN START ===")
	appendPlanLogLine(taskDir, fmt.Sprintf("task_id: %s", taskID))
	appendPlanLogLine(taskDir, fmt.Sprintf("task_dir: %s", taskDir))
	appendPlanLogLine(taskDir, fmt.Sprintf("repo_root: %s", repoRoot))

	if err := prd.CheckPlanPrerequisites(repoRoot, taskID); err != nil {
		appendPlanLogLine(taskDir, fmt.Sprintf("check_prerequisites_error: %v", err))
		_ = prd.MarkTaskFailed(taskDir, time.Now())
		return
	}

	if missing, err := prd.MissingContextFiles(repoRoot); err != nil {
		appendPlanLogLine(taskDir, fmt.Sprintf("context_check_error: %v", err))
	} else if len(missing) > 0 {
		appendPlanLogLine(taskDir, fmt.Sprintf("context_missing: %s", strings.Join(missing, ", ")))
	}

	appendPlanLogLine(taskDir, fmt.Sprintf("generator_init: begin (session timeout=%s)", config.ReviewPromptTimeout))
	explorer, err := generator.NewExplorer(repoRoot)
	if err != nil {
		appendPlanLogLine(taskDir, fmt.Sprintf("generator_init_error: %v", err))
		appendPlanLogLine(taskDir, "fallback_local_plan: true")
		if _, fallbackErr := prd.GeneratePlan(repoRoot, taskID, time.Now()); fallbackErr != nil {
			appendPlanLogLine(taskDir, fmt.Sprintf("fallback_local_plan_error: %v", fallbackErr))
			_ = prd.MarkTaskFailed(taskDir, time.Now())
			return
		}
		appendPlanLogLine(taskDir, "status: planned")
		appendPlanLogLine(taskDir, fmt.Sprintf("duration: %s", time.Since(startedAt).Round(time.Millisecond)))
		appendPlanLogLine(taskDir, "=== PLAN END ===")
		return
	}
	defer explorer.Close()
	appendPlanLogLine(taskDir, "generator_mode: explorer(readonly)")

	firstChunkLogged := false
	_, err = prd.GeneratePlanWithExplorer(
		explorer,
		repoRoot,
		taskID,
		time.Now(),
		func(chunk string) {
			if firstChunkLogged {
				return
			}
			firstChunkLogged = true
			appendPlanLogLine(taskDir, fmt.Sprintf("first_chunk_received: %d bytes", len(chunk)))
		},
		func(event generator.ToolEvent) {
			appendPlanLogLine(taskDir, formatPlanToolEvent(event))
		},
	)
	if err != nil {
		appendPlanLogLine(taskDir, fmt.Sprintf("generate_plan_with_explorer_error: %v", err))
		appendPlanLogLine(taskDir, "fallback_local_plan: true")
		if _, fallbackErr := prd.GeneratePlan(repoRoot, taskID, time.Now()); fallbackErr != nil {
			appendPlanLogLine(taskDir, fmt.Sprintf("fallback_local_plan_error: %v", fallbackErr))
			_ = prd.MarkTaskFailed(taskDir, time.Now())
			return
		}
	}

	appendPlanLogLine(taskDir, "status: planned")
	appendPlanLogLine(taskDir, fmt.Sprintf("duration: %s", time.Since(startedAt).Round(time.Millisecond)))
	appendPlanLogLine(taskDir, "=== PLAN END ===")
}

func formatPlanToolEvent(event generator.ToolEvent) string {
	base := fmt.Sprintf("[tool] status=%s kind=%s title=%s", event.Status, event.Kind, event.Title)
	if len(event.RawInput) == 0 {
		return base
	}
	input := strings.TrimSpace(string(event.RawInput))
	if input == "" {
		return base
	}
	return base + " input=" + input
}

func (s *Server) handleStartCode(w http.ResponseWriter, taskID string) {
	report, err := prd.LoadTaskStatus(s.repoRoot, taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	if !canStartCode(report) {
		writeJSONError(w, http.StatusConflict, fmt.Sprintf("当前状态为 %s，不能开始 code", report.Metadata.Status))
		return
	}

	repo, err := resolveSingleRepo(report)
	if err != nil {
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}

	branchName := buildWebPRDBranchName(taskID)
	if err := prd.StartCodingRepoBinding(report.TaskDir, repo.ID, branchName, ""); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.runCodeTask(taskID, report.TaskDir, repo.Path, repo.ID, branchName)

	writeJSON(w, http.StatusAccepted, taskActionResponse{
		TaskID: taskID,
		Status: prd.TaskStatusCoding,
	})
}

func canStartCode(report *prd.TaskStatusReport) bool {
	switch report.Metadata.Status {
	case prd.TaskStatusPlanned:
		return true
	case prd.TaskStatusFailed:
		return taskHasArtifact(report.Artifacts, "prd-refined.md") && taskHasArtifact(report.Artifacts, "design.md") && taskHasArtifact(report.Artifacts, "plan.md")
	default:
		return false
	}
}

func buildWebPRDBranchName(taskID string) string {
	return "prd_" + taskID
}

func (s *Server) runCodeTask(taskID, taskDir, repoRoot, repoID, branchName string) {
	if _, err := prd.ExecuteCodeForRepo(repoRoot, taskID, branchName, repoID, 2, nil, nil); err != nil {
		// ExecuteCodeForRepo 已负责写 code.log 和失败状态，这里只在极早期失败时兜底补一行。
		logPath := filepath.Join(taskDir, "code.log")
		file, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr == nil {
			defer file.Close()
			_, _ = file.WriteString(fmt.Sprintf("%s web_code_error: %v\n", time.Now().Format("2006-01-02 15:04:05"), err))
		}
	}
}
