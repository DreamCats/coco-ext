package prd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
)

type ContextSnapshot struct {
	GlossaryPath     string
	ArchitecturePath string
	PatternsPath     string
	GotchasPath      string
	GlossaryContent  string
	Architecture     string
	Patterns         string
	Gotchas          string
	GlossaryEntries  []GlossaryEntry
}

type GlossaryEntry struct {
	Business   string
	Identifier string
	Module     string
}

type RefinedSections struct {
	Summary       string
	Features      []string
	Boundaries    []string
	BusinessRules []string
	OpenQuestions []string
	Raw           string
}

type ResearchFinding struct {
	MatchedTerms   []GlossaryEntry
	UnmatchedTerms []string
	CandidateFiles []string
	CandidateDirs  []string
	Notes          []string
}

type ComplexityDimension struct {
	Name   string
	Score  int
	Reason string
}

type ComplexityAssessment struct {
	Dimensions []ComplexityDimension
	Total      int
	Level      string
	Conclusion string
}

type PlanArtifacts struct {
	DesignPath string
	PlanPath   string
	UsedAI     bool
}

type PlanTask struct {
	ID        string
	Title     string
	Goal      string
	DependsOn []string
	Files     []string
	Input     []string
	Output    []string
	Actions   []string
	Done      []string
}

type PlanAISections struct {
	Summary         string
	Steps           string
	Risks           string
	ValidationExtra string
}

type PlanBuild struct {
	Task       *TaskStatusReport
	Context    *ContextSnapshot
	Sections   RefinedSections
	Findings   ResearchFinding
	Assessment ComplexityAssessment
}

var rePlanASCIIWord = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]{1,}`)

func PreparePlanBuild(repoRoot, taskID string) (*PlanBuild, error) {
	task, err := LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	refinedPath := filepath.Join(task.TaskDir, "prd-refined.md")
	refinedContent, err := os.ReadFile(refinedPath)
	if err != nil {
		return nil, fmt.Errorf("读取 prd-refined.md 失败: %w", err)
	}

	context, err := LoadContextSnapshot(repoRoot)
	if err != nil {
		return nil, err
	}

	sections := ParseRefinedSections(string(refinedContent))
	findings := ResearchCodebase(repoRoot, task.Metadata.Title, sections, context)
	assessment := ScoreComplexity(sections, findings)

	return &PlanBuild{
		Task:       task,
		Context:    context,
		Sections:   sections,
		Findings:   findings,
		Assessment: assessment,
	}, nil
}

func GeneratePlan(repoRoot, taskID string, now time.Time) (*PlanArtifacts, error) {
	build, err := PreparePlanBuild(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	designContent := BuildDesignContent(build.Task, build.Context, build.Sections, build.Findings, build.Assessment)
	planContent := BuildPlanContent(build.Task, build.Sections, build.Findings, build.Assessment)
	return writePlanArtifacts(build.Task, designContent, planContent, now, false)
}

func GeneratePlanWithAI(gen *generator.Generator, repoRoot, taskID string, now time.Time, onChunk func(string)) (*PlanArtifacts, error) {
	build, err := PreparePlanBuild(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	localDesign := BuildDesignContent(build.Task, build.Context, build.Sections, build.Findings, build.Assessment)
	localPlan := BuildPlanContent(build.Task, build.Sections, build.Findings, build.Assessment)
	planHeader := BuildPlanHeader(build.Task)

	if gen == nil {
		return writePlanArtifacts(build.Task, localDesign, localPlan, now, false)
	}

	prompt := BuildPlanPrompt(build)
	raw, err := gen.PromptWithTimeout(prompt, config.ReviewPromptTimeout, onChunk)
	if err != nil {
		return writePlanArtifacts(build.Task, localDesign, localPlan, now, false)
	}

	aiSections, ok := ExtractPlanOutputs(raw)
	if !ok {
		return writePlanArtifacts(build.Task, localDesign, localPlan, now, false)
	}
	if err := ValidatePlanOutputs(build, aiSections); err != nil {
		return writePlanArtifacts(build.Task, localDesign, localPlan, now, false)
	}

	return writePlanArtifacts(build.Task, localDesign, planHeader+BuildPlanBody(build.Sections, build.Findings, build.Assessment, &aiSections), now, true)
}

func LoadContextSnapshot(repoRoot string) (*ContextSnapshot, error) {
	contextDir := filepath.Join(repoRoot, ".livecoding", "context")
	required := map[string]*string{
		"glossary.md":     nil,
		"architecture.md": nil,
		"patterns.md":     nil,
	}

	for name := range required {
		path := filepath.Join(contextDir, name)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("context 不完整，缺少 %s；请先执行 `coco-ext context init` 或 `coco-ext context update`", name)
			}
			return nil, fmt.Errorf("读取 context 文件失败: %w", err)
		}
	}

	glossaryPath := filepath.Join(contextDir, "glossary.md")
	architecturePath := filepath.Join(contextDir, "architecture.md")
	patternsPath := filepath.Join(contextDir, "patterns.md")
	gotchasPath := filepath.Join(contextDir, "gotchas.md")

	glossaryContent, err := os.ReadFile(glossaryPath)
	if err != nil {
		return nil, fmt.Errorf("读取 glossary.md 失败: %w", err)
	}
	architectureContent, err := os.ReadFile(architecturePath)
	if err != nil {
		return nil, fmt.Errorf("读取 architecture.md 失败: %w", err)
	}
	patternsContent, err := os.ReadFile(patternsPath)
	if err != nil {
		return nil, fmt.Errorf("读取 patterns.md 失败: %w", err)
	}

	gotchasContent := ""
	if data, err := os.ReadFile(gotchasPath); err == nil {
		gotchasContent = string(data)
	}

	return &ContextSnapshot{
		GlossaryPath:     glossaryPath,
		ArchitecturePath: architecturePath,
		PatternsPath:     patternsPath,
		GotchasPath:      gotchasPath,
		GlossaryContent:  string(glossaryContent),
		Architecture:     string(architectureContent),
		Patterns:         string(patternsContent),
		Gotchas:          gotchasContent,
		GlossaryEntries:  parseGlossaryEntries(string(glossaryContent)),
	}, nil
}

func ParseRefinedSections(content string) RefinedSections {
	sections := splitMarkdownSections(content)
	return RefinedSections{
		Summary:       cleanSectionLines(sections["需求概述"]),
		Features:      extractBulletItems(sections["功能点"]),
		Boundaries:    extractBulletItems(sections["边界条件"]),
		BusinessRules: extractBulletItems(sections["业务规则"]),
		OpenQuestions: extractBulletItems(sections["待确认问题"]),
		Raw:           strings.TrimSpace(content),
	}
}

func ResearchCodebase(repoRoot, title string, sections RefinedSections, context *ContextSnapshot) ResearchFinding {
	searchText := strings.Join([]string{
		title,
		sections.Summary,
		strings.Join(sections.Features, "\n"),
		strings.Join(sections.BusinessRules, "\n"),
	}, "\n")

	matched := make([]GlossaryEntry, 0)
	for _, entry := range context.GlossaryEntries {
		if containsAny(searchText, entry.Business, entry.Identifier) {
			matched = append(matched, entry)
		}
	}

	unmatched := inferUnmatchedTerms(searchText, matched)
	searchTerms := inferSearchTerms(title, sections, matched)
	candidateFiles := make([]string, 0, 8)
	if shouldFocusPRDWorkflow(searchTerms) {
		candidateFiles = preferPRDWorkflowFiles(searchTerms)
	} else {
		candidateFiles = findCandidateFiles(repoRoot, matched, searchTerms)
	}
	if len(candidateFiles) == 0 {
		candidateFiles = heuristicCandidateFiles(repoRoot, searchTerms)
	}
	candidateDirs := summarizeDirs(candidateFiles)

	notes := make([]string, 0, 4)
	if len(matched) == 0 {
		notes = append(notes, "未在 glossary 中命中明显术语，调研可信度较低。")
	}
	if len(candidateFiles) == 0 {
		notes = append(notes, "未通过现有术语映射找到候选代码文件。")
	}
	if len(sections.OpenQuestions) > 0 {
		notes = append(notes, fmt.Sprintf("存在 %d 个待确认问题，说明需求仍有不确定性。", len(sections.OpenQuestions)))
	}

	return ResearchFinding{
		MatchedTerms:   matched,
		UnmatchedTerms: unmatched,
		CandidateFiles: candidateFiles,
		CandidateDirs:  candidateDirs,
		Notes:          notes,
	}
}

func shouldFocusPRDWorkflow(searchTerms []string) bool {
	for _, term := range searchTerms {
		switch strings.ToLower(term) {
		case "prd", "plan", "refine", "status", "source", "task", "lark", "feishu", "doc", "docx", "url":
			return true
		}
	}
	return false
}

func ScoreComplexity(sections RefinedSections, findings ResearchFinding) ComplexityAssessment {
	dimensions := make([]ComplexityDimension, 0, 6)

	fileCount := len(findings.CandidateFiles)
	scopeScore := 0
	scopeReason := "候选改动文件较少，范围集中。"
	switch {
	case fileCount > 5:
		scopeScore = 2
		scopeReason = "候选改动文件超过 5 个，范围偏大。"
	case fileCount > 2:
		scopeScore = 1
		scopeReason = "候选改动文件在 3-5 个之间，范围中等。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "改动范围", Score: scopeScore, Reason: scopeReason})

	interfaceScore := 0
	interfaceReason := "未发现明显的接口或协议变更信号。"
	if containsAny(strings.Join(sections.Features, "\n"), "接口", "协议", "请求", "返回", "字段") {
		interfaceScore = 1
		interfaceReason = "需求描述中包含接口/字段类变更信号。"
	}
	if hasPathKeyword(findings.CandidateFiles, "handler", ".proto", ".thrift") {
		interfaceScore = 2
		interfaceReason = "候选文件涉及 handler/IDL，可能影响对外接口。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "接口协议", Score: interfaceScore, Reason: interfaceReason})

	dataScore := 0
	dataReason := "未发现复杂数据或持久化变更。"
	if containsAny(strings.Join(sections.Boundaries, "\n"), "状态", "缓存", "数据库", "表", "持久化") {
		dataScore = 1
		dataReason = "边界条件中出现状态/数据类描述。"
	}
	if containsAny(strings.Join(sections.BusinessRules, "\n"), "状态流转", "一致性", "数据同步") {
		dataScore = 2
		dataReason = "业务规则暗示存在复杂状态流转或一致性要求。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "数据状态", Score: dataScore, Reason: dataReason})

	questionCount := len(sections.OpenQuestions)
	ruleScore := 0
	ruleReason := "业务规则相对清晰。"
	switch {
	case questionCount > 4:
		ruleScore = 2
		ruleReason = "待确认问题较多，业务规则仍不清晰。"
	case questionCount > 1:
		ruleScore = 1
		ruleReason = "存在少量待确认问题，需要人工确认。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "规则清晰度", Score: ruleScore, Reason: ruleReason})

	dependencyScore := 0
	dependencyReason := "候选目录较集中，依赖面可控。"
	switch {
	case len(findings.CandidateDirs) > 2:
		dependencyScore = 2
		dependencyReason = "候选目录跨多个模块，可能需要跨模块协作。"
	case len(findings.CandidateDirs) > 1:
		dependencyScore = 1
		dependencyReason = "候选目录跨两个模块，存在一定依赖关系。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "依赖联动", Score: dependencyScore, Reason: dependencyReason})

	verifyScore := 0
	verifyReason := "需求较易验证。"
	if len(findings.UnmatchedTerms) > 0 {
		verifyScore = 1
		verifyReason = "存在 glossary 未命中的术语，调研结果需要额外验证。"
	}
	if len(findings.CandidateFiles) == 0 {
		verifyScore = 2
		verifyReason = "未找到候选文件，当前无法形成可靠实现方案。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "验证风险", Score: verifyScore, Reason: verifyReason})

	total := 0
	for _, dimension := range dimensions {
		total += dimension.Score
	}

	level := "简单"
	conclusion := "复杂度较低，可以进入详细编码计划阶段。"
	switch {
	case total > 6:
		level = "复杂"
		conclusion = "复杂度超过阈值，建议先人工拆解或补充上下文，不直接进入自动实现。"
	case total > 3:
		level = "中等"
		conclusion = "复杂度中等，可以生成计划，但需重点关注风险与待确认项。"
	}

	return ComplexityAssessment{
		Dimensions: dimensions,
		Total:      total,
		Level:      level,
		Conclusion: conclusion,
	}
}

func BuildPlanContent(task *TaskStatusReport, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) string {
	var b strings.Builder
	b.WriteString(BuildPlanHeader(task))
	b.WriteString(BuildPlanBody(sections, findings, assessment, nil))
	return b.String()
}

func BuildPlanHeader(task *TaskStatusReport) string {
	var b strings.Builder
	b.WriteString("# Plan\n\n")
	b.WriteString(fmt.Sprintf("- task_id: %s\n", task.TaskID))
	b.WriteString(fmt.Sprintf("- title: %s\n", task.Metadata.Title))
	b.WriteString("\n")
	return b.String()
}

func BuildPlanBody(sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment, ai *PlanAISections) string {
	var b strings.Builder
	tasks := BuildPlanTasks(sections, findings)
	b.WriteString("## 复杂度评估\n\n")
	b.WriteString(fmt.Sprintf("- complexity: %s (%d)\n", assessment.Level, assessment.Total))
	b.WriteString(fmt.Sprintf("- 结论: %s\n\n", assessment.Conclusion))
	b.WriteString("## 实现概要\n\n")
	if ai != nil && strings.TrimSpace(ai.Summary) != "" {
		b.WriteString(strings.TrimSpace(ai.Summary) + "\n\n")
	} else {
		b.WriteString("- 基于 refined PRD、context 和本地调研结果收敛改动范围。\n")
		b.WriteString("- 优先复用现有 prd workflow 命令与 task 产物，不引入额外状态机。\n\n")
	}

	if assessment.Total > 6 {
		b.WriteString("## 结论\n\n")
		b.WriteString("- 当前需求被判定为复杂，暂不建议直接进入自动 codegen。\n")
		b.WriteString("- 建议先人工拆分需求、补充上下文或补全 PRD 后再重新执行 `coco-ext prd plan`。\n\n")
	} else {
		b.WriteString("## 实现目标\n\n")
		for _, feature := range sections.Features {
			b.WriteString(fmt.Sprintf("- %s\n", feature))
		}
		if len(sections.Features) == 0 {
			b.WriteString("- 基于 refined PRD 补全实现目标。\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## 拟改文件\n\n")
	if len(findings.CandidateFiles) == 0 {
		b.WriteString("- 暂未命中候选文件，需要补充 context 或人工指定模块。\n")
	} else {
		for _, file := range findings.CandidateFiles {
			b.WriteString(fmt.Sprintf("- %s：%s\n", file, describePlannedFileChange(file)))
		}
	}
	b.WriteString("\n")

	b.WriteString("## 任务列表\n\n")
	if len(tasks) == 0 {
		b.WriteString("- 暂未生成任务列表，需要先收敛候选文件后再继续。\n\n")
	} else {
		for _, task := range tasks {
			b.WriteString(fmt.Sprintf("### %s %s\n\n", task.ID, task.Title))
			b.WriteString(fmt.Sprintf("- 目标：%s\n", task.Goal))
			if len(task.DependsOn) > 0 {
				b.WriteString(fmt.Sprintf("- 依赖任务：%s\n", strings.Join(task.DependsOn, ", ")))
			}
			if len(task.Files) > 0 {
				b.WriteString("- 涉及文件：\n")
				for _, file := range task.Files {
					b.WriteString(fmt.Sprintf("  - %s\n", file))
				}
			}
			if len(task.Input) > 0 {
				b.WriteString("- 输入：\n")
				for _, item := range task.Input {
					b.WriteString(fmt.Sprintf("  - %s\n", item))
				}
			}
			if len(task.Output) > 0 {
				b.WriteString("- 输出：\n")
				for _, item := range task.Output {
					b.WriteString(fmt.Sprintf("  - %s\n", item))
				}
			}
			if len(task.Actions) > 0 {
				b.WriteString("- 具体动作：\n")
				for _, action := range task.Actions {
					b.WriteString(fmt.Sprintf("  - %s\n", action))
				}
			}
			if len(task.Done) > 0 {
				b.WriteString("- 完成标志：\n")
				for _, item := range task.Done {
					b.WriteString(fmt.Sprintf("  - %s\n", item))
				}
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## 实施步骤\n\n")
	if ai != nil && strings.TrimSpace(ai.Steps) != "" {
		b.WriteString(strings.TrimSpace(ai.Steps) + "\n\n")
	} else {
		if len(tasks) == 0 {
			b.WriteString("- 先补充 context 或人工确认目标模块，再继续细化实施步骤。\n\n")
		} else {
			for _, task := range tasks {
				b.WriteString(fmt.Sprintf("- %s：先完成「%s」，再根据完成标志逐项自检。\n", task.ID, task.Title))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## 风险补充\n\n")
	if ai != nil && strings.TrimSpace(ai.Risks) != "" {
		b.WriteString(strings.TrimSpace(ai.Risks) + "\n\n")
	} else if len(findings.Notes) > 0 {
		for _, note := range findings.Notes {
			b.WriteString(fmt.Sprintf("- %s\n", note))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- 当前未发现额外风险补充。\n\n")
	}

	b.WriteString("## 待确认项\n\n")
	if len(sections.OpenQuestions) == 0 {
		b.WriteString("- 无额外待确认项。\n")
	} else {
		for _, question := range sections.OpenQuestions {
			b.WriteString(fmt.Sprintf("- %s\n", question))
		}
	}
	b.WriteString("\n")

	b.WriteString("## 验证建议\n\n")
	b.WriteString("- 仅编译涉及的 package，不执行全仓 build/test。\n")
	b.WriteString("- 完成实现后建议运行 `coco-ext review` 或 `/livecoding:auto-review`。\n")
	if ai != nil && strings.TrimSpace(ai.ValidationExtra) != "" {
		b.WriteString(strings.TrimSpace(ai.ValidationExtra) + "\n")
	}
	return b.String()
}

func BuildPlanPrompt(build *PlanBuild) string {
	var b strings.Builder
	b.WriteString("你是一名资深技术方案与研发计划助手。系统会自行写入 plan.md 的固定头部、复杂度骨架、拟改文件、任务列表和待确认项。你只需要基于提供的 PRD refined 内容、本地 context 事实和代码调研结果，补充 plan.md 的可变 section。\n\n")
	b.WriteString("要求：\n")
	b.WriteString("1. 只能基于提供的信息工作，不要编造未出现的模块、文件或接口。\n")
	b.WriteString("2. 不要输出 task_id、title、复杂度总分、拟改文件清单、待确认项这些固定字段。\n")
	b.WriteString("3. 如果需求复杂，仍然要在总结或风险里明确写出“不建议自动实现”。\n")
	b.WriteString("4. 输出必须严格使用下面的标记格式：\n")
	b.WriteString("=== IMPLEMENTATION SUMMARY ===\n")
	b.WriteString("- ...\n")
	b.WriteString("=== IMPLEMENTATION STEPS ===\n")
	b.WriteString("- ...\n")
	b.WriteString("=== RISK NOTES ===\n")
	b.WriteString("- ...\n")
	b.WriteString("=== VALIDATION EXTRA ===\n")
	b.WriteString("- ...\n")
	b.WriteString("5. 不要输出其它前言或解释。\n\n")

	b.WriteString("## PRD Refined\n")
	b.WriteString(build.Sections.Raw)
	b.WriteString("\n\n## Context 摘要\n")
	b.WriteString("- glossary 命中术语：\n")
	if len(build.Findings.MatchedTerms) == 0 {
		b.WriteString("  - 无\n")
	} else {
		for _, entry := range build.Findings.MatchedTerms {
			b.WriteString(fmt.Sprintf("  - %s -> %s (%s)\n", entry.Business, entry.Identifier, entry.Module))
		}
	}
	b.WriteString("- 未命中术语：\n")
	if len(build.Findings.UnmatchedTerms) == 0 {
		b.WriteString("  - 无\n")
	} else {
		for _, term := range build.Findings.UnmatchedTerms {
			b.WriteString(fmt.Sprintf("  - %s\n", term))
		}
	}

	b.WriteString("\n## 本地调研结果\n")
	b.WriteString(fmt.Sprintf("- candidate_files_count: %d\n", len(build.Findings.CandidateFiles)))
	for _, file := range build.Findings.CandidateFiles {
		b.WriteString(fmt.Sprintf("  - %s\n", file))
	}
	b.WriteString(fmt.Sprintf("- candidate_dirs_count: %d\n", len(build.Findings.CandidateDirs)))
	for _, dir := range build.Findings.CandidateDirs {
		b.WriteString(fmt.Sprintf("  - %s\n", dir))
	}
	b.WriteString("- 本地风险备注：\n")
	if len(build.Findings.Notes) == 0 {
		b.WriteString("  - 无\n")
	} else {
		for _, note := range build.Findings.Notes {
			b.WriteString(fmt.Sprintf("  - %s\n", note))
		}
	}

	b.WriteString("\n## 本地基线复杂度评分\n")
	b.WriteString(fmt.Sprintf("- total: %d\n", build.Assessment.Total))
	b.WriteString(fmt.Sprintf("- level: %s\n", build.Assessment.Level))
	for _, dim := range build.Assessment.Dimensions {
		b.WriteString(fmt.Sprintf("  - %s: %d | %s\n", dim.Name, dim.Score, dim.Reason))
	}
	return b.String()
}

func ExtractPlanOutputs(raw string) (sections PlanAISections, ok bool) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	sectionMarkers := []struct {
		marker string
		target *string
	}{
		{marker: "=== IMPLEMENTATION SUMMARY ===", target: &sections.Summary},
		{marker: "=== IMPLEMENTATION STEPS ===", target: &sections.Steps},
		{marker: "=== RISK NOTES ===", target: &sections.Risks},
		{marker: "=== VALIDATION EXTRA ===", target: &sections.ValidationExtra},
	}

	indexes := make([]int, len(sectionMarkers))
	for i, item := range sectionMarkers {
		indexes[i] = strings.Index(normalized, item.marker)
	}
	if indexes[0] == -1 {
		return PlanAISections{}, false
	}

	for i := range sectionMarkers {
		start := indexes[i]
		if start == -1 {
			continue
		}
		contentStart := start + len(sectionMarkers[i].marker)
		end := len(normalized)
		for j := i + 1; j < len(sectionMarkers); j++ {
			if indexes[j] != -1 && indexes[j] > start {
				end = indexes[j]
				break
			}
		}
		*sectionMarkers[i].target = normalizeAISection(normalized[contentStart:end])
	}
	return sections, true
}

func ExtractPlanStream(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	planMarker := "=== IMPLEMENTATION SUMMARY ==="
	index := strings.Index(normalized, planMarker)
	if index == -1 {
		return ""
	}
	return strings.TrimSpace(normalized[index:])
}

func ValidatePlanOutputs(build *PlanBuild, ai PlanAISections) error {
	combined := strings.Join([]string{ai.Summary, ai.Steps, ai.Risks, ai.ValidationExtra}, "\n")
	for _, marker := range []string{"(待生成)", "(待确认)", "未初始化"} {
		if strings.Contains(combined, marker) {
			return fmt.Errorf("AI 输出包含无效占位符: %s", marker)
		}
	}
	if strings.TrimSpace(ai.Summary) == "" {
		return fmt.Errorf("AI plan 缺少实现概要")
	}
	if build.Assessment.Total <= 6 && strings.TrimSpace(ai.Steps) == "" {
		return fmt.Errorf("AI plan 缺少实施步骤")
	}
	for _, bad := range []string{"/livecoding:prd-refine", "/livecoding:prd-plan"} {
		if strings.Contains(combined, bad) {
			return fmt.Errorf("AI plan 包含错误命令示例: %s", bad)
		}
	}
	return nil
}

func normalizeAISection(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	lines := strings.Split(body, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func BuildPlanTasks(sections RefinedSections, findings ResearchFinding) []PlanTask {
	if len(findings.CandidateFiles) == 0 {
		return nil
	}

	refineFiles := filterFiles(findings.CandidateFiles, "prd_refine.go", "internal/prd/refine.go")
	statusFiles := filterFiles(findings.CandidateFiles, "prd_status.go", "internal/prd/status.go")
	planFiles := filterFiles(findings.CandidateFiles, "prd_plan.go", "internal/prd/plan.go")

	tasks := make([]PlanTask, 0, 3)
	if len(refineFiles) > 0 {
		tasks = append(tasks, PlanTask{
			ID:    "T1",
			Title: "补齐 PRD 来源识别与来源落盘",
			Goal:  "让飞书链接/文本来源在 refine 阶段就具备清晰、可追溯的来源信息和下一步提示。",
			Files: refineFiles,
			Input: []string{
				"用户输入的 PRD 文本 / 本地文件 / 飞书链接",
				"当前 task 的 source 元信息",
			},
			Output: []string{
				"清晰的 source.json / prd.source.md",
				"可靠的 prd-refined.md 或 fallback refined",
			},
			Actions: []string{
				"统一 refine 阶段的 source 识别、标题处理和来源元信息落盘。",
				"补齐飞书链接场景下的文档标识、来源信息和后续指引文案。",
				"保证 refined 失败或 fallback 时，用户仍能拿到可继续推进的 task 产物。",
			},
			Done: []string{
				"source.json / prd.source.md 能清晰体现来源类型和来源值。",
				"飞书链接场景下用户能明确知道来源文档信息和下一步操作。",
			},
		})
	}
	if len(statusFiles) > 0 {
		tasks = append(tasks, PlanTask{
			ID:        "T2",
			Title:     "完善 task 状态页与下一步提示",
			Goal:      "让 status 输出能准确反映来源、当前状态和下一步动作，减少用户猜测。",
			DependsOn: []string{"T1"},
			Files:     statusFiles,
			Input: []string{
				"task.json / source.json / prd.source.md / prd-refined.md",
				"当前 task 产物缺失情况",
			},
			Output: []string{
				"更清晰的 status 展示",
				"准确的 next command / 下一步提示",
			},
			Actions: []string{
				"补齐 task status 对 source 类型、来源路径/链接和产物状态的展示。",
				"根据当前 task 阶段输出更明确的 next command 或人工确认指引。",
			},
			Done: []string{
				"`coco-ext prd status` 能清晰展示来源信息、产物缺失情况和下一步命令。",
				"用户无需阅读源码即可判断当前 task 应继续 refine、plan 还是人工 review。",
			},
		})
	}
	if len(planFiles) > 0 {
		tasks = append(tasks, PlanTask{
			ID:        "T3",
			Title:     "同步设计模板、计划结构与 codegen 输入",
			Goal:      "让 plan 阶段产出可直接复用到 design、plan 和后续 codegen 的结构化结果。",
			DependsOn: []string{"T1", "T2"},
			Files:     planFiles,
			Input: []string{
				"refined PRD",
				"context 调研结果",
				"候选文件与复杂度评估",
			},
			Output: []string{
				"统一模板的 design.md",
				"带任务列表的 plan.md",
				"可供后续 codegen 消费的任务拆分结果",
			},
			Actions: []string{
				"将 design/plan 产物中的固定字段与动态内容进一步解耦。",
				"在 plan.md 中生成结构化任务列表，明确后续 codegen 的执行单元。",
				"保证 AI 只补充可变 section，不破坏程序控制的骨架和任务拆分。",
			},
			Done: []string{
				"design.md 使用统一模板输出，plan.md 包含可执行任务列表。",
				"后续 codegen 可以直接消费任务列表而不需要再次人工拆解需求。",
			},
		})
	}

	if len(tasks) == 0 {
		tasks = append(tasks, PlanTask{
			ID:    "T1",
			Title: "收敛实现范围并细化任务",
			Goal:  "在当前候选文件基础上收敛改动范围，为后续 codegen 提供可执行任务。",
			Files: findings.CandidateFiles,
			Input: []string{
				"候选文件列表",
				"refined PRD 与 context",
			},
			Output: []string{
				"更明确的任务拆分结果",
			},
			Actions: []string{
				"结合 refined PRD 和 context，继续人工确认候选文件的必要性。",
				"将文件级动作拆成更细粒度的实现任务。",
			},
			Done: []string{
				"每个候选文件都能归属到明确的实现任务中。",
			},
		})
	}

	return tasks
}

func filterFiles(files []string, keywords ...string) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		for _, keyword := range keywords {
			if strings.Contains(file, keyword) {
				result = append(result, file)
				break
			}
		}
	}
	return result
}

func describePlannedFileChange(file string) string {
	switch {
	case strings.Contains(file, "cmd/prd_refine.go"):
		return "调整 refine 命令的用户交互、来源提示和 fallback 提示。"
	case strings.Contains(file, "internal/prd/refine.go"):
		return "修改来源解析、task 元信息写入和 refined 内容校验逻辑。"
	case strings.Contains(file, "cmd/prd_status.go"):
		return "调整 status 命令输出，补充来源展示与下一步提示。"
	case strings.Contains(file, "internal/prd/status.go"):
		return "补充 task 状态计算、产物检查和 next command 生成逻辑。"
	case strings.Contains(file, "cmd/prd_plan.go"):
		return "调整 plan 命令交互输出、AI 阶段展示和 fallback 行为。"
	case strings.Contains(file, "internal/prd/plan.go"):
		return "完善 research、评分、模板渲染和后续 codegen 可消费的任务列表。"
	default:
		return suggestFileAction(file)
	}
}

func writePlanArtifacts(task *TaskStatusReport, designContent, planContent string, now time.Time, usedAI bool) (*PlanArtifacts, error) {
	designPath := filepath.Join(task.TaskDir, "design.md")
	planPath := filepath.Join(task.TaskDir, "plan.md")
	if err := os.WriteFile(designPath, []byte(designContent), 0644); err != nil {
		return nil, fmt.Errorf("写入 design.md 失败: %w", err)
	}
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		return nil, fmt.Errorf("写入 plan.md 失败: %w", err)
	}

	if err := updateTaskStatus(task.TaskDir, TaskStatusPlanned, now); err != nil {
		return nil, err
	}
	return &PlanArtifacts{DesignPath: designPath, PlanPath: planPath, UsedAI: usedAI}, nil
}

func updateTaskStatus(taskDir, status string, now time.Time) error {
	metaPath := filepath.Join(taskDir, "task.json")
	meta, err := readTaskMetadata(metaPath)
	if err != nil {
		return err
	}
	meta.Status = status
	meta.UpdatedAt = now
	return writeJSONFile(metaPath, meta)
}

func parseGlossaryEntries(content string) []GlossaryEntry {
	lines := strings.Split(content, "\n")
	entries := make([]GlossaryEntry, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.Contains(line, "---") || strings.Contains(line, "业务术语") {
			continue
		}
		parts := splitTableLine(line)
		if len(parts) < 4 {
			continue
		}
		business := strings.TrimSpace(parts[0])
		identifier := strings.TrimSpace(parts[1])
		module := strings.TrimSpace(parts[3])
		if business == "" || identifier == "" {
			continue
		}
		entries = append(entries, GlossaryEntry{
			Business:   business,
			Identifier: identifier,
			Module:     module,
		})
	}
	return entries
}

func splitTableLine(line string) []string {
	trimmed := strings.Trim(line, "|")
	rawParts := strings.Split(trimmed, "|")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		parts = append(parts, strings.TrimSpace(part))
	}
	return parts
}

func splitMarkdownSections(content string) map[string]string {
	lines := strings.Split(content, "\n")
	sections := make(map[string]string)
	current := ""
	var currentLines []string
	flush := func() {
		if current == "" {
			return
		}
		sections[current] = strings.TrimSpace(strings.Join(currentLines, "\n"))
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			currentLines = currentLines[:0]
			continue
		}
		if current != "" {
			currentLines = append(currentLines, line)
		}
	}
	flush()
	return sections
}

func cleanSectionLines(section string) string {
	lines := extractBulletItems(section)
	if len(lines) == 0 {
		return strings.TrimSpace(section)
	}
	return strings.Join(lines, "；")
}

func extractBulletItems(section string) []string {
	lines := strings.Split(section, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "1. ")
		line = strings.TrimPrefix(line, "2. ")
		line = strings.TrimPrefix(line, "3. ")
		line = strings.TrimPrefix(line, "4. ")
		line = strings.TrimPrefix(line, "5. ")
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

func inferUnmatchedTerms(searchText string, matched []GlossaryEntry) []string {
	terms := []string{}
	for _, keyword := range []string{"讲解卡", "倒计时", "拍卖", "隐藏", "已结束"} {
		if containsAny(searchText, keyword) && !matchedContainsBusiness(matched, keyword) {
			terms = append(terms, keyword)
		}
	}
	return dedupeTerms(terms)
}

func matchedContainsBusiness(entries []GlossaryEntry, keyword string) bool {
	for _, entry := range entries {
		if entry.Business == keyword {
			return true
		}
	}
	return false
}

func dedupeTerms(items []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func findCandidateFiles(repoRoot string, matched []GlossaryEntry, terms []string) []string {
	result := make([]string, 0, 8)
	seen := make(map[string]bool)
	preferred := preferPRDWorkflowFiles(terms)

	for _, path := range preferred {
		if seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
		if len(result) >= 6 {
			return result
		}
	}

	if len(result) >= 4 {
		return result
	}

	for _, entry := range matched {
		for _, term := range []string{entry.Identifier, entry.Business} {
			for _, file := range searchFiles(repoRoot, term) {
				if seen[file] {
					continue
				}
				seen[file] = true
				result = append(result, file)
				if len(result) >= 8 {
					return result
				}
			}
		}
	}

	for _, term := range terms {
		for _, file := range searchFiles(repoRoot, term) {
			if seen[file] {
				continue
			}
			seen[file] = true
			result = append(result, file)
			if len(result) >= 8 {
				sort.Strings(result)
				return result
			}
		}
	}
	sort.Strings(result)
	return result
}

func searchFiles(repoRoot, term string) []string {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil
	}

	cmd := exec.Command("rg", "--files-with-matches", "--glob", "*.go", term, ".")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		output = nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ".livecoding/") {
			continue
		}
		files = append(files, line)
	}

	if len(files) < 3 {
		fileCmd := exec.Command("rg", "--files", ".", "--glob", "*.go")
		fileCmd.Dir = repoRoot
		fileOutput, fileErr := fileCmd.Output()
		if fileErr == nil {
			lowerTerm := strings.ToLower(term)
			for _, line := range strings.Split(strings.TrimSpace(string(fileOutput)), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, ".livecoding/") {
					continue
				}
				if strings.Contains(strings.ToLower(line), lowerTerm) {
					files = append(files, line)
				}
			}
		}
	}

	return dedupeAndSort(files)
}

func heuristicCandidateFiles(repoRoot string, searchTerms []string) []string {
	preferred := preferPRDWorkflowFiles(searchTerms)
	if len(preferred) >= 8 {
		return preferred[:8]
	}

	fileCmd := exec.Command("rg", "--files", ".", "--glob", "*.go")
	fileCmd.Dir = repoRoot
	output, err := fileCmd.Output()
	if err != nil {
		return preferred
	}

	priorityKeywords := make([]string, 0, 12)
	for _, term := range searchTerms {
		switch term {
		case "prd", "plan", "refine", "status", "source", "lark", "feishu", "doc", "docx", "url", "task":
			priorityKeywords = append(priorityKeywords, term)
		}
	}
	if len(priorityKeywords) == 0 {
		priorityKeywords = []string{"prd", "plan", "refine", "status", "source"}
	}

	type scoredFile struct {
		path  string
		score int
	}
	scored := make([]scoredFile, 0, 16)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ".livecoding/") {
			continue
		}
		lower := strings.ToLower(line)
		score := 0
		for _, keyword := range priorityKeywords {
			if strings.Contains(lower, keyword) {
				score++
			}
		}
		if strings.Contains(lower, "cmd/prd_") || strings.Contains(lower, "internal/prd/") {
			score += 4
		}
		if strings.Contains(lower, "refine") || strings.Contains(lower, "status") || strings.Contains(lower, "source") {
			score += 2
		}
		if strings.Contains(lower, "cmd/") {
			score++
		}
		if score > 0 {
			scored = append(scored, scoredFile{path: line, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].path < scored[j].path
		}
		return scored[i].score > scored[j].score
	})

	result := make([]string, 0, len(scored))
	seen := make(map[string]bool, len(scored)+len(preferred))
	for _, path := range preferred {
		seen[path] = true
		result = append(result, path)
	}
	for _, item := range scored {
		if seen[item.path] {
			continue
		}
		seen[item.path] = true
		result = append(result, item.path)
		if len(result) >= 8 {
			break
		}
	}
	return result
}

func preferPRDWorkflowFiles(searchTerms []string) []string {
	candidates := []string{
		"./cmd/prd_refine.go",
		"./internal/prd/refine.go",
		"./cmd/prd_status.go",
		"./internal/prd/status.go",
		"./cmd/prd_plan.go",
		"./internal/prd/plan.go",
	}
	if len(searchTerms) == 0 {
		return candidates
	}

	matchedWorkflow := false
	for _, term := range searchTerms {
		switch strings.ToLower(term) {
		case "prd", "plan", "refine", "status", "source", "task", "lark", "feishu", "doc", "docx", "url":
			matchedWorkflow = true
		}
	}
	if matchedWorkflow {
		return candidates
	}
	return nil
}

func inferSearchTerms(title string, sections RefinedSections, matched []GlossaryEntry) []string {
	sourceText := strings.Join([]string{
		title,
		sections.Summary,
		strings.Join(sections.Features, "\n"),
		strings.Join(sections.Boundaries, "\n"),
		strings.Join(sections.BusinessRules, "\n"),
		strings.Join(sections.OpenQuestions, "\n"),
	}, "\n")

	terms := make([]string, 0, 24)
	for _, entry := range matched {
		terms = append(terms, entry.Business, entry.Identifier)
	}

	for _, token := range rePlanASCIIWord.FindAllString(sourceText, -1) {
		token = strings.ToLower(strings.TrimSpace(token))
		switch token {
		case "the", "and", "for", "with", "that", "this", "from":
			continue
		}
		terms = append(terms, token)
	}

	for keyword, expansions := range map[string][]string{
		"飞书":  {"lark", "feishu", "doc", "docx"},
		"文档":  {"doc", "document", "source"},
		"链接":  {"link", "url", "source"},
		"来源":  {"source", "doc_token"},
		"下一步": {"status", "next", "task"},
		"任务":  {"task", "status"},
		"提示":  {"status", "message"},
		"体验":  {"refine", "status", "source"},
		"需求":  {"prd", "refine", "plan"},
	} {
		if strings.Contains(sourceText, keyword) {
			terms = append(terms, expansions...)
		}
	}

	if strings.Contains(strings.ToLower(sourceText), "prd") {
		terms = append(terms, "prd", "refine", "plan", "status")
	}
	if strings.Contains(strings.ToLower(sourceText), "coco-ext") {
		terms = append(terms, "prd_refine", "prd_status", "refine", "status", "source")
	}

	return dedupeAndSort(terms)
}

func dedupeAndSort(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func summarizeDirs(files []string) []string {
	seen := make(map[string]bool)
	dirs := make([]string, 0, len(files))
	for _, file := range files {
		dir := filepath.Dir(file)
		if dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func hasPathKeyword(files []string, keywords ...string) bool {
	for _, file := range files {
		for _, keyword := range keywords {
			if strings.Contains(file, keyword) {
				return true
			}
		}
	}
	return false
}

func suggestFileAction(file string) string {
	switch {
	case strings.Contains(file, "/handler/"):
		return "评估接口层入参/返回或展示逻辑是否需要调整。"
	case strings.Contains(file, "/service/"):
		return "评估业务逻辑和下游调用是否需要补充。"
	case strings.Contains(file, "/converter/"):
		return "优先检查字段映射和 response 拼装逻辑。"
	case strings.Contains(file, "/model/"):
		return "检查结构体字段或状态定义是否需要扩展。"
	default:
		return "作为候选实现文件，需人工确认是否纳入本次改动范围。"
	}
}
