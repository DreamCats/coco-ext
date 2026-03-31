package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// EventsDir 本地事件目录
	EventsDir = ".livecoding/metrics"
	// EventsFile 本地事件日志文件
	EventsFile = "events.jsonl"
)

type Event struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	RepoRoot  string         `json:"repo_root"`
	Success   bool           `json:"success"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// AppendEvent 追加一条本地 metrics 事件。
func AppendEvent(repoRoot string, event Event) error {
	event.RepoRoot = repoRoot
	if event.Timestamp == "" {
		event.Timestamp = time.Now().Format(time.RFC3339)
	}

	dir := filepath.Join(repoRoot, EventsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建 metrics 目录失败: %w", err)
	}

	path := filepath.Join(dir, EventsFile)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("打开 metrics 事件文件失败: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化 metrics 事件失败: %w", err)
	}

	writer := bufio.NewWriter(file)
	if _, err := writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("写入 metrics 事件失败: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("刷新 metrics 事件失败: %w", err)
	}
	return nil
}
