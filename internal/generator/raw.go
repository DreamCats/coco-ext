package generator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/DreamCats/coco-acp-sdk/acp"
	"github.com/DreamCats/coco-ext/internal/config"
)

// disallowedTools 是 prd code 场景下需要禁用的内置工具列表。
// 禁用后 agent 被迫以纯文本方式输出代码，不再调用工具导致卡死。
var disallowedTools = []string{
	"Read", "Edit", "Write", "Replace",
	"Bash", "Search", "Glob", "Grep",
	"Todo", "TodoRead",
}

// RawGenerator 直接管理 coco acp serve 子进程，绕过 daemon。
// 通过 --disallowed-tool 禁用所有内置工具，使 agent 变为纯 LLM 文本输出模式。
type RawGenerator struct {
	client  *acp.Client
	modelID string
}

// NewRaw 创建一个禁用所有工具的 RawGenerator。
// 适用于 prd code 等需要结构化文本输出的场景。
func NewRaw(repoPath string) (*RawGenerator, error) {
	client := acp.NewClient(repoPath,
		acp.WithCommandFactory(func(ctx context.Context) *exec.Cmd {
			args := []string{"acp", "serve"}
			for _, tool := range disallowedTools {
				args = append(args, "--disallowed-tool", tool)
			}
			return exec.CommandContext(ctx, "coco", args...)
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("启动 raw ACP client 失败: %w", err)
	}

	return &RawGenerator{
		client:  client,
		modelID: config.DefaultModel,
	}, nil
}

// PromptWithTimeout 发送 prompt 并流式返回结果。
func (g *RawGenerator) PromptWithTimeout(prompt string, timeout time.Duration, onChunk func(string)) (string, error) {
	if g.client == nil {
		return "", fmt.Errorf("raw client 已关闭")
	}

	var result strings.Builder

	g.client.SetNotifyHandler(func(method string, update *acp.SessionUpdate) {
		if update != nil && update.SessionUpdate == "agent_message_chunk" && update.Content != nil {
			result.WriteString(update.Content.Text)
			if onChunk != nil {
				onChunk(update.Content.Text)
			}
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := g.client.Prompt(ctx, prompt, g.modelID)
	if err != nil {
		// 超时或进程退出时返回已累积的内容
		return result.String(), err
	}
	return result.String(), nil
}

// Close 关闭 raw client 并终止子进程。
func (g *RawGenerator) Close() {
	if g.client != nil {
		g.client.Close()
		g.client = nil
	}
}
