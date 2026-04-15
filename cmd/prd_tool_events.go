package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/fatih/color"

	"github.com/DreamCats/coco-ext/internal/generator"
)

type todoItem struct {
	Content string `json:"Content"`
	ID      string `json:"ID"`
	Status  string `json:"Status"`
}

type todoRawInput struct {
	Todos []todoItem `json:"Todos"`
}

// renderToolEvent 渲染 agent 工具调用事件。
func renderToolEvent(event generator.ToolEvent) {
	if event.Title == "todo_write" && len(event.RawInput) > 0 {
		if event.Status == "in_progress" {
			var input todoRawInput
			if err := json.Unmarshal(event.RawInput, &input); err == nil && len(input.Todos) > 0 {
				color.Cyan("      📋 待办事项:")
				for _, item := range input.Todos {
					icon := "☐"
					if item.Status == "completed" {
						icon = "✓"
					} else if item.Status == "in_progress" {
						icon = "▶"
					}
					color.Cyan("         %s %s", icon, item.Content)
				}
			}
		}
		return
	}

	if event.Kind == "" {
		return
	}

	if event.Status == "done" {
		if event.Kind == "bash" {
			color.Cyan("")
		}
		return
	}

	switch event.Kind {
	case "edit":
		color.Cyan("      ✏️  编辑 %s", extractFileFromTitle(event.Title))
	case "bash":
		color.Cyan("      ⚡ 执行 %s", extractCmdFromTitle(event.Title))
	case "read":
		color.Cyan("      📖 读取 %s", extractFileFromTitle(event.Title))
	case "write":
		color.Cyan("      📝 写入 %s", extractFileFromTitle(event.Title))
	default:
		color.Cyan("      🔧 [%s] %s", event.Kind, event.Title)
	}
}

func extractFileFromTitle(title string) string {
	parts := strings.SplitN(title, " ", 2)
	if len(parts) == 2 && strings.HasPrefix(parts[1], "/") {
		return filepath.Base(parts[1])
	}
	return title
}

func extractCmdFromTitle(title string) string {
	if after, ok := strings.CutPrefix(title, "Run command: "); ok {
		return after
	}
	return title
}
