package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdRunInput string
var prdRunRepos []string
var prdRunAllRepos bool

var prdRunCmd = &cobra.Command{
	Use:   "run",
	Short: "一键流水线：从需求文档到代码分支",
	Long:  "自动执行 refine → plan → code 全流程，零人工干预。task-id 自动生成。",
	RunE:  runPRDRun,
}

func init() {
	prdCmd.AddCommand(prdRunCmd)
	prdRunCmd.Flags().StringVarP(&prdRunInput, "input", "i", "", "PRD 输入：纯文本、本地文件路径或飞书链接")
	prdRunCmd.Flags().StringArrayVar(&prdRunRepos, "repo", nil, "附加关联仓库路径；可重复传入")
	prdRunCmd.Flags().BoolVar(&prdRunAllRepos, "all-repos", false, "多仓 task 下按绑定顺序依次执行所有 repo 的 code；失败即停")
	_ = prdRunCmd.MarkFlagRequired("input")
}

// runStep 记录单步执行的耗时和状态。
type runStep struct {
	Name     string
	Duration time.Duration
	Status   string // "ok", "skip", "fail"
	Detail   string
}

func runPRDRun(cmd *cobra.Command, args []string) error {
	startedAt := time.Now()
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	color.Cyan("🚀 PRD Run — 一键流水线")
	fmt.Println()

	// 中断处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		color.Yellow("⚠ 收到中断信号，流水线终止")
		os.Exit(130)
	}()
	defer signal.Stop(sigCh)

	var steps []runStep

	// ===== Step 1: Refine =====
	color.Cyan("━━━ [1/3] Refine ━━━")
	stepStart := time.Now()

	task, refineErr := prd.PrepareRefineTask(repoRoot, prd.RefineInput{
		RawInput:  prdRunInput,
		RepoPaths: prdRunRepos,
		Now:       time.Now(),
	})
	if refineErr != nil {
		return fmt.Errorf("refine 准备失败: %w", refineErr)
	}
	color.Cyan("   task_id: %s", task.TaskID)
	color.Cyan("   source: %s", task.Source.Type)

	if !task.SupportsRefine {
		pendingContent := prd.BuildPendingRefinedContent(task)
		if writeErr := prd.WriteRefinedContent(task, pendingContent, time.Now(), prd.TaskStatusInitialized); writeErr != nil {
			return writeErr
		}
		return fmt.Errorf("无法获取 PRD 正文内容，请手动补充 prd.source.md 后重试")
	}

	// 连接 daemon 生成 refined PRD
	gen, err := generator.New(repoRoot)
	if err != nil {
		color.Yellow("⚠ 连接 daemon 失败，使用本地兜底: %v", err)
		fallbackContent := prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, err)
		if writeErr := prd.WriteRefinedContent(task, fallbackContent, time.Now(), prd.TaskStatusRefined); writeErr != nil {
			return writeErr
		}
	} else {
		defer gen.Close()
		refinedContent, promptErr := gen.PromptWithTimeout(
			prd.BuildRefinedPrompt(task.Title, task.Source.Content),
			config.ReviewPromptTimeout,
			func(chunk string) { fmt.Print(chunk) },
		)
		if promptErr != nil {
			color.Yellow("⚠ AI refine 失败，使用本地兜底: %v", promptErr)
			refinedContent = prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, promptErr)
		} else {
			refinedContent = prd.ExtractRefinedContent(refinedContent)
			if validateErr := prd.ValidateRefinedContent(refinedContent); validateErr != nil {
				refinedContent = prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, validateErr)
			}
		}
		if writeErr := prd.WriteRefinedContent(task, refinedContent, time.Now(), prd.TaskStatusRefined); writeErr != nil {
			return writeErr
		}
	}

	steps = append(steps, runStep{Name: "refine", Duration: time.Since(stepStart), Status: "ok"})
	color.Green("   ✓ refine 完成 (%s)", formatDurationSeconds(time.Since(stepStart)))
	fmt.Println()

	// ===== Step 2: Plan =====
	color.Cyan("━━━ [2/3] Plan ━━━")
	stepStart = time.Now()

	if checkErr := prd.CheckPlanPrerequisites(repoRoot, task.TaskID); checkErr != nil {
		steps = append(steps, runStep{Name: "plan", Duration: time.Since(stepStart), Status: "fail", Detail: checkErr.Error()})
		color.Red("   ✗ plan 前置检查失败: %v", checkErr)
		return printRunSummary(steps, startedAt, false)
	}
	if missing, err := prd.MissingContextFiles(repoRoot); err != nil {
		steps = append(steps, runStep{Name: "plan", Duration: time.Since(stepStart), Status: "fail", Detail: err.Error()})
		color.Red("   ✗ context 检查失败: %v", err)
		return printRunSummary(steps, startedAt, false)
	} else if len(missing) > 0 {
		color.Yellow("⚠ context 不完整，缺少 %s；将降级继续生成 plan", strings.Join(missing, ", "))
	}

	explorer, explorerErr := generator.NewExplorer(repoRoot)
	if explorerErr != nil {
		color.Yellow("⚠ 启动 explorer 失败，回退本地模板: %v", explorerErr)
		_, planErr := prd.GeneratePlan(repoRoot, task.TaskID, time.Now())
		if planErr != nil {
			steps = append(steps, runStep{Name: "plan", Duration: time.Since(stepStart), Status: "fail", Detail: planErr.Error()})
			color.Red("   ✗ plan 失败: %v", planErr)
			return printRunSummary(steps, startedAt, false)
		}
		steps = append(steps, runStep{Name: "plan", Duration: time.Since(stepStart), Status: "ok", Detail: "本地模板"})
		color.Green("   ✓ plan 完成（本地模板）(%s)", formatDurationSeconds(time.Since(stepStart)))
	} else {
		defer explorer.Close()
		_, planErr := prd.GeneratePlanWithExplorer(explorer, repoRoot, task.TaskID, time.Now(), func(chunk string) { fmt.Print(chunk) }, func(event generator.ToolEvent) { renderToolEvent(event) })
		if planErr != nil {
			color.Yellow("⚠ agent plan 失败，回退本地模板: %v", planErr)
			_, fallbackErr := prd.GeneratePlan(repoRoot, task.TaskID, time.Now())
			if fallbackErr != nil {
				steps = append(steps, runStep{Name: "plan", Duration: time.Since(stepStart), Status: "fail", Detail: fallbackErr.Error()})
				color.Red("   ✗ plan 失败: %v", fallbackErr)
				return printRunSummary(steps, startedAt, false)
			}
		}
		steps = append(steps, runStep{Name: "plan", Duration: time.Since(stepStart), Status: "ok"})
		color.Green("   ✓ plan 完成 (%s)", formatDurationSeconds(time.Since(stepStart)))
	}
	fmt.Println()

	// ===== Step 3: Code =====
	color.Cyan("━━━ [3/3] Code ━━━")
	stepStart = time.Now()
	buildOK := true

	if _, err := prd.PrepareAgentCode(repoRoot, task.TaskID); err != nil {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: err.Error()})
		color.Red("   ✗ code 前置检查失败: %v", err)
		return printRunSummary(steps, startedAt, false)
	}

	taskReport, err := prd.LoadTaskStatus(repoRoot, task.TaskID)
	if err != nil {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: err.Error()})
		color.Red("   ✗ 加载 task 状态失败: %v", err)
		return printRunSummary(steps, startedAt, false)
	}
	complexityLevel, _, err := prd.ReadTaskComplexity(taskReport.TaskDir)
	if err != nil {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: err.Error()})
		color.Red("   ✗ 读取复杂度失败: %v", err)
		return printRunSummary(steps, startedAt, false)
	}
	if complexityLevel == "复杂" {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "skip", Detail: "复杂任务停止在 plan 阶段"})
		color.Yellow("   - 当前复杂度为「复杂」，run 将停止在 plan 阶段")
		return printRunSummary(steps, startedAt, true)
	}

	branchName := buildPRDBranchName(task.TaskID)
	if prdRunAllRepos {
		if taskReport.Repos == nil || len(taskReport.Repos.Repos) == 0 {
			steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: "task 未绑定 repo"})
			color.Red("   ✗ task 未绑定任何 repo")
			return printRunSummary(steps, startedAt, false)
		}
		for idx, repo := range taskReport.Repos.Repos {
			color.Cyan("   [%d/%d] repo: %s", idx+1, len(taskReport.Repos.Repos), repo.ID)
			report, codeErr := executePRDCodeForRepo(repo.Path, task.TaskID, branchName, repo.ID, prdCodeMaxRetry, func(chunk string) { fmt.Print(chunk) }, func(event generator.ToolEvent) { renderToolEvent(event) })
			fmt.Println()
			if codeErr != nil {
				steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: fmt.Sprintf("repo %s: %v", repo.ID, codeErr)})
				color.Red("   ✗ repo %s code 失败: %v", repo.ID, codeErr)
				return printRunSummary(steps, startedAt, false)
			}
			renderCodeRepoProgress(report)
			if !report.BuildOK {
				buildOK = false
				steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: fmt.Sprintf("repo %s 编译未通过", repo.ID)})
				color.Yellow("   ⚠ repo %s 编译未通过", repo.ID)
				return printRunSummary(steps, startedAt, false)
			}
		}
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "ok", Detail: fmt.Sprintf("all repos completed (%d)", len(taskReport.Repos.Repos))})
		color.Green("   ✓ all repos code 完成 (%s)", formatDurationSeconds(time.Since(stepStart)))
	} else {
		report, codeErr := executePRDCodeForRepo(repoRoot, task.TaskID, branchName, "", prdCodeMaxRetry, func(chunk string) { fmt.Print(chunk) }, func(event generator.ToolEvent) { renderToolEvent(event) })
		if codeErr != nil {
			steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: codeErr.Error()})
			color.Red("   ✗ code 生成失败: %v", codeErr)
			return printRunSummary(steps, startedAt, false)
		}

		stepDetail := fmt.Sprintf("%d 文件, commit %s", len(report.FilesWritten), report.Commit)
		if !report.BuildOK {
			buildOK = false
			steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: "编译未通过"})
			color.Yellow("   ⚠ 编译未通过")
			return printRunSummary(steps, startedAt, false)
		}
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "ok", Detail: stepDetail})
		color.Green("   ✓ code 完成 (%s)", formatDurationSeconds(time.Since(stepStart)))
		color.Green("   ✓ worktree: %s", report.Worktree)

		if taskReport.Repos != nil && len(taskReport.Repos.Repos) > 1 {
			if refreshed, loadErr := prd.LoadTaskStatus(repoRoot, task.TaskID); loadErr == nil && len(refreshed.RepoNext) > 0 {
				color.Cyan("   其他 repo 待执行:")
				for _, action := range refreshed.RepoNext {
					color.Cyan("      %s", action)
				}
			}
		}
	}

	fmt.Println()
	return printRunSummary(steps, startedAt, buildOK)
}

func printRunSummary(steps []runStep, startedAt time.Time, buildOK bool) error {
	color.Cyan("━━━ 汇总 ━━━")

	allOK := true
	hasSkip := false
	for _, s := range steps {
		icon := "✓"
		if s.Status == "fail" {
			icon = "✗"
			allOK = false
		} else if s.Status == "skip" {
			icon = "-"
			hasSkip = true
		}
		detail := ""
		if s.Detail != "" {
			detail = fmt.Sprintf(" (%s)", s.Detail)
		}
		color.Cyan("   %s %-8s %s%s", icon, s.Name, formatDurationSeconds(s.Duration), detail)
		if s.Status == "fail" && s.Detail != "" {
			color.Red("      %s", s.Detail)
		}
	}

	fmt.Println()
	if allOK && buildOK && !hasSkip {
		color.Green("✓ prd run 全部通过")
		color.Green("  分支: prd_<task_id>")
		color.Green("  next: 确认代码后执行 coco-ext push")
	} else if allOK && hasSkip {
		color.Yellow("⚠ prd run 已在 plan 阶段停止")
		color.Yellow("  next: 根据 plan 结果决定是否继续执行 coco-ext prd code")
	} else {
		color.Yellow("⚠ prd run 部分失败，请检查上方日志")
	}
	color.Green("⏱ 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))

	if !allOK || !buildOK {
		return fmt.Errorf("流水线未完全成功")
	}
	return nil
}
