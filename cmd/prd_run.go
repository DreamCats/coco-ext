package cmd

import (
	"fmt"
	"os"
	"os/signal"
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

var prdRunCmd = &cobra.Command{
	Use:   "run",
	Short: "一键流水线：从需求文档到代码分支",
	Long:  "自动执行 refine → plan → code 全流程，零人工干预。task-id 自动生成。",
	RunE:  runPRDRun,
}

func init() {
	prdCmd.AddCommand(prdRunCmd)
	prdRunCmd.Flags().StringVarP(&prdRunInput, "input", "i", "", "PRD 输入：纯文本、本地文件路径或飞书链接")
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
		RawInput: prdRunInput,
		Now:      time.Now(),
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
			nil,
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

	taskDir := task.TaskDir

	// ===== Step 2: Plan =====
	color.Cyan("━━━ [2/3] Plan ━━━")
	stepStart = time.Now()

	if checkErr := prd.CheckPlanPrerequisites(repoRoot, task.TaskID); checkErr != nil {
		steps = append(steps, runStep{Name: "plan", Duration: time.Since(stepStart), Status: "fail", Detail: checkErr.Error()})
		color.Red("   ✗ plan 前置检查失败: %v", checkErr)
		return printRunSummary(steps, startedAt, false)
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
		_, planErr := prd.GeneratePlanWithExplorer(explorer, repoRoot, task.TaskID, time.Now(), nil, nil)
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

	prepareTaskDir, prepareErr := prd.PrepareAgentCode(repoRoot, task.TaskID)
	if prepareErr != nil {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: prepareErr.Error()})
		color.Red("   ✗ code 前置检查失败: %v", prepareErr)
		return printRunSummary(steps, startedAt, false)
	}

	branchName := "prd/" + task.TaskID
	origBranch := codeCurrentBranch(repoRoot)

	// stash 未提交改动
	stashed := false
	if codeHasUncommittedChanges(repoRoot) {
		if stashErr := codeStash(repoRoot); stashErr != nil {
			return fmt.Errorf("stash 失败: %w", stashErr)
		}
		stashed = true
	}

	// 切到工作分支
	if checkoutErr := codeCheckoutBranch(repoRoot, branchName); checkoutErr != nil {
		if stashed {
			codeStashPop(repoRoot)
		}
		return checkoutErr
	}

	// 确保任何失败都恢复状态
	codeOK := false
	defer func() {
		if codeOK {
			return
		}
		codeCheckoutBranchQuiet(repoRoot, origBranch)
		if stashed {
			codeStashPop(repoRoot)
		}
	}()

	agent, agentErr := generator.NewAgent(repoRoot)
	if agentErr != nil {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: agentErr.Error()})
		color.Red("   ✗ 启动 agent 失败: %v", agentErr)
		return printRunSummary(steps, startedAt, false)
	}
	defer agent.Close()

	result, codeErr := prd.GenerateCodeWithAgent(agent, prepareTaskDir, time.Now(), nil, nil)
	if codeErr != nil {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: codeErr.Error()})
		color.Red("   ✗ code 生成失败: %v", codeErr)
		return printRunSummary(steps, startedAt, false)
	}

	// 编译失败自动重试
	if !result.BuildOK && prdCodeMaxRetry > 0 {
		changedPkgs := codeExtractChangedPackages(repoRoot)
		if len(changedPkgs) > 0 {
			for attempt := 1; attempt <= prdCodeMaxRetry; attempt++ {
				buildOutput, buildErr := codeRunBuildPackages(repoRoot, changedPkgs)
				if buildErr == nil {
					result.BuildOK = true
					break
				}
				followUp := fmt.Sprintf("编译失败，请修复以下错误：\n%s\n修复后重新运行 go build 验证，然后输出 === CODE RESULT ===", buildOutput)
				reply, retryErr := agent.PromptWithTools(followUp, config.CodePromptTimeout, nil, nil)
				if retryErr != nil && reply == "" {
					break
				}
				cr := prd.ParseCodeResult(reply)
				if cr != nil {
					result.BuildOK = cr.BuildOK
					if len(cr.Files) > 0 {
						result.FilesChanged = cr.Files
					}
					if cr.Summary != "" {
						result.Summary = cr.Summary
					}
				} else {
					_, buildErr2 := codeRunBuildPackages(repoRoot, changedPkgs)
					if buildErr2 == nil {
						result.BuildOK = true
					}
				}
			}
		}
	}

	// 收集改动并 commit
	changedFiles := result.FilesChanged
	if len(changedFiles) == 0 {
		changedFiles = codeCollectChanges(repoRoot)
	} else {
		gitChanged := codeCollectChanges(repoRoot)
		if len(gitChanged) > 0 {
			changedFiles = codeMergeFileLists(changedFiles, gitChanged)
		}
	}

	commitHash := ""
	if result.BuildOK && len(changedFiles) > 0 {
		hash, commitErr := codeCommitOnBranch(repoRoot, task.TaskID, changedFiles, result.Summary)
		if commitErr != nil {
			color.Yellow("   ⚠ auto-commit 失败: %v", commitErr)
		} else {
			commitHash = hash
		}
	}

	// 恢复状态
	codeOK = true
	codeCheckoutBranchQuiet(repoRoot, origBranch)
	if stashed {
		if popErr := codeStashPop(repoRoot); popErr != nil {
			color.Yellow("   ⚠ stash pop 失败: %v", popErr)
		}
	}

	stepDetail := fmt.Sprintf("%d 文件, commit %s", len(changedFiles), commitHash)
	if !result.BuildOK {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "fail", Detail: "编译未通过"})
		color.Yellow("   ⚠ 编译未通过")
	} else {
		steps = append(steps, runStep{Name: "code", Duration: time.Since(stepStart), Status: "ok", Detail: stepDetail})
		color.Green("   ✓ code 完成 (%s)", formatDurationSeconds(time.Since(stepStart)))
	}

	// 写入 result file
	report := prd.CodeResultReport{
		Status:       "success",
		TaskID:       task.TaskID,
		Branch:       branchName,
		Commit:       commitHash,
		BuildOK:      result.BuildOK,
		FilesWritten: changedFiles,
		Log:          fmt.Sprintf("%s/code.log", taskDir),
		StartedAt:    startedAt.Format(time.RFC3339),
		FinishedAt:   time.Now().Format(time.RFC3339),
	}
	if !result.BuildOK {
		report.Status = "build_unknown"
	}
	_ = prd.WriteCodeResultReport(taskDir, report)

	fmt.Println()
	return printRunSummary(steps, startedAt, result.BuildOK)
}

func printRunSummary(steps []runStep, startedAt time.Time, buildOK bool) error {
	color.Cyan("━━━ 汇总 ━━━")

	allOK := true
	for _, s := range steps {
		icon := "✓"
		if s.Status == "fail" {
			icon = "✗"
			allOK = false
		} else if s.Status == "skip" {
			icon = "-"
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
	if allOK && buildOK {
		color.Green("✓ prd run 全部通过")
		color.Green("  分支: prd/<task_id>")
		color.Green("  next: 确认代码后执行 coco-ext push")
	} else {
		color.Yellow("⚠ prd run 部分失败，请检查上方日志")
	}
	color.Green("⏱ 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))

	if !allOK || !buildOK {
		return fmt.Errorf("流水线未完全成功")
	}
	return nil
}
