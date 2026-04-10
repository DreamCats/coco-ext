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
	TaskID     string
	ToolEvents []generator.ToolEvent
	AgentReply string
	BuildOK    bool
}

// BuildAgentCodePrompt 构建 agent 模式的 prompt。
// 将 plan/design 内容直接嵌入 prompt（而非给文件路径），
// 避免 agent 通过文件路径锚定到主仓库而非 worktree。
func BuildAgentCodePrompt(taskDir, workDir string) (string, error) {
	planContent, err := os.ReadFile(filepath.Join(taskDir, "plan.md"))
	if err != nil {
		return "", fmt.Errorf("读取 plan.md 失败: %w", err)
	}
	designContent, err := os.ReadFile(filepath.Join(taskDir, "design.md"))
	if err != nil {
		return "", fmt.Errorf("读取 design.md 失败: %w", err)
	}

	var b strings.Builder

	b.WriteString("你是一个代码实现 agent。请基于下方提供的技术方案和实施计划，在当前仓库中完成所有代码改动。\n\n")

	b.WriteString("## 技术方案 (design.md)\n\n")
	b.WriteString(string(designContent))
	b.WriteString("\n\n")

	b.WriteString("## 实施计划 (plan.md)\n\n")
	b.WriteString(string(planContent))
	b.WriteString("\n\n")

	b.WriteString("## 工作流程\n\n")
	b.WriteString("1. 根据上方 plan.md 中「拟改文件」列表，逐个读取需要修改的源文件\n")
	b.WriteString("2. 使用 Edit 工具逐个修改文件（优先局部修改，不要整文件覆写）\n")
	b.WriteString("3. 所有文件改完后，执行 `go build ./涉及的包/...` 验证编译\n")
	b.WriteString("4. 如果编译失败，自行修复直到编译通过\n\n")

	b.WriteString("## 规则\n\n")
	b.WriteString("- 严格按照 plan.md 中「拟改文件」列表工作，不要改动计划外的文件\n")
	b.WriteString("- 保持原有代码风格、import 顺序、注释风格\n")
	b.WriteString("- 不要添加多余的注释或文档\n")
	b.WriteString("- 最终必须确保编译通过\n")
	b.WriteString("- 完成后输出一行总结：改了哪些文件、编译是否通过\n")

	return b.String(), nil
}

// GenerateCodeWithAgent 使用 agent 模式生成代码。
// agent 自主读文件、改代码、跑编译，coco-ext 只做编排。
func GenerateCodeWithAgent(agent *generator.AgentGenerator, taskDir, workDir string, now time.Time, onChunk func(string), onTool func(generator.ToolEvent)) (*AgentCodeResult, error) {
	prompt, err := BuildAgentCodePrompt(taskDir, workDir)
	if err != nil {
		return nil, err
	}

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

	// 判断编译是否通过：检查 agent 是否执行了 go build 且成功
	buildOK := inferBuildResult(toolEvents, reply)

	if buildOK {
		taskMetaDir := taskDir
		_ = updateTaskStatus(taskMetaDir, TaskStatusCoded, now)
	}

	return &AgentCodeResult{
		ToolEvents: toolEvents,
		AgentReply: reply,
		BuildOK:    buildOK,
	}, nil
}

// inferBuildResult 从 agent 的工具调用和回复中推断编译是否通过。
func inferBuildResult(events []generator.ToolEvent, reply string) bool {
	// 检查是否有 bash 工具调用了 go build
	hasBuild := false
	for _, e := range events {
		if e.Kind == "bash" && strings.Contains(e.Title, "go build") {
			hasBuild = true
		}
	}
	if !hasBuild {
		return false
	}

	// 从 reply 中推断
	lower := strings.ToLower(reply)
	if strings.Contains(lower, "编译通过") || strings.Contains(lower, "build passed") ||
		strings.Contains(lower, "编译成功") || strings.Contains(lower, "successfully") {
		return true
	}

	// 没有明确失败信号也视为通过（agent 会主动报错）
	if !strings.Contains(lower, "编译失败") && !strings.Contains(lower, "build failed") &&
		!strings.Contains(lower, "build error") {
		return hasBuild
	}

	return false
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
