package prd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
)

// BuildExplorerPlanPrompt 构建只读 agent 的 plan prompt。
// PRD 必读，context 文件只展示章节目录（渐进式披露），agent 按需读取。
func BuildExplorerPlanPrompt(repoRoot, taskDir string) string {
	var b strings.Builder

	b.WriteString("你是一名资深技术方案与研发计划助手。请基于需求文档，调研当前仓库，并输出 design.md 和 plan.md 的完整内容。\n\n")

	b.WriteString("## 需求文档（必读）\n\n")
	b.WriteString(filepath.Join(taskDir, "prd-refined.md") + "\n\n")

	// 渐进式披露：自动提取 context 文件的章节标题作为目录
	b.WriteString("## 参考文档（按需读取）\n\n")
	b.WriteString("以下是仓库知识库的目录索引，根据需求相关性选择读取：\n\n")
	contextDir := filepath.Join(repoRoot, ".livecoding", "context")
	contextFiles := []struct {
		path        string
		description string
	}{
		{"architecture.md", "仓库架构概览"},
		{"patterns.md", "代码模式与骨架"},
		{"gotchas.md", "踩坑记录与隐式约定"},
	}
	for _, cf := range contextFiles {
		fullPath := filepath.Join(contextDir, cf.path)
		toc := extractMarkdownHeadings(fullPath)
		b.WriteString(fmt.Sprintf("### %s — %s\n", cf.path, cf.description))
		if len(toc) > 0 {
			for _, h := range toc {
				b.WriteString(fmt.Sprintf("- %s\n", h))
			}
		} else {
			b.WriteString("- （文件不存在或为空）\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## 工作流程\n\n")
	b.WriteString("1. 读取需求文档，理解要做什么\n")
	b.WriteString("2. 根据需求相关性，选择读取上方参考文档中可能涉及的章节\n")
	b.WriteString("3. 在仓库中搜索并读取相关源文件，理解现有代码\n")
	b.WriteString("4. 理解代码后，输出技术方案和实施计划\n\n")

	b.WriteString("## 输出格式\n\n")
	b.WriteString("严格按以下标记输出两个文件的完整内容：\n\n")
	b.WriteString("=== DESIGN.md ===\n")
	b.WriteString("# Design\n\n")
	b.WriteString("（包含：背景与目标、改动范围、技术方案、接口变更、存储变更、")
	b.WriteString("监控告警、性能评估、QA 验证项、上线与回滚计划等完整技术设计）\n\n")
	b.WriteString("=== PLAN.md ===\n")
	b.WriteString("# Plan\n\n")
	b.WriteString("（包含：复杂度评估、实现概要、实现目标、拟改文件列表（带改动说明）、")
	b.WriteString("任务列表、实施步骤、风险补充、待确认项、验证建议等完整实施计划）\n\n")

	b.WriteString("## 规则\n\n")
	b.WriteString("- 这是只读调研任务，不要修改任何文件\n")
	b.WriteString("- 候选改动文件必须基于你实际读到的代码判断，不要猜测\n")
	b.WriteString("- design.md 要包含实质性的技术分析，不要写模板废话\n")
	b.WriteString("- plan.md 中的拟改文件要说明每个文件需要做什么改动\n")
	b.WriteString("- 保持输出为纯 Markdown，不要用代码块包裹整个文件\n")

	return b.String()
}

// ExplorerPlanResult 是 agent 调研模式的结果。
type ExplorerPlanResult struct {
	DesignContent string
	PlanContent   string
	ToolEvents    []generator.ToolEvent
	UsedAI        bool
}

// extractMarkdownHeadings 从 markdown 文件中提取 ## 级别的标题。
// 用于渐进式披露：让 agent 看到目录再决定是否读全文。
func extractMarkdownHeadings(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var headings []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			h := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if h != "" {
				headings = append(headings, h)
			}
		}
	}
	return headings
}

// GeneratePlanWithExplorer 使用只读 agent 调研并生成 design.md + plan.md。
func GeneratePlanWithExplorer(explorer *generator.AgentGenerator, repoRoot, taskID string, now time.Time, onChunk func(string), onTool func(generator.ToolEvent)) (*PlanArtifacts, error) {
	taskDir := filepath.Join(repoRoot, ".livecoding", "tasks", taskID)

	prompt := BuildExplorerPlanPrompt(repoRoot, taskDir)

	var toolEvents []generator.ToolEvent
	wrappedOnTool := func(event generator.ToolEvent) {
		toolEvents = append(toolEvents, event)
		if onTool != nil {
			onTool(event)
		}
	}

	reply, err := explorer.PromptWithTools(prompt, config.CodePromptTimeout, onChunk, wrappedOnTool)
	if err != nil && reply == "" {
		return nil, fmt.Errorf("explorer 调研失败: %w", err)
	}

	// 解析 agent 输出
	design, plan := parseExplorerOutput(reply)
	if design == "" || plan == "" {
		return nil, fmt.Errorf("explorer 输出中未找到 DESIGN.md 或 PLAN.md 标记")
	}

	// 写入文件
	task, loadErr := LoadTaskStatus(repoRoot, taskID)
	if loadErr != nil {
		return nil, loadErr
	}

	designPath := filepath.Join(task.TaskDir, "design.md")
	planPath := filepath.Join(task.TaskDir, "plan.md")
	if writeErr := os.WriteFile(designPath, []byte(design), 0644); writeErr != nil {
		return nil, fmt.Errorf("写入 design.md 失败: %w", writeErr)
	}
	if writeErr := os.WriteFile(planPath, []byte(plan), 0644); writeErr != nil {
		return nil, fmt.Errorf("写入 plan.md 失败: %w", writeErr)
	}
	if statusErr := updateTaskStatus(task.TaskDir, TaskStatusPlanned, now); statusErr != nil {
		return nil, statusErr
	}

	return &PlanArtifacts{
		DesignPath: designPath,
		PlanPath:   planPath,
		UsedAI:     true,
	}, nil
}

// CheckPlanPrerequisites 校验 plan 前置条件：task 状态 + context + 文件存在。
func CheckPlanPrerequisites(repoRoot, taskID string) error {
	task, err := LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return err
	}
	if task.Metadata.Status != TaskStatusRefined && task.Metadata.Status != TaskStatusPlanned {
		return fmt.Errorf("task 状态为 %s，需要先执行 coco-ext prd refine", task.Metadata.Status)
	}
	if _, err := LoadContextSnapshot(repoRoot); err != nil {
		return err
	}
	for _, name := range []string{"prd-refined.md"} {
		path := filepath.Join(repoRoot, ".livecoding", "tasks", taskID, name)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%s 不存在: %w", name, err)
		}
	}
	return nil
}

// parseExplorerOutput 从 agent 输出中提取 design.md 和 plan.md 内容。
func parseExplorerOutput(raw string) (design, plan string) {
	const designMarker = "=== DESIGN.md ==="
	const planMarker = "=== PLAN.md ==="

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")

	designIdx := strings.Index(normalized, designMarker)
	planIdx := strings.Index(normalized, planMarker)

	if designIdx == -1 || planIdx == -1 {
		return "", ""
	}

	// design 内容：从 designMarker 结尾到 planMarker 开头
	designStart := designIdx + len(designMarker)
	if planIdx > designStart {
		design = strings.TrimSpace(normalized[designStart:planIdx])
	}

	// plan 内容：从 planMarker 结尾到文件尾
	planStart := planIdx + len(planMarker)
	if planStart < len(normalized) {
		plan = strings.TrimSpace(normalized[planStart:])
	}

	return design, plan
}
