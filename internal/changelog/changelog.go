package changelog

import (
	"crypto/sha1"
	"encoding/hex"
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
	NewCommitID string `json:"new_commit_id"` // amend 后可能变化
	Timestamp   string `json:"timestamp"`
}

// MessageHash 计算 commit message 的 hash（用于查找已有的 changelog）
func MessageHash(msg string) string {
	hash := sha1.Sum([]byte(msg))
	return hex.EncodeToString(hash[:8]) // 只取前 8 字符，足够短
}

// GetChangelogPathByHash 用 message hash 获取 changelog 路径
func GetChangelogPathByHash(repoRoot, branch, msgHash string) string {
	branchSlug := filepath.Base(branch)
	if branchSlug == "" {
		branchSlug = "unknown"
	}
	return filepath.Join(repoRoot, config.ChangelogDir, branchSlug, msgHash+".md")
}

// ReadByMessage 用 commit message 查找 changelog
func ReadByMessage(repoRoot, branch, msg string) (*Entry, error) {
	hash := MessageHash(msg)
	path := GetChangelogPathByHash(repoRoot, branch, hash)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

// HasOptimizedMessageByMessage 用 commit message 检查是否已有优化结果
func HasOptimizedMessageByMessage(repoRoot, branch, msg string) bool {
	entry, err := ReadByMessage(repoRoot, branch, msg)
	if err != nil {
		return false
	}
	return entry.Optimized != "" && strings.HasPrefix(entry.PushResult, "error") == false
}

// GetOptimizedMessageByMessage 用 commit message 获取已优化的 message
func GetOptimizedMessageByMessage(repoRoot, branch, msg string) (string, bool) {
	entry, err := ReadByMessage(repoRoot, branch, msg)
	if err != nil || entry.Optimized == "" {
		return "", false
	}
	// 只有 push 成功才复用
	if !strings.HasPrefix(entry.PushResult, "error") {
		return entry.Optimized, true
	}
	return "", false
}

// WriteByMessageHash 用 message hash 写入 changelog
func WriteByMessageHash(repoRoot, branch, msgHash string, entry *Entry) error {
	dir := filepath.Dir(GetChangelogPathByHash(repoRoot, branch, msgHash))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	path := GetChangelogPathByHash(repoRoot, branch, msgHash)
	content := Format(entry)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("写入 changelog 失败: %w", err)
	}
	return nil
}

// Read 读取指定分支和 commitID 的 changelog（兼容旧接口）
func Read(repoRoot, branch, commitID string) (*Entry, error) {
	path := GetChangelogPathByHash(repoRoot, branch, commitID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

// Write 写入 changelog（兼容旧接口）
func Write(repoRoot, branch, commitID string, entry *Entry) error {
	return WriteByMessageHash(repoRoot, branch, commitID, entry)
}

// GetChangelogPath 获取 changelog 文件路径
func GetChangelogPath(repoRoot, branch, commitID string) string {
	return GetChangelogPathByHash(repoRoot, branch, commitID)
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
