package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
)

type RepoBinding struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Status   string `json:"status,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Commit   string `json:"commit,omitempty"`
}

type ReposMetadata struct {
	Repos []RepoBinding `json:"repos"`
}

type taskDirEntry struct {
	id      string
	path    string
	modTime int64
}

func globalTasksRoot() string {
	return filepath.Join(config.DefaultConfigDir(), "tasks")
}

func ensureGlobalTasksRoot() error {
	if err := os.MkdirAll(globalTasksRoot(), 0755); err != nil {
		return fmt.Errorf("创建全局 tasks 目录失败: %w", err)
	}
	return nil
}

func createTaskDir(taskID string) (string, error) {
	if err := ensureGlobalTasksRoot(); err != nil {
		return "", err
	}

	taskDir := filepath.Join(globalTasksRoot(), taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return "", fmt.Errorf("创建 task 目录失败: %w", err)
	}
	return taskDir, nil
}

func findTaskDir(_ string, taskID string) (string, error) {
	taskDir := filepath.Join(globalTasksRoot(), taskID)
	info, err := os.Stat(taskDir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", os.ErrNotExist
	}
	return taskDir, nil
}

func listTaskDirs(_ string) ([]taskDirEntry, error) {
	entries, err := os.ReadDir(globalTasksRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取全局 tasks 目录失败: %w", err)
	}

	taskDirs := make([]taskDirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		taskDirs = append(taskDirs, taskDirEntry{
			id:      entry.Name(),
			path:    filepath.Join(globalTasksRoot(), entry.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}

	sort.Slice(taskDirs, func(i, j int) bool {
		if taskDirs[i].modTime == taskDirs[j].modTime {
			return taskDirs[i].id > taskDirs[j].id
		}
		return taskDirs[i].modTime > taskDirs[j].modTime
	})
	return taskDirs, nil
}

func reposMetadataPath(taskDir string) string {
	return filepath.Join(taskDir, "repos.json")
}

func repoCodeResultPath(taskDir, repoID string) string {
	return filepath.Join(taskDir, "code-results", sanitizeRepoResultFileName(repoID)+".json")
}

func initTaskRepos(taskDir, repoRoot string, extraRepoPaths []string, autoAddRepo bool) error {
	repos := make([]RepoBinding, 0, len(extraRepoPaths)+1)
	seen := make(map[string]bool, len(extraRepoPaths)+1)
	if autoAddRepo {
		repoID := deriveRepoID(repoRoot)
		repos = append(repos, RepoBinding{
			ID:     repoID,
			Path:   repoRoot,
			Status: TaskStatusInitialized,
		})
		seen[repoID] = true
	}
	for _, repoPath := range extraRepoPaths {
		repoPath = filepath.Clean(strings.TrimSpace(repoPath))
		if repoPath == "" {
			continue
		}
		id := deriveRepoID(repoPath)
		if seen[id] {
			continue
		}
		seen[id] = true
		repos = append(repos, RepoBinding{
			ID:     id,
			Path:   repoPath,
			Status: TaskStatusInitialized,
		})
	}
	if len(repos) == 0 {
		return fmt.Errorf("至少需要一个关联 repo")
	}
	meta := ReposMetadata{
		Repos: repos,
	}
	if err := writeJSONFile(reposMetadataPath(taskDir), meta); err != nil {
		return err
	}
	return syncTaskMetadataFromRepos(taskDir)
}

func readReposMetadata(path string) (*ReposMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta ReposMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func updateReposMetadata(taskDir string, mutator func(*ReposMetadata)) error {
	path := reposMetadataPath(taskDir)

	meta, err := readReposMetadata(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		meta = &ReposMetadata{}
	}

	mutator(meta)
	return writeJSONFile(path, meta)
}

func syncRepoBindingFromCodeResult(taskDir string, report CodeResultReport) error {
	if strings.TrimSpace(report.RepoID) == "" {
		return nil
	}

	if err := updateReposMetadata(taskDir, func(meta *ReposMetadata) {
		found := false
		for i := range meta.Repos {
			if meta.Repos[i].ID != report.RepoID {
				continue
			}
			meta.Repos[i].Path = firstNonEmpty(report.RepoPath, meta.Repos[i].Path)
			meta.Repos[i].Status = deriveRepoStatus(report)
			meta.Repos[i].Branch = report.Branch
			meta.Repos[i].Worktree = report.Worktree
			meta.Repos[i].Commit = report.Commit
			found = true
			break
		}

		if !found {
			meta.Repos = append(meta.Repos, RepoBinding{
				ID:       report.RepoID,
				Path:     report.RepoPath,
				Status:   deriveRepoStatus(report),
				Branch:   report.Branch,
				Worktree: report.Worktree,
				Commit:   report.Commit,
			})
		}
	}); err != nil {
		return err
	}
	return syncTaskMetadataFromRepos(taskDir)
}

func deriveRepoStatus(report CodeResultReport) string {
	if report.Status == "success" && report.BuildOK {
		return TaskStatusCoded
	}
	if report.Status == "build_unknown" {
		return TaskStatusFailed
	}
	if report.Status != "" {
		return report.Status
	}
	return TaskStatusPlanned
}

func deriveRepoID(repoRoot string) string {
	repoRoot = filepath.Clean(repoRoot)
	base := filepath.Base(repoRoot)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "repo"
	}
	return base
}

func syncRepoStatuses(taskDir, status string) error {
	if err := updateReposMetadata(taskDir, func(meta *ReposMetadata) {
		for i := range meta.Repos {
			meta.Repos[i].Status = status
		}
	}); err != nil {
		return err
	}
	return syncTaskMetadataFromRepos(taskDir)
}

func updateRepoBinding(taskDir, repoID string, mutator func(*RepoBinding)) error {
	if strings.TrimSpace(repoID) == "" {
		return fmt.Errorf("repo_id 不能为空")
	}

	if err := updateReposMetadata(taskDir, func(meta *ReposMetadata) {
		for i := range meta.Repos {
			if meta.Repos[i].ID == repoID {
				mutator(&meta.Repos[i])
				return
			}
		}
		meta.Repos = append(meta.Repos, RepoBinding{ID: repoID})
		mutator(&meta.Repos[len(meta.Repos)-1])
	}); err != nil {
		return err
	}
	return syncTaskMetadataFromRepos(taskDir)
}

func ResetRepoBinding(taskDir, repoID string) error {
	return updateRepoBinding(taskDir, repoID, func(repo *RepoBinding) {
		repo.Status = TaskStatusPlanned
		repo.Branch = ""
		repo.Worktree = ""
		repo.Commit = ""
	})
}

func StartCodingRepoBinding(taskDir, repoID, branchName, worktree string) error {
	return updateRepoBinding(taskDir, repoID, func(repo *RepoBinding) {
		repo.Status = TaskStatusCoding
		repo.Branch = branchName
		repo.Worktree = worktree
	})
}

func MarkRepoBindingFailed(taskDir, repoID, branchName, worktree string) error {
	return updateRepoBinding(taskDir, repoID, func(repo *RepoBinding) {
		repo.Status = TaskStatusFailed
		repo.Branch = firstNonEmpty(branchName, repo.Branch)
		repo.Worktree = firstNonEmpty(worktree, repo.Worktree)
	})
}

func ArchiveRepoBinding(taskDir, repoID string) error {
	return updateRepoBinding(taskDir, repoID, func(repo *RepoBinding) {
		repo.Status = TaskStatusArchived
	})
}

func listRepoBindings(taskDir string) ([]RepoBinding, error) {
	meta, err := readReposMetadata(reposMetadataPath(taskDir))
	if err != nil {
		return nil, err
	}
	return meta.Repos, nil
}

func RemoveRepoCodeResult(taskDir, repoID string) error {
	path := repoCodeResultPath(taskDir, repoID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func RemoveRepoCodeLog(taskDir, repoID string) error {
	path := repoCodeLogPath(taskDir, repoID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ResolveTaskRepo(taskDir, repoRoot, requestedRepoID string) (*RepoBinding, error) {
	meta, err := readReposMetadata(reposMetadataPath(taskDir))
	if err != nil {
		return nil, err
	}

	repoID := strings.TrimSpace(requestedRepoID)
	if repoID == "" {
		if len(meta.Repos) == 1 {
			repoID = meta.Repos[0].ID
		} else {
			ids := make([]string, 0, len(meta.Repos))
			for _, repo := range meta.Repos {
				ids = append(ids, repo.ID)
			}
			sort.Strings(ids)
			return nil, fmt.Errorf("该 task 关联多个 repo，请显式指定 `--repo`。可选 repo: %s", strings.Join(ids, ", "))
		}
	}

	for _, repo := range meta.Repos {
		if repo.ID != repoID {
			continue
		}
		if repo.Path != "" && filepath.Clean(repo.Path) != filepath.Clean(repoRoot) {
			return nil, fmt.Errorf("repo %s 绑定路径为 %s，但当前仓库为 %s，请切换到正确仓库目录后再执行", repoID, repo.Path, repoRoot)
		}
		resolved := repo
		return &resolved, nil
	}

	ids := make([]string, 0, len(meta.Repos))
	for _, repo := range meta.Repos {
		ids = append(ids, repo.ID)
	}
	sort.Strings(ids)
	return nil, fmt.Errorf("task 未绑定 repo %s，可选 repo: %s", repoID, strings.Join(ids, ", "))
}

func syncTaskMetadataFromRepos(taskDir string) error {
	metaPath := filepath.Join(taskDir, "task.json")
	meta, err := readTaskMetadata(metaPath)
	if err != nil {
		return err
	}

	reposMeta, err := readReposMetadata(reposMetadataPath(taskDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	meta.RepoCount = len(reposMeta.Repos)
	meta.UpdatedAt = time.Now()
	if aggregate := aggregateTaskStatus(meta.Status, reposMeta); aggregate != "" {
		meta.Status = aggregate
	}
	return writeJSONFile(metaPath, meta)
}

func aggregateTaskStatus(current string, repos *ReposMetadata) string {
	if repos == nil || len(repos.Repos) == 0 {
		return current
	}

	allArchived := true
	allPlannedLike := true
	hasCoding := false
	hasCoded := false
	hasCompleted := false
	hasFailed := false

	for _, repo := range repos.Repos {
		switch repo.Status {
		case TaskStatusArchived:
			hasCompleted = true
			allPlannedLike = false
		case TaskStatusCoded:
			hasCoded = true
			hasCompleted = true
			allArchived = false
			allPlannedLike = false
		case TaskStatusCoding:
			hasCoding = true
			allArchived = false
			allPlannedLike = false
		case TaskStatusFailed:
			hasFailed = true
			allArchived = false
			allPlannedLike = false
		case TaskStatusInitialized, TaskStatusRefined, TaskStatusPlanned, "", "pending":
			allArchived = false
		default:
			allArchived = false
			allPlannedLike = false
		}
	}

	switch {
	case allArchived:
		return TaskStatusArchived
	case hasFailed:
		return TaskStatusFailed
	case hasCoding:
		return TaskStatusCoding
	case hasCompleted && !allCodedOrArchived(repos):
		return TaskStatusPartiallyCoded
	case hasCoded && !allCodedOrArchived(repos):
		return TaskStatusPartiallyCoded
	case hasCoded && allCodedOrArchived(repos):
		return TaskStatusCoded
	case allPlannedLike:
		if current == TaskStatusInitialized || current == TaskStatusRefined || current == TaskStatusPlanning || current == TaskStatusPlanned {
			return current
		}
		return TaskStatusPlanned
	default:
		return current
	}
}

func allCodedOrArchived(repos *ReposMetadata) bool {
	for _, repo := range repos.Repos {
		if repo.Status != TaskStatusCoded && repo.Status != TaskStatusArchived {
			return false
		}
	}
	return len(repos.Repos) > 0
}
