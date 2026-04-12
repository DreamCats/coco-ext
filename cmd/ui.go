package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/ui"
)

var (
	uiHost   string
	uiPort   int
	uiWebDir string
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "启动本地 Web UI",
}

var uiServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动本地 Web UI 服务",
	Long:  "启动本地 HTTP 服务，提供 PRD task 的 tasks/detail/workspace API，以及创建 task、触发 plan、按 repo 推进 code/reset/archive 等交互能力。默认优先使用内嵌静态前端；开发态可通过 --web-dir 覆盖。",
	RunE:  runUIServe,
}

func init() {
	rootCmd.AddCommand(uiCmd)
	uiCmd.AddCommand(uiServeCmd)

	uiServeCmd.Flags().StringVar(&uiHost, "host", "127.0.0.1", "监听地址")
	uiServeCmd.Flags().IntVar(&uiPort, "port", 4317, "监听端口")
	uiServeCmd.Flags().StringVar(&uiWebDir, "web-dir", "", "开发态静态前端目录；为空时优先使用内嵌静态资源")
}

func runUIServe(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	webDir := strings.TrimSpace(uiWebDir)
	if webDir != "" && !filepath.IsAbs(webDir) {
		webDir = filepath.Join(repoRoot, webDir)
	}

	server, err := ui.NewServer(repoRoot, webDir)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", uiHost, uiPort)
	color.Cyan("🌐 UI Serve")
	color.Cyan("   repo: %s", repoRoot)
	color.Cyan("   addr: http://%s", addr)
	if info, err := os.Stat(webDir); webDir != "" && err == nil && info.IsDir() {
		color.Cyan("   web: %s", webDir)
	} else {
		color.Cyan("   web: embedded")
	}

	return http.ListenAndServe(addr, server.Handler())
}
