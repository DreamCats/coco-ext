package generator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DreamCats/coco-acp-sdk/acp"
	"github.com/DreamCats/coco-ext/internal/config"
)

// ToolEvent 代表 agent 执行的一次工具调用事件。
type ToolEvent struct {
	Kind   string // "read", "edit", "bash", "write", ...
	Title  string // 人类可读描述
	Status string // "in_progress", "done"
}

// AgentGenerator 使用 yolo 模式的完整 agent，让 agent 自主读写文件和编译。
// 与 RawGenerator 的区别：不禁用工具，让 agent 拥有完整的 Read/Edit/Write/Bash 能力。
type AgentGenerator struct {
	client  *acp.Client
	modelID string
}

// NewAgent 创建 yolo 模式的 AgentGenerator。
// agent 拥有完整工具能力，可自主读写文件、执行命令。
func NewAgent(repoPath string) (*AgentGenerator, error) {
	client := acp.NewClient(repoPath,
		acp.WithServeFlags(&acp.ServeFlags{Yolo: true}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("启动 agent 失败: %w", err)
	}

	return &AgentGenerator{
		client:  client,
		modelID: config.DefaultModel,
	}, nil
}

// PromptWithTools 发送 prompt，agent 自主使用工具完成任务。
// onChunk 接收文本输出，onTool 接收工具调用事件（可为 nil）。
func (g *AgentGenerator) PromptWithTools(prompt string, timeout time.Duration, onChunk func(string), onTool func(ToolEvent)) (string, error) {
	if g.client == nil {
		return "", fmt.Errorf("agent client 已关闭")
	}

	var result strings.Builder

	g.client.SetNotifyHandler(func(method string, update *acp.SessionUpdate) {
		if update == nil {
			return
		}
		switch update.SessionUpdate {
		case acp.UpdateAgentMessageChunk:
			if update.Content != nil {
				result.WriteString(update.Content.Text)
				if onChunk != nil {
					onChunk(update.Content.Text)
				}
			}
		case acp.UpdateToolCall:
			if onTool != nil {
				onTool(ToolEvent{
					Kind:   update.Kind,
					Title:  update.Title,
					Status: update.Status,
				})
			}
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := g.client.Prompt(ctx, prompt, g.modelID)
	if err != nil {
		return result.String(), err
	}
	return result.String(), nil
}

// Close 关闭 agent client 并终止子进程。
func (g *AgentGenerator) Close() {
	if g.client != nil {
		g.client.Close()
		g.client = nil
	}
}
