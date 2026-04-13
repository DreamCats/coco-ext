package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "coco-ext",
	Short: "仓库级 AI 研发工作流工具",
	Long:  "coco-flow（当前技术名：coco-ext）是一个面向代码仓库的 AI 研发工作流工具，提供 context、PRD task、plan/code、review、submit/push、metrics 和本地 Web UI 能力。",
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
