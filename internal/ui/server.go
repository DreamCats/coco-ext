package ui

import (
	"fmt"
	"net/http"
	"os"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
)

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
