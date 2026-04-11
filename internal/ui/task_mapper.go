package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DreamCats/coco-ext/internal/prd"
)

func readTaskArtifacts(taskDir string) map[string]string {
	artifacts := make(map[string]string, len(uiArtifactOrder))
	for _, name := range uiArtifactOrder {
		data, err := os.ReadFile(filepath.Join(taskDir, name))
		if err != nil {
			artifacts[name] = missingArtifactPlaceholder(name)
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			artifacts[name] = emptyArtifactPlaceholder(name)
			continue
		}
		artifacts[name] = string(data)
	}
	return artifacts
}

func buildTimeline(status string) []TaskTimelineItem {
	refineState, planState, codeState, archiveState := "pending", "pending", "pending", "pending"
	refineDetail, planDetail, codeDetail, archiveDetail := "等待 refine", "等待 plan", "等待 code", "等待 archive"

	switch status {
	case prd.TaskStatusInitialized:
		refineState = "current"
		refineDetail = "已初始化 task，等待生成 refined PRD"
	case prd.TaskStatusRefined:
		refineState, planState = "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "等待生成 design.md 与 plan.md"
	case prd.TaskStatusPlanned:
		refineState, planState, codeState = "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "可进入 code 阶段"
	case prd.TaskStatusCoding:
		refineState, planState, codeState = "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "至少一个 repo 正在执行 code"
	case prd.TaskStatusPartiallyCoded:
		refineState, planState, codeState = "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "部分 repo 已完成，仍有 repo 待推进"
	case prd.TaskStatusCoded:
		refineState, planState, codeState, archiveState = "done", "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "所有关联 repo 已完成 code"
		archiveDetail = "可归档收尾"
	case prd.TaskStatusArchived:
		refineState, planState, codeState, archiveState = "done", "done", "done", "done"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "已完成 code"
		archiveDetail = "已归档"
	case prd.TaskStatusFailed:
		refineState, planState, codeState = "done", "done", "current"
		refineDetail = "已生成 refined PRD"
		planDetail = "已完成 plan"
		codeDetail = "存在失败的 repo，需继续处理"
	}

	return []TaskTimelineItem{
		{Label: "Refine", State: refineState, Detail: refineDetail},
		{Label: "Plan", State: planState, Detail: planDetail},
		{Label: "Code", State: codeState, Detail: codeDetail},
		{Label: "Archive", State: archiveState, Detail: archiveDetail},
	}
}

func localOwner() string {
	if user := strings.TrimSpace(os.Getenv("USER")); user != "" {
		return user
	}
	return "local"
}

func missingArtifactPlaceholder(name string) string {
	if name == "refine.log" {
		return "当前没有可用的 refine.log。可能任务尚未启动 refine，或日志写入失败。"
	}
	return fmt.Sprintf("该 task 当前没有 `%s`。", name)
}

func emptyArtifactPlaceholder(name string) string {
	if name == "refine.log" {
		return "refine.log 当前为空。"
	}
	if name == "code.log" {
		return "当前没有可用的 code.log。可能这个 task 是旧数据，或尚未进入 code 阶段。"
	}
	return fmt.Sprintf("`%s` 当前为空。", name)
}
