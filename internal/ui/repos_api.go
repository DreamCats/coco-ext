package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

type validateRepoRequest struct {
	Path string `json:"path"`
}

func (s *Server) handleRecentRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	repos, err := s.collectRecentRepos()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repos": repos})
}

func (s *Server) handleValidateRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req validateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	repo, err := validateRepoPath(req.Path)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) collectRecentRepos() ([]RepoCandidate, error) {
	tasks, err := prd.ListTasks(s.repoRoot, "")
	if err != nil {
		return nil, err
	}

	type aggregate struct {
		repo       RepoCandidate
		lastSeenTS int64
	}

	aggregates := map[string]aggregate{}
	for _, task := range tasks {
		report, err := prd.LoadTaskStatus(s.repoRoot, task.TaskID)
		if err != nil || report.Repos == nil {
			continue
		}
		lastSeen := report.Metadata.UpdatedAt
		for _, repo := range report.Repos.Repos {
			path := strings.TrimSpace(repo.Path)
			if path == "" {
				continue
			}
			current := aggregates[repo.ID]
			candidate := RepoCandidate{
				ID:          repo.ID,
				DisplayName: firstNonEmptyString(repo.ID, filepath.Base(path)),
				Path:        path,
				TaskCount:   current.repo.TaskCount + 1,
				LastSeenAt:  lastSeen.Format("2006-01-02 15:04"),
			}
			if current.lastSeenTS > lastSeen.UnixNano() {
				candidate.LastSeenAt = current.repo.LastSeenAt
			}
			ts := maxInt64(current.lastSeenTS, lastSeen.UnixNano())
			aggregates[repo.ID] = aggregate{repo: candidate, lastSeenTS: ts}
		}
	}

	repos := make([]RepoCandidate, 0, len(aggregates))
	for _, item := range aggregates {
		repos = append(repos, item.repo)
	}
	sort.Slice(repos, func(i, j int) bool {
		if repos[i].LastSeenAt == repos[j].LastSeenAt {
			if repos[i].TaskCount == repos[j].TaskCount {
				return repos[i].DisplayName < repos[j].DisplayName
			}
			return repos[i].TaskCount > repos[j].TaskCount
		}
		return repos[i].LastSeenAt > repos[j].LastSeenAt
	})
	return repos, nil
}

func validateRepoPath(path string) (*RepoCandidate, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("repo 路径不能为空")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析 repo 路径失败: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("repo 路径不存在: %s", absPath)
		}
		return nil, fmt.Errorf("读取 repo 路径失败: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repo 路径不是目录: %s", absPath)
	}
	if !internalgit.IsGitRepo(absPath) {
		return nil, fmt.Errorf("目录不是 git 仓库: %s", absPath)
	}

	return &RepoCandidate{
		ID:          filepath.Base(absPath),
		DisplayName: filepath.Base(absPath),
		Path:        absPath,
	}, nil
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func firstNonEmptyString(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}
