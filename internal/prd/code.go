package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
)

// CodeBuild 包含代码生成所需的所有输入信息。
type CodeBuild struct {
	RepoRoot       string
	TaskID         string
	Task           *TaskStatusReport
	Context        *ContextSnapshot
	PlanContent    string
	DesignContent  string
	CandidateFiles []string
	FileSources    map[string]string // path -> current content
	PlanIdents     []string          // 从 plan.md 提取的 Go 标识符
}

// CodeFile 代表 AI 生成的单个文件内容。
type CodeFile struct {
	Path    string
	Content string
}

// CodeFileResult 代表单个文件的写入结果。
type CodeFileResult struct {
	Path    string `json:"path"`
	Written bool   `json:"written"`
	Error   string `json:"error,omitempty"`
}

// CodeResult 代表整个代码生成的结果。
type CodeResult struct {
	Branch      string
	TaskID      string
	Files       []CodeFileResult
	CommitHash  string
	BuildOK     bool
	BuildOutput string
}

// CodeResultReport 是写入 code-result.json 的结构，供 LLM 消费。
type CodeResultReport struct {
	Status       string   `json:"status"`
	TaskID       string   `json:"task_id"`
	Branch       string   `json:"branch"`
	Worktree     string   `json:"worktree"`
	Commit       string   `json:"commit,omitempty"`
	BuildOK      bool     `json:"build_ok"`
	FilesWritten []string `json:"files_written,omitempty"`
	Error        string   `json:"error,omitempty"`
	Log          string   `json:"log"`
	StartedAt    string   `json:"started_at"`
	FinishedAt   string   `json:"finished_at,omitempty"`
}

// WriteCodeResultReport 将结果写入 task 目录的 code-result.json。
func WriteCodeResultReport(taskDir string, report CodeResultReport) error {
	return writeJSONFile(filepath.Join(taskDir, "code-result.json"), report)
}

// ReadCodeResultReport 读取 code-result.json。
func ReadCodeResultReport(taskDir string) (*CodeResultReport, error) {
	data, err := os.ReadFile(filepath.Join(taskDir, "code-result.json"))
	if err != nil {
		return nil, err
	}
	var report CodeResultReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// PrepareCodeBuild 校验 task 状态，读取 plan/design/context/源文件，返回 CodeBuild。
func PrepareCodeBuild(repoRoot, taskID string) (*CodeBuild, error) {
	task, err := LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	if task.Metadata.Status != TaskStatusPlanned && task.Metadata.Status != TaskStatusCoded {
		return nil, fmt.Errorf("task 状态为 %s，需要先执行 coco-ext prd plan 使其达到 planned 状态", task.Metadata.Status)
	}

	planPath := filepath.Join(task.TaskDir, "plan.md")
	planContent, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("读取 plan.md 失败: %w", err)
	}

	if strings.Contains(string(planContent), "complexity: 复杂") {
		return nil, fmt.Errorf("当前需求复杂度为「复杂」，不支持自动代码生成，建议人工实现")
	}

	designPath := filepath.Join(task.TaskDir, "design.md")
	designContent, err := os.ReadFile(designPath)
	if err != nil {
		return nil, fmt.Errorf("读取 design.md 失败: %w", err)
	}

	context, err := LoadContextSnapshot(repoRoot)
	if err != nil {
		return nil, err
	}

	candidateFiles := extractCandidateFilesFromPlan(string(planContent))
	if len(candidateFiles) == 0 {
		return nil, fmt.Errorf("plan.md 中未找到拟改文件列表")
	}

	planIdents := ExtractIdentifiersFromPlan(string(planContent))

	fileSources := make(map[string]string, len(candidateFiles))
	for _, file := range candidateFiles {
		absPath := filepath.Join(repoRoot, file)
		data, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				fileSources[file] = ""
				continue
			}
			return nil, fmt.Errorf("读取源文件 %s 失败: %w", file, err)
		}
		fileSources[file] = string(data)
	}

	return &CodeBuild{
		RepoRoot:       repoRoot,
		TaskID:         taskID,
		Task:           task,
		Context:        context,
		PlanContent:    string(planContent),
		DesignContent:  string(designContent),
		CandidateFiles: candidateFiles,
		FileSources:    fileSources,
		PlanIdents:     planIdents,
	}, nil
}

// extractCandidateFilesFromPlan 从 plan.md 的「拟改文件」段落提取文件路径。
func extractCandidateFilesFromPlan(planContent string) []string {
	lines := strings.Split(planContent, "\n")
	inSection := false
	var files []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## 拟改文件" {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if !inSection || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		entry := strings.TrimPrefix(trimmed, "- ")
		parts := strings.SplitN(entry, "：", 2)
		if len(parts) < 2 {
			parts = strings.SplitN(entry, ":", 2)
		}
		filePath := strings.TrimSpace(parts[0])
		if filePath != "" && (strings.Contains(filePath, ".") || strings.Contains(filePath, "/")) {
			files = append(files, filePath)
		}
	}
	return files
}

// BuildCodePrompt 构建代码生成的 AI prompt。
func BuildCodePrompt(build *CodeBuild) string {
	var b strings.Builder
	b.WriteString("你是一名资深 Go 开发工程师。基于提供的技术方案（design.md）、实施计划（plan.md）、代码模式（patterns）和当前源文件，直接输出修改后的完整文件内容。\n\n")

	b.WriteString("要求：\n")
	b.WriteString("1. 严格按照 plan.md 中的任务列表和具体动作进行修改，不要做额外的改动。\n")
	b.WriteString("2. 输出每个需要修改的文件的完整内容（不是 diff，是完整文件）。\n")
	b.WriteString("3. 保持原有代码风格、import 顺序、注释风格不变。\n")
	b.WriteString("4. 如果某个文件不需要修改，不要输出该文件。\n")
	b.WriteString("5. 不要输出任何解释、思考过程或前言，直接输出文件内容。\n")
	b.WriteString("6. 使用以下标记格式分隔每个文件：\n\n")
	b.WriteString("=== FILE: path/to/file.go ===\n")
	b.WriteString("<完整文件内容>\n")
	b.WriteString("=== FILE: path/to/another.go ===\n")
	b.WriteString("<完整文件内容>\n")
	b.WriteString("=== END ===\n\n")

	b.WriteString("## 技术方案（design.md 摘要）\n\n")
	b.WriteString(truncateForPrompt(build.DesignContent, 2000))
	b.WriteString("\n\n")

	b.WriteString("## 实施计划（plan.md）\n\n")
	b.WriteString(truncateForPrompt(build.PlanContent, 3000))
	b.WriteString("\n\n")

	b.WriteString("## 代码模式参考（patterns.md）\n\n")
	b.WriteString(truncateForPrompt(build.Context.Patterns, 2000))
	b.WriteString("\n\n")

	if build.Context.Gotchas != "" {
		b.WriteString("## 注意事项（gotchas.md）\n\n")
		b.WriteString(truncateForPrompt(build.Context.Gotchas, 1000))
		b.WriteString("\n\n")
	}

	b.WriteString("## 当前源文件（仅展示与 plan 相关的函数/类型，省略部分用注释标注行数）\n\n")
	for _, file := range build.CandidateFiles {
		content := build.FileSources[file]
		b.WriteString(fmt.Sprintf("### %s\n\n", file))
		if content == "" {
			b.WriteString("（新文件，当前不存在）\n\n")
		} else {
			extracted := ExtractRelevantSource(file, content, build.PlanIdents)
			b.WriteString("```go\n")
			b.WriteString(extracted)
			if !strings.HasSuffix(extracted, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n\n")
		}
	}

	return b.String()
}

func truncateForPrompt(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen]) + "\n\n... (已截断)"
}

// ParseCodeOutput 从 AI 输出中解析文件内容。
func ParseCodeOutput(raw string) ([]CodeFile, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")

	const fileMarker = "=== FILE: "
	const endMarker = "=== END ==="

	idx := strings.Index(normalized, fileMarker)
	if idx == -1 {
		return nil, fmt.Errorf("AI 输出中未找到文件标记 '=== FILE: ...'")
	}
	normalized = normalized[idx:]

	if endIdx := strings.Index(normalized, endMarker); endIdx != -1 {
		normalized = normalized[:endIdx]
	}

	parts := strings.Split(normalized, fileMarker)
	var files []CodeFile
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		lines := strings.SplitN(part, "\n", 2)
		if len(lines) < 2 {
			continue
		}

		header := strings.TrimSpace(lines[0])
		header = strings.TrimSuffix(header, "===")
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		content := lines[1]
		content = strings.TrimSpace(content)
		if strings.HasPrefix(content, "```") {
			if idx := strings.Index(content, "\n"); idx != -1 {
				content = content[idx+1:]
			}
			if lastIdx := strings.LastIndex(content, "```"); lastIdx != -1 {
				content = content[:lastIdx]
			}
		}

		content = strings.TrimRight(content, "\n") + "\n"

		files = append(files, CodeFile{
			Path:    header,
			Content: content,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("AI 输出中未解析到有效的文件内容")
	}

	return files, nil
}

// WriteCodeFiles 将生成的文件写入磁盘。workDir 是写入根目录（主仓库或 worktree）。
func WriteCodeFiles(workDir string, files []CodeFile) []CodeFileResult {
	results := make([]CodeFileResult, 0, len(files))
	for _, file := range files {
		absPath := filepath.Join(workDir, file.Path)

		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			results = append(results, CodeFileResult{
				Path:  file.Path,
				Error: fmt.Sprintf("创建目录失败: %v", err),
			})
			continue
		}

		if err := os.WriteFile(absPath, []byte(file.Content), 0644); err != nil {
			results = append(results, CodeFileResult{
				Path:  file.Path,
				Error: fmt.Sprintf("写入失败: %v", err),
			})
			continue
		}

		results = append(results, CodeFileResult{
			Path:    file.Path,
			Written: true,
		})
	}
	return results
}

// CheckGoBuild 对改动涉及的 package 执行编译检查。workDir 是编译目录。
func CheckGoBuild(workDir string, files []CodeFile) (bool, string) {
	pkgs := make(map[string]bool)
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		pkgs["./"+dir+"/..."] = true
	}

	var allOutput strings.Builder
	allOK := true
	for pkg := range pkgs {
		cmd := exec.Command("go", "build", pkg)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			allOK = false
			allOutput.WriteString(fmt.Sprintf("go build %s 失败:\n%s\n", pkg, string(output)))
		}
	}

	return allOK, allOutput.String()
}

// WarmupDaemon 发送一个极简 prompt 验证 daemon 连通性。
func WarmupDaemon(gen *generator.Generator) error {
	_, err := gen.PromptWithTimeout("回复 OK", 30*time.Second, nil)
	if err != nil {
		return fmt.Errorf("daemon 连通性检查失败: %w", err)
	}
	return nil
}

// GenerateCode 是代码生成的主流程。workDir 为写入和编译的目录（主仓库或 worktree）。
func GenerateCode(gen *generator.Generator, build *CodeBuild, workDir string, now time.Time, onChunk func(string)) (*CodeResult, error) {
	prompt := BuildCodePrompt(build)
	raw, err := gen.PromptWithTimeout(prompt, config.CodePromptTimeout, onChunk)
	if err != nil {
		return nil, fmt.Errorf("AI 代码生成失败: %w", err)
	}

	files, err := ParseCodeOutput(raw)
	if err != nil {
		return nil, fmt.Errorf("解析 AI 输出失败: %w", err)
	}

	candidateSet := make(map[string]bool, len(build.CandidateFiles))
	for _, f := range build.CandidateFiles {
		candidateSet[f] = true
	}
	validFiles := make([]CodeFile, 0, len(files))
	for _, file := range files {
		if candidateSet[file.Path] {
			validFiles = append(validFiles, file)
		}
	}

	if len(validFiles) == 0 {
		return nil, fmt.Errorf("AI 输出的文件均不在 plan.md 的拟改文件列表中")
	}

	writeResults := WriteCodeFiles(workDir, validFiles)

	buildOK, buildOutput := CheckGoBuild(workDir, validFiles)

	if buildOK {
		_ = updateTaskStatus(build.Task.TaskDir, TaskStatusCoded, now)
	}

	return &CodeResult{
		TaskID:      build.TaskID,
		Files:       writeResults,
		BuildOK:     buildOK,
		BuildOutput: buildOutput,
	}, nil
}
