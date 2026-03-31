package cmd

import "github.com/spf13/cobra"

var prdCmd = &cobra.Command{
	Use:   "prd",
	Short: "PRD -> MR 任务流相关命令",
	Long:  "管理 PRD refine、评估、编码、MR 等任务流产物。",
}

func init() {
	rootCmd.AddCommand(prdCmd)
}
