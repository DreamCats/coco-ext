package cmd

import "github.com/spf13/cobra"

var prdCmd = &cobra.Command{
	Use:   "prd",
	Short: "PRD refine/plan 任务流相关命令",
	Long:  "管理 PRD refine、plan、状态查看等任务流产物。",
}

func init() {
	rootCmd.AddCommand(prdCmd)
}
