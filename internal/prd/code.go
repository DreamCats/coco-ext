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

// CodePatch 代表一个 search/replace 补丁块。
type CodePatch struct {
	Search  string
	Replace string
}

// CodeFile 代表 AI 生成的单个文件变更。
// Patches 非空时为 PATCH 模式（局部替换），否则为 FILE 模式（完整内容）。
type CodeFile struct {
	Path    string
	Content string      // FILE 模式：完整文件内容
	Patches []CodePatch // PATCH 模式：search/replace 块列表
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
// 结构：强指令（头）→ 数据 → 强指令（尾），首尾夹击抑制 agent 漂移。
func BuildCodePrompt(build *CodeBuild) string {
	var b strings.Builder

	// ===== 头部强指令 =====
	b.WriteString("[IMPORTANT] 你是一个纯文本代码生成器。禁止使用任何工具，禁止读取文件，禁止创建待办事项。你的唯一任务是：基于下方提供的信息，直接输出代码改动。\n\n")
	b.WriteString("输出格式（严格遵守，二选一）：\n\n")
	b.WriteString("格式一 — 局部补丁（优先使用，节省输出）：\n")
	b.WriteString("=== PATCH: path/to/file.go ===\n")
	b.WriteString("<<<<<<< SEARCH\n")
	b.WriteString("<要替换的原始代码，必须与源文件完全一致，包括缩进>\n")
	b.WriteString("======= REPLACE\n")
	b.WriteString("<替换后的新代码>\n")
	b.WriteString(">>>>>>>\n")
	b.WriteString("（同一文件可包含多个 SEARCH/REPLACE 块）\n\n")
	b.WriteString("格式二 — 完整文件（仅用于新文件或修改超过 50% 的文件）：\n")
	b.WriteString("=== FILE: path/to/file.go ===\n")
	b.WriteString("<该文件的完整源代码>\n\n")
	b.WriteString("所有文件输出完毕后以 === END === 结束。\n\n")
	b.WriteString("规则：\n")
	b.WriteString("- 第一行必须是 === PATCH: 或 === FILE:，不允许有任何前言、解释或思考过程\n")
	b.WriteString("- 已有文件的少量修改必须使用 PATCH 格式，禁止输出完整文件浪费 token\n")
	b.WriteString("- SEARCH 块中的代码必须与源文件中的内容完全一致（包括缩进和空行）\n")
	b.WriteString("- 保持原有代码风格、import 顺序、注释风格\n")
	b.WriteString("- 【关键】必须输出所有需要修改的文件，不能遗漏\n")
	b.WriteString("- 输出 === END === 后立即停止\n\n")

	// ===== 数据部分 =====
	b.WriteString("---\n\n")

	b.WriteString("## 技术方案\n\n")
	b.WriteString(truncateForPrompt(build.DesignContent, 2000))
	b.WriteString("\n\n")

	b.WriteString("## 实施计划\n\n")
	b.WriteString(truncateForPrompt(build.PlanContent, 3000))
	b.WriteString("\n\n")

	b.WriteString("## 代码模式参考\n\n")
	b.WriteString(truncateForPrompt(build.Context.Patterns, 2000))
	b.WriteString("\n\n")

	if build.Context.Gotchas != "" {
		b.WriteString("## 注意事项\n\n")
		b.WriteString(truncateForPrompt(build.Context.Gotchas, 1000))
		b.WriteString("\n\n")
	}

	b.WriteString("## 当前源文件\n\n")
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

	// ===== 尾部强指令 =====
	b.WriteString("---\n\n")
	b.WriteString("[FINAL INSTRUCTION] 现在直接输出代码改动。\n")
	b.WriteString("你必须输出以下文件（每个都需要修改）：\n")
	for i, file := range build.CandidateFiles {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, file))
	}
	b.WriteString("\n优先使用 === PATCH: === 格式（局部替换），仅新文件使用 === FILE: === 格式。\n")
	b.WriteString("第一行必须是 === PATCH: 或 === FILE:，所有文件输出完毕后以 === END === 结束。禁止输出任何其它内容。开始：\n")

	return b.String()
}

// matchCandidatePath 将 AI 输出的路径（可能是绝对路径）匹配到候选文件的相对路径。
func matchCandidatePath(outputPath string, candidates []string) string {
	// 精确匹配
	for _, c := range candidates {
		if outputPath == c {
			return c
		}
	}
	// 后缀匹配：AI 可能输出 /full/path/to/repo/dal/tcc/foo.go，候选是 dal/tcc/foo.go
	for _, c := range candidates {
		if strings.HasSuffix(outputPath, "/"+c) {
			return c
		}
	}
	return ""
}

// codePromptWithEarlyStop 发送 prompt 并在检测到 === END === 时提前返回。
func codePromptWithEarlyStop(gen *generator.Generator, prompt string, onChunk func(string)) (string, error) {
	stop := make(chan struct{}, 1)
	var buf strings.Builder
	raw, err := gen.PromptWithEarlyStop(prompt, config.CodePromptTimeout, func(chunk string) {
		buf.WriteString(chunk)
		if onChunk != nil {
			onChunk(chunk)
		}
		if strings.Contains(buf.String(), "=== END ===") {
			select {
			case stop <- struct{}{}:
			default:
			}
		}
	}, stop)
	return raw, err
}

func truncateForPrompt(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen]) + "\n\n... (已截断)"
}

// ParseCodeOutput 从 AI 输出中解析文件内容，支持 FILE（完整文件）和 PATCH（search/replace）两种格式。
func ParseCodeOutput(raw string) ([]CodeFile, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")

	// 截断到 === END ===
	if endIdx := strings.Index(normalized, "=== END ==="); endIdx != -1 {
		normalized = normalized[:endIdx]
	}

	// 逐行状态机解析
	lines := strings.Split(normalized, "\n")

	type block struct {
		path    string
		isPatch bool
		lines   []string
	}
	var blocks []block
	var cur *block
	started := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "=== FILE: ") || strings.HasPrefix(trimmed, "=== PATCH: ") {
			if cur != nil {
				blocks = append(blocks, *cur)
			}
			isPatch := strings.HasPrefix(trimmed, "=== PATCH: ")
			header := trimmed
			if isPatch {
				header = strings.TrimPrefix(header, "=== PATCH: ")
			} else {
				header = strings.TrimPrefix(header, "=== FILE: ")
			}
			header = strings.TrimSuffix(header, "===")
			header = strings.TrimSpace(header)
			cur = &block{path: header, isPatch: isPatch}
			started = true
			continue
		}

		if !started || cur == nil {
			continue
		}
		cur.lines = append(cur.lines, line)
	}
	if cur != nil {
		blocks = append(blocks, *cur)
	}

	var files []CodeFile
	for _, blk := range blocks {
		if blk.path == "" {
			continue
		}
		if blk.isPatch {
			patches := parsePatchBlocks(strings.Join(blk.lines, "\n"))
			if len(patches) > 0 {
				files = append(files, CodeFile{Path: blk.path, Patches: patches})
			}
		} else {
			content := cleanFullFileContent(strings.Join(blk.lines, "\n"))
			if content != "" {
				files = append(files, CodeFile{Path: blk.path, Content: content})
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("AI 输出中未解析到有效的文件内容")
	}
	return files, nil
}

// parsePatchBlocks 从 PATCH 段落中解析 <<<<<<< SEARCH / ======= REPLACE / >>>>>>> 块。
func parsePatchBlocks(content string) []CodePatch {
	var patches []CodePatch
	remaining := content
	for {
		searchIdx := strings.Index(remaining, "<<<<<<< SEARCH")
		if searchIdx == -1 {
			break
		}
		remaining = remaining[searchIdx+len("<<<<<<< SEARCH"):]
		if nl := strings.Index(remaining, "\n"); nl >= 0 {
			remaining = remaining[nl+1:]
		}

		replaceIdx := strings.Index(remaining, "======= REPLACE")
		if replaceIdx == -1 {
			break
		}
		searchContent := strings.TrimRight(remaining[:replaceIdx], "\n")
		remaining = remaining[replaceIdx+len("======= REPLACE"):]
		if nl := strings.Index(remaining, "\n"); nl >= 0 {
			remaining = remaining[nl+1:]
		}

		endIdx := strings.Index(remaining, ">>>>>>>")
		if endIdx == -1 {
			break
		}
		replaceContent := strings.TrimRight(remaining[:endIdx], "\n")
		remaining = remaining[endIdx+len(">>>>>>>"):]

		patches = append(patches, CodePatch{Search: searchContent, Replace: replaceContent})
	}
	return patches
}

// cleanFullFileContent 清理 FILE 模式的完整文件内容（去除 markdown 代码围栏等）。
func cleanFullFileContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if strings.HasPrefix(content, "```") {
		if idx := strings.Index(content, "\n"); idx != -1 {
			content = content[idx+1:]
		}
		if lastIdx := strings.LastIndex(content, "```"); lastIdx != -1 {
			content = content[:lastIdx]
		}
	}
	return strings.TrimRight(content, "\n") + "\n"
}

// WriteCodeFiles 将生成的文件写入磁盘。workDir 是写入根目录（主仓库或 worktree）。
// 支持 FILE 模式（完整覆写）和 PATCH 模式（search/replace 局部替换）。
func WriteCodeFiles(workDir string, files []CodeFile) []CodeFileResult {
	results := make([]CodeFileResult, 0, len(files))
	for _, file := range files {
		absPath := filepath.Join(workDir, file.Path)

		if len(file.Patches) > 0 {
			// PATCH 模式：读取已有文件，应用 search/replace
			existing, err := os.ReadFile(absPath)
			if err != nil {
				results = append(results, CodeFileResult{
					Path:  file.Path,
					Error: fmt.Sprintf("读取文件失败（PATCH 模式需要文件已存在）: %v", err),
				})
				continue
			}
			content := string(existing)
			patchFailed := false
			for i, patch := range file.Patches {
				if !strings.Contains(content, patch.Search) {
					results = append(results, CodeFileResult{
						Path:  file.Path,
						Error: fmt.Sprintf("PATCH #%d 匹配失败: 未找到要替换的代码片段", i+1),
					})
					patchFailed = true
					break
				}
				content = strings.Replace(content, patch.Search, patch.Replace, 1)
			}
			if patchFailed {
				continue
			}
			if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
				results = append(results, CodeFileResult{
					Path:  file.Path,
					Error: fmt.Sprintf("写入失败: %v", err),
				})
				continue
			}
			results = append(results, CodeFileResult{Path: file.Path, Written: true})
		} else {
			// FILE 模式：完整覆写
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
			results = append(results, CodeFileResult{Path: file.Path, Written: true})
		}
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

// codeMaxFollowUps 最多追加几轮跟进 prompt 让 agent 输出代码。
const codeMaxFollowUps = 3

// codeFollowUpPrompts 当 agent 没有直接输出代码时的跟进指令。
var codeFollowUpPrompts = []string{
	"确认，请直接输出代码。使用 === PATCH: path === 格式（局部修改）或 === FILE: path === 格式（新文件），最后 === END ===。不要解释。",
	"直接输出 === PATCH: ... === 或 === FILE: ... === 格式的代码，不要再解释。",
	"=== PATCH:",
}

// GenerateCode 是代码生成的主流程。workDir 为写入和编译的目录（主仓库或 worktree）。
func GenerateCode(gen *generator.Generator, build *CodeBuild, workDir string, now time.Time, onChunk func(string)) (*CodeResult, error) {
	prompt := BuildCodePrompt(build)

	raw, err := codePromptWithEarlyStop(gen, prompt, onChunk)
	if err != nil {
		return nil, fmt.Errorf("AI 代码生成失败: %w", err)
	}

	// 如果第一轮没有输出 FILE/PATCH 标记，自动跟进
	hasCodeMarker := strings.Contains(raw, "=== FILE:") || strings.Contains(raw, "=== PATCH:")
	for i := 0; i < codeMaxFollowUps && !hasCodeMarker; i++ {
		if onChunk != nil {
			onChunk(fmt.Sprintf("\n[coco-ext] 未检测到代码输出，发送跟进指令 (%d/%d)...\n", i+1, codeMaxFollowUps))
		}
		followUp := codeFollowUpPrompts[i]
		more, followErr := codePromptWithEarlyStop(gen, followUp, onChunk)
		if followErr != nil {
			break
		}
		raw += more
		hasCodeMarker = strings.Contains(raw, "=== FILE:") || strings.Contains(raw, "=== PATCH:")
	}

	files, err := ParseCodeOutput(raw)
	if err != nil {
		return nil, fmt.Errorf("解析 AI 输出失败: %w", err)
	}

	// 路径匹配：AI 可能输出绝对路径，需要用后缀匹配归一化为相对路径
	validFiles := make([]CodeFile, 0, len(files))
	for _, file := range files {
		relPath := matchCandidatePath(file.Path, build.CandidateFiles)
		if relPath != "" {
			validFiles = append(validFiles, CodeFile{Path: relPath, Content: file.Content, Patches: file.Patches})
		}
	}

	if len(validFiles) == 0 {
		var outputPaths []string
		for _, f := range files {
			outputPaths = append(outputPaths, f.Path)
		}
		return nil, fmt.Errorf("AI 输出的文件均不在 plan.md 的拟改文件列表中\n  AI 输出: %v\n  候选: %v", outputPaths, build.CandidateFiles)
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
