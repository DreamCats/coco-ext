package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-acp-sdk/daemon"
	"github.com/DreamCats/coco-repo/internal/config"
)

var daemonCwd string

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "daemon 管理（内部命令）",
	Hidden: true,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 daemon 服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		if daemonCwd == "" {
			daemonCwd = "."
		}

		server := daemon.NewServer(configDir, daemonCwd)
		return server.Run()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看 daemon 状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		if daemon.IsRunningAt(configDir) {
			conn, err := daemon.Dial(".", &daemon.DialOption{ConfigDir: configDir})
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := conn.Status()
			if err != nil {
				return err
			}
			fmt.Printf("daemon 运行中 (pid=%d, session=%s, model=%s, uptime=%s)\n",
				resp.PID, resp.SessionID, resp.ModelID, resp.Uptime)
		} else {
			fmt.Println("daemon 未运行")
		}
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止 daemon 服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		if !daemon.IsRunningAt(configDir) {
			fmt.Println("daemon 未运行")
			return nil
		}

		conn, err := daemon.Dial(".", &daemon.DialOption{ConfigDir: configDir})
		if err != nil {
			return err
		}
		defer conn.Close()

		if err := conn.Shutdown(); err != nil {
			return err
		}
		fmt.Println("daemon 已停止")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonStartCmd.Flags().StringVar(&daemonCwd, "cwd", "", "工作目录")
}
