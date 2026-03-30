package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
)

// Entry Changelog 条目
type Entry struct {
	Original    string `json:"original"`
	Optimized   string `json:"optimized"`
	PushResult  string `json:"push_result"` // "success" 或 "error: xxx"
	CommitID    string `json:"commit_id"`
	NewCommitID string `json:"new_commit_id"`
	Timestamp   string `json:"timestamp"`
}

// GetChangelogPath 用 commit ID 获取 changelog 路径
func GetChangelogPath(repoRoot, branch, commitID string) string {
	branchSlug := filepath.Base(branch)
	if branchSlug == "" {
		branchSlug = "unknown"
	}
	return filepath.Join(repoRoot, config.ChangelogDir, branchSlug, commitID+".md")
}

// ReadByCommitID 用 commit ID 查找 changelog
func ReadByCommitID(repoRoot, branch, commitID string) (*Entry, error) {
	path := GetChangelogPath(repoRoot, branch, commitID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

// HasOptimizedMessageByCommitID 用 commit ID 检查是否已有成功记录
func HasOptimizedMessageByCommitID(repoRoot, branch, commitID string) bool {
	entry, err := ReadByCommitID(repoRoot, branch, commitID)
	if err != nil {
		return false
	}
	return entry.Optimized != "" && !strings.HasPrefix(entry.PushResult, "error")
}

// GetOptimizedMessageByCommitID 用 commit ID 获取已优化的 message
func GetOptimizedMessageByCommitID(repoRoot, branch, commitID string) (string, bool) {
	entry, err := ReadByCommitID(repoRoot, branch, commitID)
	if err != nil || entry.Optimized == "" {
		return "", false
	}
	// 只有 push 成功才复用
	if !strings.HasPrefix(entry.PushResult, "error") {
		return entry.Optimized, true
	}
	return "", false
}

// WriteByCommitID 用 commit ID 写入 changelog
func WriteByCommitID(repoRoot, branch, commitID string, entry *Entry) error {
	dir := filepath.Dir(GetChangelogPath(repoRoot, branch, commitID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	path := GetChangelogPath(repoRoot, branch, commitID)
	content := Format(entry)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("写入 changelog 失败: %w", err)
	}
	return nil
}

// Read 读取指定分支和 commitID 的 changelog
func Read(repoRoot, branch, commitID string) (*Entry, error) {
	return ReadByCommitID(repoRoot, branch, commitID)
}

// Write 写入 changelog
func Write(repoRoot, branch, commitID string, entry *Entry) error {
	return WriteByCommitID(repoRoot, branch, commitID, entry)
}

// Parse 解析 changelog 内容
func Parse(content string) (*Entry, error) {
	entry := &Entry{}
	lines := splitLines(content)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch line {
		case "## original":
			if i+1 < len(lines) {
				entry.Original = lines[i+1]
				i++
			}
		case "## optimized":
			if i+1 < len(lines) {
				entry.Optimized = lines[i+1]
				i++
			}
		case "## push_result":
			if i+1 < len(lines) {
				entry.PushResult = lines[i+1]
				i++
			}
		case "## commit_id":
			if i+1 < len(lines) {
				entry.CommitID = lines[i+1]
				i++
			}
		case "## new_commit_id":
			if i+1 < len(lines) {
				entry.NewCommitID = lines[i+1]
				i++
			}
		case "## timestamp":
			if i+1 < len(lines) {
				entry.Timestamp = lines[i+1]
				i++
			}
		}
	}
	return entry, nil
}

// Format 将 entry 格式化为 markdown
func Format(entry *Entry) string {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format("2006-01-02 15:04:05")
	}
	return fmt.Sprintf(`# Commit Changelog

## original
%s

## optimized
%s

## push_result
%s

## commit_id
%s

## new_commit_id
%s

## timestamp
%s
`, entry.Original, entry.Optimized, entry.PushResult, entry.CommitID, entry.NewCommitID, entry.Timestamp)
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" || len(lines) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}
