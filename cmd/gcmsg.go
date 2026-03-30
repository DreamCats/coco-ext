package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/changelog"
	"github.com/DreamCats/coco-ext/internal/gcmsg"
)

var gcmsgAmend bool
var gcmsgChangelog bool
var gcmsgOriginal string
var gcmsgPush bool
var gcmsgCommitID string // hook 传入的原始 commit ID，用于 changelog key
var gcmsgStaged bool
var gcmsgCommitMsgFile string

var gcmsgCmd = &cobra.Command{
	Use:   "gcmsg",
	Short: "生成 commit message",
	Long:  "根据代码变更生成符合规范的 commit message，支持写入 commit message 文件或 --amend 覆盖上一个 commit",
	RunE:  runGcmsg,
}

func init() {
	rootCmd.AddCommand(gcmsgCmd)
	gcmsgCmd.Flags().BoolVarP(&gcmsgAmend, "amend", "", false, "自动 amend 到上一个 commit")
	gcmsgCmd.Flags().BoolVarP(&gcmsgChangelog, "changelog", "", false, "写入 changelog")
	gcmsgCmd.Flags().StringVarP(&gcmsgOriginal, "original", "", "", "原始 commit message")
	gcmsgCmd.Flags().BoolVarP(&gcmsgPush, "push", "", false, "amend 成功后执行 push")
	gcmsgCmd.Flags().StringVarP(&gcmsgCommitID, "commit-id", "", "", "原始 commit ID（用于 changelog key）")
	gcmsgCmd.Flags().BoolVarP(&gcmsgStaged, "staged", "", false, "基于暂存区 diff 生成 commit message")
	gcmsgCmd.Flags().StringVarP(&gcmsgCommitMsgFile, "commit-msg-file", "", "", "将生成的 message 写入指定的 commit message 文件")
}

func runGcmsg(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	// 获取当前分支
	branch, err := getCurrentBranch(repoRoot)
	if err != nil {
		branch = "unknown"
	}

	originalMsg, err := getOriginalCommitMessage(repoRoot)
	if err != nil {
		originalMsg = ""
	}

	color.Cyan("正在获取代码变更...")
	diff, err := getCommitDiff(repoRoot)
	if err != nil {
		return fmt.Errorf("获取 diff 失败: %w", err)
	}

	var newMsg string
	var usedCache bool

	// 用原始 commit ID 检查 changelog（由 hook 传入）
	if gcmsgChangelog && gcmsgCommitID != "" {
		if optimizedMsg, ok := changelog.GetOptimizedMessageByCommitID(repoRoot, branch, gcmsgCommitID); ok {
			color.Cyan("找到已记录的优化 message，复用...")
			newMsg = optimizedMsg
			usedCache = true
		}
	}

	// 如果没有缓存，调用 acp 生成
	if !usedCache {
		color.Cyan("正在生成 commit message...")
		newMsg, err = gcmsg.GenerateCommitMsg(repoRoot, diff)
		if err != nil {
			color.Yellow("⚠ AI 生成失败，使用本地兜底 message: %v", err)
			newMsg, err = buildFallbackCommitMsg(repoRoot)
			if err != nil {
				color.Red("生成失败: %v", err)
				return err
			}
		}
	}

	// 打印生成的消息
	fmt.Println("\n生成的 commit message:")
	color.Green("---")
	fmt.Println(newMsg)
	color.Green("---\n")

	if gcmsgCommitMsgFile != "" {
		color.Cyan("正在写入 commit message 文件...")
		if err := writeCommitMessageFile(gcmsgCommitMsgFile, newMsg); err != nil {
			return err
		}
		color.Green("✓ commit message 已写入")
		return nil
	}

	if gcmsgAmend {
		color.Cyan("正在执行 amend...")
		if err := amendCommit(repoRoot, newMsg); err != nil {
			color.Red("Amend 失败: %v", err)
			if gcmsgChangelog {
				writeChangelogError(repoRoot, branch, gcmsgCommitID, originalMsg, newMsg, err.Error())
			}
			return err
		}

		// 获取 amend 后的 commit ID
		newCommitID, err := getShortCommitID(repoRoot)
		if err != nil {
			newCommitID = "unknown"
		}

		color.Green("✓ commit message 已更新")

		// 执行 push
		if gcmsgPush {
			color.Cyan("正在执行 push...")
			if err := pushGit(repoRoot); err != nil {
				color.Red("✗ push 失败: %v", err)
				if gcmsgChangelog {
					writeChangelogError(repoRoot, branch, gcmsgCommitID, originalMsg, newMsg, "push failed: "+err.Error())
				}
				return err
			}
			color.Green("✓ push 成功")
		}

		// 写入 changelog
		if gcmsgChangelog {
			if err := writeChangelogSuccess(repoRoot, branch, gcmsgCommitID, originalMsg, newMsg, newCommitID); err != nil {
				color.Yellow("⚠ 写入 changelog 失败: %v", err)
			}
		}
	}

	return nil
}

func getCommitDiff(repoRoot string) (string, error) {
	if gcmsgStaged {
		return getStagedDiff(repoRoot)
	}
	return getCurrentCommitDiff(repoRoot)
}

// getCurrentCommitDiff 获取当前分支最新 commit 的 diff
func getCurrentCommitDiff(repoRoot string) (string, error) {
	cmd := exec.Command("git", "show", "--pretty=format:", "--binary", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func getStagedDiff(repoRoot string) (string, error) {
	args := []string{"diff", "--cached", "--binary", "--no-ext-diff"}
	if !hasHead(repoRoot) {
		args = append(args, "--root")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// amendCommit 用新 message 覆盖上一个 commit
func amendCommit(repoRoot, newMsg string) error {
	// 使用 git commit --amend
	cmd := exec.Command("git", "commit", "--amend", "-m", newMsg)
	cmd.Dir = repoRoot
	_, err := cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

// pushGit 执行 git push
func pushGit(repoRoot string) error {
	cmd := exec.Command("git", "push")
	cmd.Dir = repoRoot
	_, err := cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

// cleanCommitMsg 清理 commit message（去掉多余空白）
func cleanCommitMsg(msg string) string {
	lines := strings.Split(msg, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	return strings.Join(cleanLines, "\n")
}

func getOriginalCommitMessage(repoRoot string) (string, error) {
	if gcmsgCommitMsgFile != "" {
		return readCommitMessageFile(gcmsgCommitMsgFile)
	}
	return getCurrentCommitMessage(repoRoot)
}

// getCurrentBranch 获取当前分支名
func getCurrentBranch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = repoRoot
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
		branch = strings.TrimSpace(string(output))
	}
	return branch, nil
}

// getShortCommitID 获取当前 commit 的短 ID
func getShortCommitID(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getCurrentCommitMessage 获取当前 commit 的 message（第一行）
func getCurrentCommitMessage(repoRoot string) (string, error) {
	cmd := exec.Command("git", "log", "-1", "--pretty=%s")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func readCommitMessageFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed, nil
	}
	return "", nil
}

func writeCommitMessageFile(path, msg string) error {
	content := cleanCommitMsg(msg)
	if content == "" {
		return fmt.Errorf("commit message 为空，拒绝写入 %s", path)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0600); err != nil {
		return fmt.Errorf("写入 commit message 文件失败: %w", err)
	}
	return nil
}

type changedFile struct {
	status string
	path   string
}

func buildFallbackCommitMsg(repoRoot string) (string, error) {
	return buildFallbackCommitMsgWithMode(repoRoot, gcmsgStaged)
}

func buildFallbackCommitMsgWithMode(repoRoot string, staged bool) (string, error) {
	files, err := getChangedFilesForFallback(repoRoot, staged)
	if err != nil {
		return "", fmt.Errorf("本地兜底 message 生成失败: %w", err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("本地兜底 message 生成失败: 未检测到变更文件")
	}

	prefix := detectFallbackType(files)
	subject := buildFallbackSubject(files, prefix)
	return fmt.Sprintf("%s: %s", prefix, subject), nil
}

func getChangedFilesForFallback(repoRoot string, staged bool) ([]changedFile, error) {
	var cmd *exec.Cmd
	if staged {
		args := []string{"diff", "--cached", "--name-status", "--find-renames"}
		if !hasHead(repoRoot) {
			args = append(args, "--root")
		}
		cmd = exec.Command("git", args...)
	} else {
		cmd = exec.Command("git", "show", "--pretty=format:", "--name-status", "--find-renames", "HEAD")
	}
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	files := make([]changedFile, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		pathValue := parts[len(parts)-1]
		files = append(files, changedFile{
			status: normalizeGitStatus(parts[0]),
			path:   pathValue,
		})
	}
	return files, nil
}

func normalizeGitStatus(status string) string {
	if status == "" {
		return "M"
	}
	switch status[0] {
	case 'A':
		return "A"
	case 'D':
		return "D"
	case 'R':
		return "R"
	default:
		return "M"
	}
}

func detectFallbackType(files []changedFile) string {
	allDocs := true
	allTests := true
	allBuild := true

	for _, file := range files {
		pathValue := strings.ToLower(file.path)
		if !isDocFile(pathValue) {
			allDocs = false
		}
		if !strings.HasSuffix(pathValue, "_test.go") {
			allTests = false
		}
		if !isBuildFile(pathValue) {
			allBuild = false
		}
	}

	switch {
	case allDocs:
		return "docs"
	case allTests:
		return "test"
	case allBuild:
		return "build"
	default:
		return "chore"
	}
}

func isDocFile(pathValue string) bool {
	base := strings.ToLower(filepath.Base(pathValue))
	switch {
	case strings.HasSuffix(base, ".md"),
		strings.HasSuffix(base, ".rst"),
		strings.HasSuffix(base, ".txt"),
		base == "agents.md",
		base == "readme",
		base == "readme.md",
		strings.HasPrefix(pathValue, "docs/"):
		return true
	default:
		return false
	}
}

func isBuildFile(pathValue string) bool {
	base := filepath.Base(pathValue)
	switch base {
	case "go.mod", "go.sum", "go.mod.lock", "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "Makefile":
		return true
	default:
		return false
	}
}

func buildFallbackSubject(files []changedFile, prefix string) string {
	if len(files) == 1 {
		file := files[0]
		action := map[string]string{
			"A": "添加",
			"D": "删除",
			"R": "重命名",
			"M": "更新",
		}[file.status]
		if action == "" {
			action = "更新"
		}
		return fmt.Sprintf("%s %s", action, file.path)
	}

	switch prefix {
	case "docs":
		return fmt.Sprintf("更新 %d 个文档文件", len(files))
	case "test":
		return fmt.Sprintf("更新 %d 个测试文件", len(files))
	case "build":
		return fmt.Sprintf("更新 %d 个构建配置文件", len(files))
	default:
		return fmt.Sprintf("更新 %d 个文件", len(files))
	}
}

func hasHead(repoRoot string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "HEAD")
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

// writeChangelogSuccess 写入成功的 changelog
func writeChangelogSuccess(repoRoot, branch, commitID, original, optimized, newCommitID string) error {
	entry := &changelog.Entry{
		Original:    original,
		Optimized:   optimized,
		PushResult:  "success",
		CommitID:    commitID,
		NewCommitID: newCommitID,
	}
	return changelog.WriteByCommitID(repoRoot, branch, commitID, entry)
}

// writeChangelogError 写入失败的 changelog
func writeChangelogError(repoRoot, branch, commitID, original, optimized, errMsg string) error {
	entry := &changelog.Entry{
		Original:   original,
		Optimized:  optimized,
		PushResult: "error: " + errMsg,
		CommitID:   commitID,
	}
	return changelog.WriteByCommitID(repoRoot, branch, commitID, entry)
}
