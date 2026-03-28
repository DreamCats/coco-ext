package generator

import (
	_ "embed"
	"fmt"
)

//go:embed prompts/glossary.md
var glossaryPrompt string

//go:embed prompts/architecture.md
var architecturePrompt string

//go:embed prompts/patterns.md
var patternsPrompt string

//go:embed prompts/gotchas.md
var gotchasPrompt string

//go:embed prompts/update.md
var updatePrompt string

// GetPrompt 根据知识文件名返回对应的 prompt
func GetPrompt(name, scanSummary string) string {
	switch name {
	case "glossary.md":
		return fmt.Sprintf(glossaryPrompt, scanSummary)
	case "architecture.md":
		return fmt.Sprintf(architecturePrompt, scanSummary)
	case "patterns.md":
		return fmt.Sprintf(patternsPrompt, scanSummary)
	case "gotchas.md":
		return fmt.Sprintf(gotchasPrompt, scanSummary)
	default:
		return ""
	}
}

// GetUpdatePrompt 生成增量更新的 prompt
func GetUpdatePrompt(name, existingContent, diffContent string) string {
	return fmt.Sprintf(updatePrompt, name, existingContent, diffContent)
}
