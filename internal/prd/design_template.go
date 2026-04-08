package prd

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const defaultDesignTemplateName = "content-ecommerce"

//go:embed templates/design/*.md.tmpl
var designTemplateFS embed.FS

type DesignInfoItem struct {
	Label string
	Value string
}

type DesignRegionMode struct {
	Region string
	Mode   string
	Note   string
}

type DesignManpowerRow struct {
	PSM     string
	Content string
	Effort  string
	Owner   string
}

type DesignDiagram struct {
	Title   string
	Type    string
	Content string
}

type DesignTemplateData struct {
	Title               string
	TaskID              string
	Status              string
	ComplexityLevel     string
	ComplexityTotal     int
	IsSimple            bool
	InfoItems           []DesignInfoItem
	Why                 string
	ChangePoints        []string
	ScopeFiles          []string
	ExcludedScope       []string
	SystemDesign        []string
	IDLChanges          []string
	StorageChanges      []string
	DependencyChanges   []string
	ExperimentChanges   []string
	ShowMultiRegion     bool
	RegionModes         []DesignRegionMode
	ShowRegionIsolation bool
	RegionIsolation     []string
	ShowCompliance      bool
	ComplianceNotes     []string
	MonitoringNotes     []string
	PerformanceNotes    []string
	QAInputs            []string
	RolloutPlan         []string
	RollbackPlan        []string
	ManpowerRows        []DesignManpowerRow
	PostLaunchNotes     []string
	DesignNotes         []string
	Diagrams            []DesignDiagram
}

func BuildDesignContent(repoRoot string, task *TaskStatusReport, context *ContextSnapshot, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) string {
	data := BuildDesignTemplateData(repoRoot, task, context, sections, findings, assessment)
	content, err := RenderDesignTemplate(defaultDesignTemplateName, data)
	if err != nil {
		return BuildFallbackDesignContent(task, context, sections, findings, assessment)
	}
	return content
}

func BuildDesignTemplateData(repoRoot string, task *TaskStatusReport, context *ContextSnapshot, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) DesignTemplateData {
	projectName := detectProjectName(repoRoot)

	sourceValue := task.Metadata.SourceValue
	if task.Source != nil {
		switch {
		case task.Source.URL != "":
			sourceValue = fmt.Sprintf("[%s](%s)", task.Source.Title, task.Source.URL)
		case task.Source.Path != "":
			sourceValue = task.Source.Path
		case task.Source.Title != "":
			sourceValue = task.Source.Title
		}
	}
	if strings.TrimSpace(sourceValue) == "" {
		sourceValue = task.Metadata.Title
	}

	changePoints := make([]string, 0, 3)
	for _, feature := range sections.Features {
		if strings.TrimSpace(feature) == "" {
			continue
		}
		changePoints = append(changePoints, feature)
		if len(changePoints) >= 3 {
			break
		}
	}
	if len(changePoints) == 0 && sections.Summary != "" {
		changePoints = append(changePoints, sections.Summary)
	}
	if len(changePoints) == 0 {
		changePoints = append(changePoints, task.Metadata.Title)
	}

	scopeFiles := findings.CandidateFiles
	if len(scopeFiles) == 0 {
		scopeFiles = []string{"当前未命中候选文件，需要补充 context 或人工确认模块。"}
	}

	excludedScope := buildExcludedScope(sections)
	systemDesign := buildDesignSystemNotes(findings, assessment)
	idlChanges := buildIDLChanges(findings)
	storageChanges := buildStorageChanges(sections)
	dependencyChanges := buildDependencyChanges(sections)
	experimentChanges := []string{"本次需求不涉及实验平台或新增埋点。"}
	monitoringNotes := buildMonitoringNotes(findings, sections)
	performanceNotes := buildPerformanceNotes(sections)
	qaInputs := buildDesignQAInputs(sections, findings)
	rolloutPlan := buildRolloutPlan(projectName)
	rollbackPlan := buildRollbackPlan(projectName)
	manpowerRows := []DesignManpowerRow{
		{
			PSM:     projectName,
			Content: summarizeManpowerContent(sections),
			Effort:  estimateManpower(assessment),
			Owner:   "待补充",
		},
	}
	postLaunchNotes := []string{
		"上线后补充真实使用反馈与效果验证。",
		"评估是否需要后续迭代优化。",
	}

	designNotes := make([]string, 0, len(findings.Notes))
	for _, note := range findings.Notes {
		designNotes = append(designNotes, note)
	}
	if len(findings.UnmatchedTerms) > 0 {
		designNotes = append(designNotes, "存在 glossary 未命中的术语，需结合实际代码进一步人工确认。")
	}

	return DesignTemplateData{
		Title:           task.Metadata.Title,
		TaskID:          task.TaskID,
		Status:          TaskStatusPlanned,
		ComplexityLevel: assessment.Level,
		ComplexityTotal: assessment.Total,
		IsSimple:        assessment.Level == "简单",
		InfoItems: []DesignInfoItem{
			{Label: "PRD / Bug单(Hotfix)", Value: sourceValue},
			{Label: "Meego", Value: "N/A"},
			{Label: "PM", Value: "待补充"},
			{Label: "Tech Owner", Value: "待补充"},
			{Label: "Server", Value: projectName},
			{Label: "FE", Value: "N/A"},
			{Label: "Client", Value: "N/A"},
			{Label: "QA", Value: "待补充"},
			{Label: "客户端跟版版本(如有)", Value: "N/A"},
			{Label: "PPE环境", Value: "N/A"},
		},
		Why:               buildDesignWhy(task, sections),
		ChangePoints:      changePoints,
		ScopeFiles:        scopeFiles,
		ExcludedScope:     excludedScope,
		SystemDesign:      systemDesign,
		IDLChanges:        idlChanges,
		StorageChanges:    storageChanges,
		DependencyChanges: dependencyChanges,
		ExperimentChanges: experimentChanges,
		ShowMultiRegion:   false,
		RegionModes: []DesignRegionMode{
			{Region: "US", Mode: "N/A", Note: "待评估"},
			{Region: "UK", Mode: "N/A", Note: "待评估"},
			{Region: "EU", Mode: "N/A", Note: "待评估"},
			{Region: "JP", Mode: "N/A", Note: "待评估"},
			{Region: "SEA", Mode: "N/A", Note: "待评估"},
			{Region: "LATAM", Mode: "N/A", Note: "待评估"},
		},
		ShowRegionIsolation: false,
		RegionIsolation:     []string{"待评估是否涉及区域隔离。"},
		ShowCompliance:      false,
		ComplianceNotes:     []string{"本次需求不涉及新增合规逻辑。"},
		MonitoringNotes:     monitoringNotes,
		PerformanceNotes:    performanceNotes,
		QAInputs:            qaInputs,
		RolloutPlan:         rolloutPlan,
		RollbackPlan:        rollbackPlan,
		ManpowerRows:        manpowerRows,
		PostLaunchNotes:     postLaunchNotes,
		DesignNotes:         designNotes,
		Diagrams:            buildDesignDiagrams(task, sections, findings, assessment),
	}
}

func RenderDesignTemplate(name string, data DesignTemplateData) (string, error) {
	templatePath := fmt.Sprintf("templates/design/%s.md.tmpl", name)
	raw, err := designTemplateFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("读取 design 模板失败: %w", err)
	}

	tpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("解析 design 模板失败: %w", err)
	}

	var b bytes.Buffer
	if err := tpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("渲染 design 模板失败: %w", err)
	}
	return strings.TrimSpace(b.String()) + "\n", nil
}

func BuildFallbackDesignContent(task *TaskStatusReport, context *ContextSnapshot, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) string {
	var b strings.Builder
	b.WriteString("# Design\n\n")
	b.WriteString(fmt.Sprintf("- task_id: %s\n", task.TaskID))
	b.WriteString(fmt.Sprintf("- title: %s\n", task.Metadata.Title))
	b.WriteString(fmt.Sprintf("- status: %s\n\n", TaskStatusPlanned))
	b.WriteString("## 调研摘要\n\n")
	if sections.Summary != "" {
		b.WriteString(sections.Summary + "\n\n")
	}
	b.WriteString("## Context 检查\n\n")
	b.WriteString(fmt.Sprintf("- glossary: %s\n", context.GlossaryPath))
	b.WriteString(fmt.Sprintf("- architecture: %s\n", context.ArchitecturePath))
	b.WriteString(fmt.Sprintf("- patterns: %s\n", context.PatternsPath))
	if context.GotchasPath != "" {
		b.WriteString(fmt.Sprintf("- gotchas: %s\n", context.GotchasPath))
	}
	b.WriteString("\n## 候选实现范围\n\n")
	for _, file := range findings.CandidateFiles {
		b.WriteString(fmt.Sprintf("- %s\n", file))
	}
	b.WriteString("\n## 复杂度评估\n\n")
	b.WriteString(fmt.Sprintf("- 总分: %d\n", assessment.Total))
	b.WriteString(fmt.Sprintf("- 等级: %s\n", assessment.Level))
	b.WriteString(fmt.Sprintf("- 结论: %s\n", assessment.Conclusion))
	return b.String()
}

// detectProjectName 从 go.mod 或目录名推断项目名称
func detectProjectName(repoRoot string) string {
	goModPath := filepath.Join(repoRoot, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				mod := strings.TrimPrefix(line, "module ")
				mod = strings.TrimSpace(mod)
				// 取最后一段作为项目名
				parts := strings.Split(mod, "/")
				return parts[len(parts)-1]
			}
		}
	}
	return filepath.Base(repoRoot)
}

func buildDesignWhy(task *TaskStatusReport, sections RefinedSections) string {
	if strings.TrimSpace(sections.Summary) != "" {
		return sections.Summary
	}
	return task.Metadata.Title
}

func buildExcludedScope(sections RefinedSections) []string {
	excluded := []string{"不在本次范围内的模块和功能保持不变。"}
	if len(sections.Boundaries) > 0 {
		for _, boundary := range sections.Boundaries {
			if strings.Contains(boundary, "不涉及") || strings.Contains(boundary, "不包含") ||
				strings.Contains(boundary, "不在") || strings.Contains(boundary, "仅") {
				excluded = append(excluded, boundary)
			}
		}
	}
	return excluded
}

func buildDesignSystemNotes(findings ResearchFinding, assessment ComplexityAssessment) []string {
	notes := make([]string, 0, 4)
	if len(findings.CandidateFiles) > 0 {
		notes = append(notes, fmt.Sprintf("基于本地调研结果，候选实现文件共 %d 个。", len(findings.CandidateFiles)))
		dirs := summarizeDirsForNotes(findings.CandidateDirs)
		if dirs != "" {
			notes = append(notes, fmt.Sprintf("改动集中在 %s。", dirs))
		}
	} else {
		notes = append(notes, "当前未命中明确候选文件，建议通过 context 和人工确认收敛实现范围。")
	}
	notes = append(notes, fmt.Sprintf("当前复杂度评估为 %s（%d 分），实现时优先保持最小改动范围。", assessment.Level, assessment.Total))
	return notes
}

func summarizeDirsForNotes(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	if len(dirs) <= 3 {
		return strings.Join(dirs, "、")
	}
	return strings.Join(dirs[:3], "、") + " 等目录"
}

func buildIDLChanges(findings ResearchFinding) []string {
	for _, file := range findings.CandidateFiles {
		if strings.HasSuffix(file, ".proto") || strings.HasSuffix(file, ".thrift") {
			return []string{"候选文件中包含 IDL 文件，需评估是否涉及协议变更。"}
		}
	}
	return []string{"本次需求不涉及 IDL、协议字段或公开 API 变更。"}
}

func buildStorageChanges(sections RefinedSections) []string {
	joined := strings.Join(append(sections.Features, sections.Boundaries...), "\n")
	if containsAny(joined, "数据库", "表", "持久化", "存储", "配置中心", "Redis", "MySQL", "MongoDB") {
		return []string{"需求描述中涉及存储或配置变更，需人工评估具体方案。"}
	}
	return []string{"本次需求不涉及数据库、持久化模型或配置中心变更。"}
}

func buildDependencyChanges(sections RefinedSections) []string {
	joined := strings.Join(append(sections.Features, sections.Boundaries...), "\n")
	if containsAny(joined, "下游", "依赖", "第三方", "外部服务", "RPC", "HTTP") {
		return []string{"需求描述中涉及外部依赖变更，需人工评估影响范围。"}
	}
	return []string{"不新增下游服务依赖。"}
}

func buildMonitoringNotes(findings ResearchFinding, sections RefinedSections) []string {
	notes := make([]string, 0, 2)
	if len(findings.CandidateFiles) > 0 {
		notes = append(notes, fmt.Sprintf("关注候选文件改动后的功能是否正常（共 %d 个文件）。", len(findings.CandidateFiles)))
	}
	if len(sections.OpenQuestions) > 0 {
		notes = append(notes, "存在待确认问题，上线后需重点观察相关场景。")
	}
	if len(notes) == 0 {
		notes = append(notes, "按常规监控关注即可。")
	}
	return notes
}

func buildPerformanceNotes(sections RefinedSections) []string {
	joined := strings.Join(append(sections.Features, sections.Boundaries...), "\n")
	if containsAny(joined, "性能", "流量", "QPS", "延迟", "并发", "批量") {
		return []string{"需求涉及性能相关描述，需人工评估性能影响与流量预估。"}
	}
	return []string{"本次改动预计不影响线上性能，无额外流量评估需求。"}
}

func buildDesignQAInputs(sections RefinedSections, findings ResearchFinding) []string {
	inputs := make([]string, 0, 8)

	// 从功能点生成验证项
	for i, feature := range sections.Features {
		if i >= 3 {
			break
		}
		inputs = append(inputs, fmt.Sprintf("验证功能点：%s", feature))
	}

	// 从边界条件生成验证项
	for i, boundary := range sections.Boundaries {
		if i >= 2 {
			break
		}
		inputs = append(inputs, fmt.Sprintf("验证边界条件：%s", boundary))
	}

	// 从待确认问题生成验证项
	for _, question := range sections.OpenQuestions {
		inputs = append(inputs, fmt.Sprintf("人工确认并补测：%s", question))
	}

	if len(findings.CandidateFiles) > 0 {
		inputs = append(inputs, fmt.Sprintf("重点回归候选实现文件：%s。", strings.Join(findings.CandidateFiles, "、")))
	}

	if len(inputs) == 0 {
		inputs = append(inputs, "按需求描述验证核心功能是否符合预期。")
	}

	return inputs
}

func buildRolloutPlan(projectName string) []string {
	return []string{
		fmt.Sprintf("通过 %s 的常规发布流程上线。", projectName),
		"上线后重点验证需求涉及的核心路径。",
	}
}

func buildRollbackPlan(projectName string) []string {
	return []string{
		fmt.Sprintf("如出现问题，通过 %s 的常规回滚流程回退到上一版本。", projectName),
	}
}

func summarizeManpowerContent(sections RefinedSections) string {
	if len(sections.Features) > 0 {
		// 取第一个功能点做摘要
		summary := sections.Features[0]
		if len(summary) > 40 {
			summary = summary[:40] + "..."
		}
		return summary
	}
	return "需求实现"
}

func estimateManpower(assessment ComplexityAssessment) string {
	switch {
	case assessment.Total <= 2:
		return "≤0.5pd"
	case assessment.Level == "简单":
		return "0.5-1pd"
	case assessment.Level == "中等":
		return "2-3pd"
	default:
		return "5pd+"
	}
}

func buildDesignFlowchartMermaid(findings ResearchFinding, sections RefinedSections) string {
	var b strings.Builder
	b.WriteString("flowchart TD\n")

	if len(sections.Features) > 0 {
		b.WriteString(fmt.Sprintf("  A[%s] --> B[实现改动]\n", truncateForMermaid(sections.Features[0], 40)))
	} else {
		b.WriteString("  A[需求输入] --> B[实现改动]\n")
	}

	if len(findings.CandidateDirs) > 0 {
		for i, dir := range findings.CandidateDirs {
			if i >= 3 {
				break
			}
			nodeID := string(rune('C' + i))
			b.WriteString(fmt.Sprintf("  B --> %s[修改 %s]\n", nodeID, dir))
		}
	} else {
		b.WriteString("  B --> C[待确认改动范围]\n")
	}

	return b.String()
}

func truncateForMermaid(s string, maxLen int) string {
	// 移除 mermaid 特殊字符
	s = strings.NewReplacer("[", "(", "]", ")", "{", "(", "}", ")").Replace(s)
	if len([]rune(s)) > maxLen {
		return string([]rune(s)[:maxLen]) + "..."
	}
	return s
}

func buildDesignSequenceMermaid(sections RefinedSections, findings ResearchFinding) string {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	b.WriteString("  participant U as User\n")
	b.WriteString("  participant S as System\n")

	if len(sections.Features) > 0 {
		b.WriteString(fmt.Sprintf("  U->>S: %s\n", truncateForMermaid(sections.Features[0], 50)))
	} else {
		b.WriteString("  U->>S: 触发需求场景\n")
	}

	if len(findings.CandidateDirs) > 0 {
		b.WriteString(fmt.Sprintf("  S->>S: 处理逻辑（涉及 %s）\n", truncateForMermaid(strings.Join(findings.CandidateDirs, ", "), 40)))
	}

	b.WriteString("  S-->>U: 返回结果\n")

	return strings.TrimSpace(b.String())
}

func buildDesignDiagrams(_ *TaskStatusReport, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) []DesignDiagram {
	// 简单需求不生成图示
	if assessment.Level == "简单" {
		return nil
	}

	diagrams := make([]DesignDiagram, 0, 2)

	if len(findings.CandidateFiles) > 0 {
		diagrams = append(diagrams, DesignDiagram{
			Title:   "改动流程图",
			Type:    "mermaid",
			Content: buildDesignFlowchartMermaid(findings, sections),
		})
	}

	if assessment.Total >= 6 && len(findings.CandidateDirs) > 0 {
		diagrams = append(diagrams, DesignDiagram{
			Title:   "时序图",
			Type:    "mermaid",
			Content: buildDesignSequenceMermaid(sections, findings),
		})
	}

	return diagrams
}
