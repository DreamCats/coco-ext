package prd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type editableArtifactRule struct {
	allowedStatuses map[string]bool
	nextStatus      string
	invalidate      []string
}

var editableArtifactRules = map[string]editableArtifactRule{
	"prd.source.md": {
		allowedStatuses: allowedTaskStatuses(TaskStatusInitialized, TaskStatusRefined, TaskStatusPlanned),
		nextStatus:      TaskStatusInitialized,
		invalidate:      []string{"prd-refined.md", "design.md", "plan.md", "refine.log", "plan.log"},
	},
	"prd-refined.md": {
		allowedStatuses: allowedTaskStatuses(TaskStatusRefined, TaskStatusPlanned),
		nextStatus:      TaskStatusRefined,
		invalidate:      []string{"design.md", "plan.md", "plan.log"},
	},
	"design.md": {
		allowedStatuses: allowedTaskStatuses(TaskStatusPlanned),
		nextStatus:      TaskStatusPlanned,
	},
	"plan.md": {
		allowedStatuses: allowedTaskStatuses(TaskStatusPlanned),
		nextStatus:      TaskStatusPlanned,
	},
}

func allowedTaskStatuses(statuses ...string) map[string]bool {
	allowed := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		allowed[status] = true
	}
	return allowed
}

// UpdateTaskArtifact 覆盖 task 级 Markdown 产物，并根据编辑的文档回退 task 状态。
func UpdateTaskArtifact(repoRoot, taskID, name, content string, now time.Time) error {
	rule, ok := editableArtifactRules[name]
	if !ok {
		return fmt.Errorf("artifact %s 不支持编辑", name)
	}

	report, err := LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return err
	}

	if !rule.allowedStatuses[report.Metadata.Status] {
		return fmt.Errorf("当前状态为 %s，不能编辑 %s", report.Metadata.Status, name)
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return fmt.Errorf("%s 不能为空", name)
	}

	targetPath := filepath.Join(report.TaskDir, name)
	if err := os.WriteFile(targetPath, []byte(trimmed+"\n"), 0644); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", name, err)
	}

	for _, staleName := range rule.invalidate {
		if err := removeTaskArtifact(report.TaskDir, staleName); err != nil {
			return err
		}
	}

	if err := updateTaskStatus(report.TaskDir, rule.nextStatus, now); err != nil {
		return err
	}
	return nil
}

func removeTaskArtifact(taskDir, name string) error {
	path := filepath.Join(taskDir, name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除过期产物 %s 失败: %w", name, err)
	}
	return nil
}
