package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdStatusTaskID string

var prdStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看 prd task 当前状态、产物与下一步建议",
	Long:  "默认显示最近一个 task；也可通过 --task 指定 task id。",
	RunE:  runPRDStatus,
}

func init() {
	prdCmd.AddCommand(prdStatusCmd)
	prdStatusCmd.Flags().StringVar(&prdStatusTaskID, "task", "", "指定 task id；默认显示最近一个 task")
}

func runPRDStatus(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	taskID, err := prd.ResolveTaskID(repoRoot, prdStatusTaskID)
	if err != nil {
		return err
	}

	report, err := prd.LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return err
	}

	color.Cyan("📋 PRD Task Status")
	color.Cyan("   task_id: %s", report.TaskID)
	color.Cyan("   title: %s", report.Metadata.Title)
	color.Cyan("   task status: %s", report.Metadata.Status)
	color.Cyan("   task dir: %s", report.TaskDir)
	if report.Repos != nil {
		color.Cyan("   repos: %d", len(report.Repos.Repos))
	}
	if report.Source != nil {
		color.Cyan("   source: %s", report.Source.Type)
		if report.Source.URL != "" {
			color.Cyan("   source url: %s", report.Source.URL)
		}
		if report.Source.Path != "" {
			color.Cyan("   source path: %s", report.Source.Path)
		}
	}
	fmt.Println()

	if report.Repos != nil && len(report.Repos.Repos) > 0 {
		color.Cyan("Repos")
		counts := map[string]int{}
		for _, repo := range report.Repos.Repos {
			counts[repo.Status]++
		}
		color.Cyan("   summary: %s", formatCountMap(counts))
		for _, repo := range report.Repos.Repos {
			color.Cyan("   - %s [%s]", repo.ID, repo.Status)
			if repo.Path != "" {
				color.Cyan("     path: %s", repo.Path)
			}
			if repo.Branch != "" {
				color.Cyan("     branch: %s", repo.Branch)
			}
			if repo.Worktree != "" {
				color.Cyan("     worktree: %s", repo.Worktree)
			}
		}
		fmt.Println()
	}

	color.Cyan("Artifacts")
	for _, artifact := range report.Artifacts {
		if artifact.Exists {
			color.Green("   ✓ %s", artifact.Name)
		} else {
			color.Red("   ✗ %s", artifact.Name)
		}
	}
	fmt.Println()

	if len(report.Missing) > 0 {
		color.Yellow("Missing")
		for _, name := range report.Missing {
			color.Yellow("   - %s", name)
		}
		fmt.Println()
	}

	color.Cyan("Next")
	color.Green("   %s", report.NextCommand)
	if len(report.RepoNext) > 0 {
		color.Cyan("Repo Next")
		for _, action := range report.RepoNext {
			color.Green("   %s", action)
		}
	}

	return nil
}
