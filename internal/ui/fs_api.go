package ui

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
)

func (s *Server) handleFSRoots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	roots := make([]RemoteRoot, 0, 4)
	seen := map[string]bool{}
	addRoot := func(label, path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		roots = append(roots, RemoteRoot{Label: label, Path: path})
	}

	addRoot("Current Repo", s.repoRoot)
	addRoot("Repo Parent", filepath.Dir(s.repoRoot))
	if home, err := os.UserHomeDir(); err == nil {
		addRoot("Home", home)
	}
	addRoot("Root", string(filepath.Separator))

	writeJSON(w, http.StatusOK, map[string]any{"roots": roots})
}

func (s *Server) handleFSList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	dirPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if dirPath == "" {
		writeJSONError(w, http.StatusBadRequest, "path 不能为空")
		return
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "解析路径失败")
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSONError(w, http.StatusNotFound, "目录不存在")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "path 不是目录")
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	dirs := make([]RemoteDirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		childPath := filepath.Join(absPath, name)
		dirs = append(dirs, RemoteDirEntry{
			Name:      name,
			Path:      childPath,
			IsGitRepo: internalgit.IsGitRepo(childPath),
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].IsGitRepo != dirs[j].IsGitRepo {
			return dirs[i].IsGitRepo
		}
		return dirs[i].Name < dirs[j].Name
	})

	parentPath := filepath.Dir(absPath)
	if parentPath == absPath {
		parentPath = ""
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":       absPath,
		"parentPath": parentPath,
		"entries":    dirs,
	})
}
