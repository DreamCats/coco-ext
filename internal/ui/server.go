package ui

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DreamCats/coco-ext/internal/config"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

type TaskListItem struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	SourceType string   `json:"sourceType"`
	UpdatedAt  string   `json:"updatedAt"`
	RepoCount  int      `json:"repoCount"`
	RepoIDs    []string `json:"repoIds"`
}

type RepoView struct {
	ID           string    `json:"id"`
	DisplayName  string    `json:"displayName"`
	Path         string    `json:"path"`
	Status       string    `json:"status"`
	Branch       string    `json:"branch,omitempty"`
	Worktree     string    `json:"worktree,omitempty"`
	Commit       string    `json:"commit,omitempty"`
	Build        string    `json:"build,omitempty"`
	FilesWritten []string  `json:"filesWritten,omitempty"`
	DiffSummary  *DiffView `json:"diffSummary,omitempty"`
}

type DiffView struct {
	RepoID    string   `json:"repoId"`
	Commit    string   `json:"commit"`
	Branch    string   `json:"branch"`
	Files     []string `json:"files"`
	Additions int      `json:"additions"`
	Deletions int      `json:"deletions"`
	Patch     string   `json:"patch"`
}

type TaskTimelineItem struct {
	Label  string `json:"label"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

type TaskDetail struct {
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Status     string             `json:"status"`
	SourceType string             `json:"sourceType"`
	UpdatedAt  string             `json:"updatedAt"`
	Owner      string             `json:"owner"`
	Complexity string             `json:"complexity"`
	NextAction string             `json:"nextAction"`
	RepoNext   []string           `json:"repoNext"`
	Repos      []RepoView         `json:"repos"`
	Timeline   []TaskTimelineItem `json:"timeline"`
	Artifacts  map[string]string  `json:"artifacts"`
}

var uiArtifactOrder = []string{
	"prd.source.md",
	"prd-refined.md",
	"design.md",
	"plan.md",
	"code-result.json",
	"code.log",
}

type WorkspaceSummary struct {
	RepoRoot      string   `json:"repoRoot"`
	TasksRoot     string   `json:"tasksRoot"`
	ContextRoot   string   `json:"contextRoot"`
	WorktreeRoot  string   `json:"worktreeRoot"`
	ReposInvolved []string `json:"reposInvolved"`
	TaskCount     int      `json:"taskCount"`
}

type Server struct {
	repoRoot string
	webDir   string
	mux      *http.ServeMux
}

func NewServer(repoRoot, webDir string) (*Server, error) {
	if !internalgit.IsGitRepo(repoRoot) {
		return nil, fmt.Errorf("当前目录不是 git 仓库")
	}

	s := &Server{
		repoRoot: repoRoot,
		webDir:   webDir,
		mux:      http.NewServeMux(),
	}
	s.registerRoutes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return withCORS(s.mux)
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/tasks", s.handleTasks)
	s.mux.HandleFunc("/api/tasks/", s.handleTaskDetail)
	s.mux.HandleFunc("/api/workspace", s.handleWorkspace)

	if info, err := os.Stat(s.webDir); err == nil && info.IsDir() {
		s.mux.Handle("/", spaHandler(s.webDir))
		return
	}
	if embeddedFS, err := embeddedStaticFS(); err == nil {
		s.mux.Handle("/", embeddedSPAHandler(embeddedFS))
	}
}

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

	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	taskID = strings.Trim(taskID, "/")
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

	detail := TaskDetail{
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
	}

	writeJSON(w, http.StatusOK, detail)
}

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

func readTaskArtifacts(taskDir string) map[string]string {
	artifacts := make(map[string]string, len(uiArtifactOrder))
	for _, name := range uiArtifactOrder {
		data, err := os.ReadFile(filepath.Join(taskDir, name))
		if err != nil {
			artifacts[name] = missingArtifactPlaceholder(name)
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			artifacts[name] = emptyArtifactPlaceholder(name)
			continue
		}
		artifacts[name] = string(data)
	}
	return artifacts
}

func buildTimeline(status string) []TaskTimelineItem {
	refineState, planState, codeState, archiveState := "pending", "pending", "pending", "pending"
	refineDetail, planDetail, codeDetail, archiveDetail := "等待 refine", "等待 plan", "等待 code", "等待 archive"

	switch status {
	case prd.TaskStatusInitialized:
		refineState = "current"
		refineDetail = "已初始化 task，等待生成 refined PRD"
	case prd.TaskStatusRefined:
		refineState, planState = "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "等待生成 design.md 与 plan.md"
	case prd.TaskStatusPlanned:
		refineState, planState, codeState = "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "可进入 code 阶段"
	case prd.TaskStatusCoding:
		refineState, planState, codeState = "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "至少一个 repo 正在执行 code"
	case prd.TaskStatusPartiallyCoded:
		refineState, planState, codeState = "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "部分 repo 已完成，仍有 repo 待推进"
	case prd.TaskStatusCoded:
		refineState, planState, codeState, archiveState = "done", "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "所有关联 repo 已完成 code"
		archiveDetail = "可归档收尾"
	case prd.TaskStatusArchived:
		refineState, planState, codeState, archiveState = "done", "done", "done", "done"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "已完成 code"
		archiveDetail = "已归档"
	case prd.TaskStatusFailed:
		refineState, planState, codeState = "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "存在失败的 repo，需继续处理"
	}

	return []TaskTimelineItem{
		{Label: "Refine", State: refineState, Detail: refineDetail},
		{Label: "Plan", State: planState, Detail: planDetail},
		{Label: "Code", State: codeState, Detail: codeDetail},
		{Label: "Archive", State: archiveState, Detail: archiveDetail},
	}
}

func collectRepoIDs(repos *prd.ReposMetadata) []string {
	if repos == nil {
		return nil
	}
	ids := make([]string, 0, len(repos.Repos))
	for _, repo := range repos.Repos {
		if strings.TrimSpace(repo.ID) == "" {
			continue
		}
		ids = append(ids, repo.ID)
	}
	return ids
}

func localOwner() string {
	if user := strings.TrimSpace(os.Getenv("USER")); user != "" {
		return user
	}
	return "local"
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

func missingArtifactPlaceholder(name string) string {
	return fmt.Sprintf("该 task 当前没有 `%s`。", name)
}

func emptyArtifactPlaceholder(name string) string {
	if name == "code.log" {
		return "当前没有可用的 code.log。可能这个 task 是旧数据，或尚未进入 code 阶段。"
	}
	return fmt.Sprintf("`%s` 当前为空。", name)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func spaHandler(root string) http.Handler {
	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(root, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	})
}

func embeddedSPAHandler(staticFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if cleanPath == "." {
			cleanPath = ""
		}
		if cleanPath != "" {
			if _, err := fs.Stat(staticFS, cleanPath); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		data, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
