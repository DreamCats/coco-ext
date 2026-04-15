package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var (
	prdListStatus   string
	prdListJSONOnly bool
)

var prdListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有 prd task",
	Long:  "扫描全局 tasks 目录下所有 task，按时间排序展示概要信息。支持 --status 过滤、--json 结构化输出。",
	RunE:  runPRDList,
}

func init() {
	prdCmd.AddCommand(prdListCmd)
	prdListCmd.Flags().StringVar(&prdListStatus, "status", "", "按状态过滤（initialized/refined/planned/failed/coding/partially_coded/coded/archived）")
	prdListCmd.Flags().BoolVar(&prdListJSONOnly, "json", false, "输出 JSON 格式（供 LLM 消费）")
}

func runPRDList(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	tasks, err := prd.ListTasks(repoRoot, prdListStatus)
	if err != nil {
		return err
	}

	if prdListJSONOnly {
		data, _ := json.MarshalIndent(tasks, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if len(tasks) == 0 {
		color.Yellow("没有找到任何 task。")
		if prdListStatus != "" {
			color.Yellow("当前过滤状态: %s", prdListStatus)
		}
		return nil
	}

	color.Cyan("📋 PRD Tasks (%d 个)", len(tasks))
	fmt.Println()

	for _, t := range tasks {
		statusColor := color.CyanString
		switch t.Status {
		case prd.TaskStatusArchived:
			statusColor = color.WhiteString
		case prd.TaskStatusCoded:
			statusColor = color.GreenString
		case prd.TaskStatusPartiallyCoded:
			statusColor = color.MagentaString
		case prd.TaskStatusPlanned:
			statusColor = color.YellowString
		}

		fmt.Printf("  %s  %s  %s\n",
			color.CyanString(t.TaskID),
			statusColor("[%s]", t.Status),
			t.Title,
		)
		fmt.Printf("    created: %s  source: %s  repos: %d\n",
			t.CreatedAt.Format("2006-01-02 15:04"),
			t.SourceType,
			t.RepoCount,
		)
		fmt.Println()
	}

	return nil
}
