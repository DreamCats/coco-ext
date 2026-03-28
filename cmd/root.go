package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "coco-ext",
	Short: "coco 扩展工具箱",
	Long:  "coco-ext 通过 AI 自动扫描分析仓库，生成并维护 .livecoding/context/ 目录下的业务知识文件，让 AI 编码具备业务上下文。",
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
