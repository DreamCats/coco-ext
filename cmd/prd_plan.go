package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/generator"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdPlanTaskID string

var prdPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "基于 refined PRD 调研代码库，生成 design.md 与 plan.md",
	Long:  "启动只读 agent 调研仓库代码，基于 PRD 和 context 生成 design.md 与 plan.md。失败时回退到本地模板方案。",
	RunE:  runPRDPlan,
}

func init() {
	prdCmd.AddCommand(prdPlanCmd)
	prdPlanCmd.Flags().StringVar(&prdPlanTaskID, "task", "", "指定 task id；默认读取最近一个 task")
}

func runPRDPlan(cmd *cobra.Command, args []string) error {
	startedAt := time.Now()
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	taskID, err := prd.ResolveTaskID(repoRoot, prdPlanTaskID)
	if err != nil {
		return err
	}

	color.Cyan("🧭 PRD Plan")
	color.Cyan("   task_id: %s", taskID)

	// 校验 task 状态与必要文件
	if err := prd.CheckPlanPrerequisites(repoRoot, taskID); err != nil {
		return err
	}
	if missing, err := prd.MissingContextFiles(repoRoot); err != nil {
		return err
	} else if len(missing) > 0 {
		color.Yellow("⚠ context 不完整，缺少 %s；将降级继续生成 plan", strings.Join(missing, ", "))
	}

	color.Cyan("   [1/3] 正在启动只读 agent（yolo，禁止写入）...")
	connectStartedAt := time.Now()
	explorer, err := generator.NewExplorer(repoRoot)
	if err != nil {
		color.Yellow("⚠ 启动 agent 失败，回退本地模板: %v", err)
		return runLocalPlan(repoRoot, taskID, startedAt)
	}
	defer explorer.Close()
	color.Green("   [1/3] agent 已就绪 ✓")
	color.Cyan("      启动耗时: %s", formatDurationSeconds(time.Since(connectStartedAt)))

	color.Cyan("   [2/3] agent 正在调研代码库...")
	generateStartedAt := time.Now()

	artifacts, err := prd.GeneratePlanWithExplorer(explorer, repoRoot, taskID, time.Now(),
		func(chunk string) {
			fmt.Print(chunk)
		},
		func(event generator.ToolEvent) {
			renderToolEvent(event)
		},
	)
	fmt.Println()

	if err != nil {
		color.Yellow("⚠ agent 调研失败，回退本地模板: %v", err)
		return runLocalPlan(repoRoot, taskID, startedAt)
	}
	color.Cyan("      调研耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))

	color.Green("   [3/3] 产物已写入 task 目录 ✓")
	color.Green("✓ plan 完成（agent 调研）")
	color.Green("  design.md: %s", artifacts.DesignPath)
	color.Green("  plan.md: %s", artifacts.PlanPath)
	color.Green("⏱ 本次 plan 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))

	return nil
}

// runLocalPlan 本地模板 fallback（不依赖 agent）。
func runLocalPlan(repoRoot, taskID string, startedAt time.Time) error {
	color.Cyan("   [fallback] 使用本地模板生成...")

	artifacts, err := prd.GeneratePlan(repoRoot, taskID, time.Now())
	if err != nil {
		return err
	}

	color.Green("✓ plan 完成（本地模板）")
	color.Green("  design.md: %s", artifacts.DesignPath)
	color.Green("  plan.md: %s", artifacts.PlanPath)
	color.Green("⏱ 本次 plan 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))
	return nil
}
