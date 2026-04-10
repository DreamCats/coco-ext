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

// AgentCodeResult 是 agent 模式代码生成的结果。
type AgentCodeResult struct {
	TaskID      string
	ToolEvents  []generator.ToolEvent
	AgentReply  string
	BuildOK     bool
	FilesChanged []string // agent 报告的改动文件列表
	Summary     string    // agent 报告的改动摘要
}

// BuildAgentCodePrompt 构建 agent 模式的 prompt。
// 给文件路径让 agent 自己读，不塞内容。
func BuildAgentCodePrompt(taskDir string) string {
	var b strings.Builder

	b.WriteString("你是一个代码实现 agent。请基于需求文档，在当前仓库中完成所有代码改动。\n\n")

	b.WriteString("## 输入文档\n\n")
	b.WriteString(fmt.Sprintf("1. 实施计划: %s\n", filepath.Join(taskDir, "plan.md")))
	b.WriteString(fmt.Sprintf("2. 技术方案: %s\n", filepath.Join(taskDir, "design.md")))
	b.WriteString(fmt.Sprintf("3. 需求文档: %s\n\n", filepath.Join(taskDir, "prd-refined.md")))

	b.WriteString("## 工作流程\n\n")
	b.WriteString("1. 先读取 plan.md，了解要改哪些文件、每个文件的改动目标\n")
	b.WriteString("2. 再读取 design.md，了解技术方案细节\n")
	b.WriteString("3. 逐个读取需要修改的源文件，理解现有代码\n")
	b.WriteString("4. 使用 Edit 工具逐个修改文件（优先局部修改，不要整文件覆写）\n")
	b.WriteString("5. 所有文件改完后，执行 `go build ./涉及的包/...` 验证编译\n")
	b.WriteString("6. 如果编译失败，自行修复直到编译通过\n\n")

	b.WriteString("## 规则\n\n")
	b.WriteString("- 严格按照 plan.md 中「拟改文件」列表工作，不要改动计划外的文件\n")
	b.WriteString("- 保持原有代码风格、import 顺序、注释风格\n")
	b.WriteString("- 不要添加多余的注释或文档\n")
	b.WriteString("- 最终必须确保编译通过\n\n")

	b.WriteString("## 输出要求\n\n")
	b.WriteString("所有代码改动完成并验证编译后，在最后输出结构化结果：\n\n")
	b.WriteString("=== CODE RESULT ===\n")
	b.WriteString("build: passed 或 failed\n")
	b.WriteString("files:\n")
	b.WriteString("  - path/to/file1.go\n")
	b.WriteString("  - path/to/file2.go\n")
	b.WriteString("summary: 一句话描述做了什么改动\n")
	b.WriteString("=== END ===\n")

	return b.String()
}

// GenerateCodeWithAgent 使用 agent 模式生成代码。
// agent 自主读文件、改代码、跑编译，coco-ext 只做编排。
func GenerateCodeWithAgent(agent *generator.AgentGenerator, taskDir string, now time.Time, onChunk func(string), onTool func(generator.ToolEvent)) (*AgentCodeResult, error) {
	prompt := BuildAgentCodePrompt(taskDir)

	var toolEvents []generator.ToolEvent
	wrappedOnTool := func(event generator.ToolEvent) {
		toolEvents = append(toolEvents, event)
		if onTool != nil {
			onTool(event)
		}
	}

	reply, err := agent.PromptWithTools(prompt, config.CodePromptTimeout, onChunk, wrappedOnTool)
	if err != nil {
		if reply == "" {
			return nil, fmt.Errorf("agent 代码生成失败: %w", err)
		}
		// 超时但有部分输出，仍然返回结果
	}

	// 优先解析结构化结果，失败则 fallback 到启发式
	cr := parseCodeResult(reply)
	if cr == nil {
		cr = inferCodeResult(toolEvents, reply)
	}

	if cr.BuildOK {
		_ = updateTaskStatus(taskDir, TaskStatusCoded, now)
	}

	return &AgentCodeResult{
		ToolEvents:   toolEvents,
		AgentReply:   reply,
		BuildOK:      cr.BuildOK,
		FilesChanged: cr.Files,
		Summary:      cr.Summary,
	}, nil
}

// codeResult 是从 agent 输出中解析出的结构化结果。
type codeResult struct {
	BuildOK bool
	Files   []string
	Summary string
}

// parseCodeResult 从 agent 输出中解析 === CODE RESULT === 块。
func parseCodeResult(raw string) *codeResult {
	const marker = "=== CODE RESULT ==="
	const endMarker = "=== END ==="

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	start := strings.LastIndex(normalized, marker)
	if start == -1 {
		return nil
	}

	content := normalized[start+len(marker):]
	if end := strings.Index(content, endMarker); end != -1 {
		content = content[:end]
	}

	result := &codeResult{}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "build:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "build:"))
			result.BuildOK = val == "passed" || val == "ok" || val == "true"
			continue
		}

		if strings.HasPrefix(line, "summary:") {
			result.Summary = strings.TrimSpace(strings.TrimPrefix(line, "summary:"))
			continue
		}

		// files 列表项
		if strings.HasPrefix(line, "- ") {
			file := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if file != "" {
				result.Files = append(result.Files, file)
			}
		}
	}

	// 如果解析到了 build 或 files，认为有效
	if result.Summary != "" || len(result.Files) > 0 {
		return result
	}
	return nil
}

// inferCodeResult 是启发式 fallback，当 agent 未输出结构化结果时使用。
func inferCodeResult(events []generator.ToolEvent, reply string) *codeResult {
	cr := &codeResult{}

	// 检查是否有 bash 工具调用了 go build
	hasBuild := false
	for _, e := range events {
		if e.Kind == "bash" && strings.Contains(e.Title, "go build") {
			hasBuild = true
		}
	}

	// 从 reply 推断
	lower := strings.ToLower(reply)
	if strings.Contains(lower, "编译通过") || strings.Contains(lower, "build passed") ||
		strings.Contains(lower, "编译成功") || strings.Contains(lower, "successfully") {
		cr.BuildOK = true
	} else if !strings.Contains(lower, "编译失败") && !strings.Contains(lower, "build failed") &&
		!strings.Contains(lower, "build error") {
		cr.BuildOK = hasBuild
	}

	// 从 reply 中提取文件路径作为 fallback
	cr.Files = extractFilesFromReply(reply)

	return cr
}

// extractFilesFromReply 从 agent 回复中提取 .go 文件路径。
func extractFilesFromReply(reply string) []string {
	var files []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		// 匹配形如 "path/to/file.go" 的行
		if strings.Contains(line, ".go") && (strings.Contains(line, "/") || strings.Contains(line, ".go:")) {
			// 提取可能的文件路径
			for _, token := range strings.Fields(line) {
				token = strings.TrimRight(token, "：:.,;)")
				if strings.HasSuffix(token, ".go") && strings.Contains(token, "/") && !seen[token] {
					seen[token] = true
					files = append(files, token)
				}
			}
		}
	}
	return files
}

// PrepareAgentCode 校验 task 状态，返回必要信息（不读源文件，让 agent 自己读）。
func PrepareAgentCode(repoRoot, taskID string) (taskDir string, err error) {
	task, err := LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return "", err
	}

	if task.Metadata.Status != TaskStatusPlanned && task.Metadata.Status != TaskStatusCoded {
		return "", fmt.Errorf("task 状态为 %s，需要先执行 coco-ext prd plan 使其达到 planned 状态", task.Metadata.Status)
	}

	// 只检查 plan.md 和 design.md 存在
	for _, name := range []string{"plan.md", "design.md"} {
		path := filepath.Join(task.TaskDir, name)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("%s 不存在: %w", name, err)
		}
	}

	return task.TaskDir, nil
}
