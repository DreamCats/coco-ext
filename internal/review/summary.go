package review

import (
	"fmt"
	"strings"
	"time"
)

func BuildReport(inputs ReportInputs) string {
	allFindings := mergeFindings(inputs.Scope.Findings, inputs.Release.Findings, inputs.Impact.Findings, inputs.Quality.Findings)
	p0 := filterFindings(allFindings, SeverityP0)
	p1 := filterFindings(allFindings, SeverityP1)
	p2 := filterFindings(allFindings, SeverityP2)

	rating, advice := classifyReview(p0, p1)
	var b strings.Builder

	b.WriteString("# Auto Review 综合审查报告\n\n")
	b.WriteString(fmt.Sprintf("> 分支：%s → %s  \n", inputs.Facts.DiffInfo.SourceBranch, inputs.Facts.DiffInfo.TargetBranch))
	b.WriteString(fmt.Sprintf("> 改动：%d 个文件，+%d/-%d 行  \n", inputs.Facts.DiffInfo.FileCount, inputs.Facts.DiffInfo.Additions, inputs.Facts.DiffInfo.Deletions))
	b.WriteString(fmt.Sprintf("> 审查时间：%s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString(renderChangeSummary(inputs.Facts))

	b.WriteString(fmt.Sprintf("## 综合评级：%s\n\n", rating))
	if inputs.Quality.Summary != "" {
		b.WriteString(fmt.Sprintf("> AI 总结：%s\n\n", inputs.Quality.Summary))
	}

	b.WriteString(renderFindingSection("P0 — 必须修复", p0))
	b.WriteString(renderFindingSection("P1 — 建议修复", p1))
	b.WriteString(renderFindingSection("P2 — 可选优化", p2))

	b.WriteString("## 模块详情\n\n")
	b.WriteString(renderScopeDetails(inputs.Scope))
	b.WriteString(renderImpactDetails(inputs.Impact))
	b.WriteString(renderReleaseDetails(inputs.Release))
	b.WriteString(renderQualityDetails(inputs.Quality))

	b.WriteString("## 操作建议\n\n")
	b.WriteString(advice)
	b.WriteString("\n")

	return strings.TrimSpace(b.String()) + "\n"
}

func mergeFindings(groups ...[]Finding) []Finding {
	merged := make([]Finding, 0)
	for _, group := range groups {
		merged = append(merged, group...)
	}
	return merged
}

func filterFindings(findings []Finding, severity Severity) []Finding {
	result := make([]Finding, 0)
	for _, finding := range findings {
		if finding.Severity == severity {
			result = append(result, finding)
		}
	}
	return result
}

func classifyReview(p0, p1 []Finding) (string, string) {
	switch {
	case len(p0) > 0:
		return "🔴 需修复", "发现必须修复的问题，建议先处理 P0 后再重新运行 review。"
	case len(p1) > 2:
		return "🟡 建议修复后提交", "当前存在较多建议修复项，建议处理主要 P1 后再提交。"
	default:
		return "🟢 可提交", "整体风险可控，可以提交；如有 P1/P2，建议按优先级后续处理。"
	}
}

func renderFindingSection(title string, findings []Finding) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", title))
	if len(findings) == 0 {
		b.WriteString("未发现相关问题。\n\n")
		return b.String()
	}
	for _, finding := range findings {
		label := finding.Source
		if finding.File != "" {
			b.WriteString(fmt.Sprintf("- [%s] `%s`: %s。%s", label, finding.File, finding.Title, finding.Detail))
		} else {
			b.WriteString(fmt.Sprintf("- [%s] %s。%s", label, finding.Title, finding.Detail))
		}
		if finding.Suggestion != "" {
			b.WriteString(fmt.Sprintf(" 建议：%s", finding.Suggestion))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func renderScopeDetails(scope ScopeResult) string {
	var b strings.Builder
	b.WriteString("### Scope Guard\n\n")
	b.WriteString(fmt.Sprintf("- 核心改动：%s\n", renderPathList(scope.Core)))
	b.WriteString(fmt.Sprintf("- 边缘改动：%s\n", renderPathList(scope.Edge)))
	b.WriteString(fmt.Sprintf("- 离群改动：%s\n\n", renderPathList(scope.Outliers)))
	return b.String()
}

func renderImpactDetails(impact ImpactResult) string {
	var b strings.Builder
	b.WriteString("### Impact Lite\n\n")
	if impact.Skipped {
		b.WriteString("- 跳过原因：" + impact.Reason + "\n\n")
		return b.String()
	}
	if len(impact.Findings) == 0 {
		b.WriteString("- 未发现需要额外关注的影响面。\n\n")
		return b.String()
	}
	for _, finding := range impact.Findings {
		b.WriteString(fmt.Sprintf("- %s。%s\n", finding.Title, finding.Detail))
	}
	b.WriteString("\n")
	return b.String()
}

func renderReleaseDetails(release ReleaseResult) string {
	var b strings.Builder
	b.WriteString("### Release Gate Lite\n\n")
	if len(release.Findings) == 0 {
		b.WriteString("- 未检测到额外的发布风险信号。\n\n")
		return b.String()
	}
	for _, finding := range release.Findings {
		detail := finding.Title
		if finding.File != "" {
			detail = fmt.Sprintf("`%s`: %s", finding.File, finding.Title)
		}
		b.WriteString(fmt.Sprintf("- [%s] %s。%s\n", finding.Severity, detail, finding.Detail))
	}
	b.WriteString("\n")
	return b.String()
}

func renderQualityDetails(quality QualityResult) string {
	var b strings.Builder
	b.WriteString("### AI Code Review\n\n")
	if quality.Summary != "" {
		b.WriteString("- 总结：" + quality.Summary + "\n")
	}
	if len(quality.Findings) == 0 {
		b.WriteString("- 未提取到结构化代码质量问题。\n")
		if quality.Raw != "" {
			b.WriteString("- 原始输出：\n\n")
			b.WriteString("```text\n")
			b.WriteString(quality.Raw)
			b.WriteString("\n```\n\n")
			return b.String()
		}
		b.WriteString("\n")
		return b.String()
	}
	for _, finding := range quality.Findings {
		if finding.File != "" {
			b.WriteString(fmt.Sprintf("- [%s] `%s`: %s。%s\n", finding.Severity, finding.File, finding.Title, finding.Detail))
		} else {
			b.WriteString(fmt.Sprintf("- [%s] %s。%s\n", finding.Severity, finding.Title, finding.Detail))
		}
	}
	b.WriteString("\n")
	return b.String()
}

func renderPathList(paths []string) string {
	if len(paths) == 0 {
		return "无"
	}
	return strings.Join(paths, ", ")
}

func renderChangeSummary(facts Facts) string {
	docCount := 0
	goCount := 0
	configCount := 0
	idlCount := 0
	otherCount := 0
	for _, file := range facts.Files {
		switch {
		case file.IsGoLike:
			goCount++
		case file.IsDocLike:
			docCount++
		case file.IsConfigLike:
			configCount++
		case file.IsIDLLike:
			idlCount++
		default:
			otherCount++
		}
	}

	var changeType string
	switch {
	case len(facts.Files) == 0:
		changeType = "无变更"
	case docCount == len(facts.Files):
		changeType = "文档变更"
	case goCount == len(facts.Files):
		changeType = "代码变更"
	case configCount == len(facts.Files):
		changeType = "配置变更"
	default:
		changeType = "混合变更"
	}

	mainFiles := make([]string, 0, min(len(facts.Files), 3))
	for i, file := range facts.Files {
		if i >= 3 {
			break
		}
		mainFiles = append(mainFiles, file.Path)
	}

	var b strings.Builder
	b.WriteString("## 变更摘要\n\n")
	b.WriteString(fmt.Sprintf("- 变更类型：%s\n", changeType))
	b.WriteString(fmt.Sprintf("- 文件分布：代码 %d / 文档 %d / 配置 %d / IDL %d / 其他 %d\n", goCount, docCount, configCount, idlCount, otherCount))
	if len(mainFiles) > 0 {
		b.WriteString(fmt.Sprintf("- 主要文件：%s\n", strings.Join(mainFiles, ", ")))
	}
	b.WriteString("\n")
	return b.String()
}
