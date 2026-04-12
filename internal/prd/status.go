package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

type ArtifactStatus struct {
	Name   string
	Path   string
	Exists bool
}

type TaskStatusReport struct {
	TaskID      string
	TaskDir     string
	Metadata    TaskMetadata
	Source      *SourceMetadata
	Repos       *ReposMetadata
	Artifacts   []ArtifactStatus
	Missing     []string
	NextCommand string
	RepoNext    []string
}

var trackedArtifactNames = []string{
	"repos.json",
	"source.json",
	"prd.source.md",
	"prd-refined.md",
	"design.md",
	"plan.md",
}

var reTaskPlanComplexity = regexp.MustCompile(`(?m)^- complexity:\s*([^\s]+)\s*\((\d+)\)\s*$`)

// LoadTaskStatus 加载指定 task 的状态信息。
func LoadTaskStatus(repoRoot, taskID string) (*TaskStatusReport, error) {
	taskDir, err := findTaskDir(repoRoot, taskID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task 不存在: %s", taskID)
		}
		return nil, fmt.Errorf("查找 task 失败: %w", err)
	}

	meta, err := readTaskMetadata(filepath.Join(taskDir, "task.json"))
	if err != nil {
		return nil, err
	}

	sourceMeta, _ := readSourceMetadata(filepath.Join(taskDir, "source.json"))
	reposMeta, _ := readReposMetadata(reposMetadataPath(taskDir))
	artifacts, missing := collectArtifacts(taskDir)

	return &TaskStatusReport{
		TaskID:      taskID,
		TaskDir:     taskDir,
		Metadata:    *meta,
		Source:      sourceMeta,
		Repos:       reposMeta,
		Artifacts:   artifacts,
		Missing:     missing,
		NextCommand: suggestNextCommand(taskDir, taskID, meta.Status, artifacts, reposMeta),
		RepoNext:    buildRepoNextActions(taskID, reposMeta),
	}, nil
}

// ResolveTaskID 解析用户指定或最近的 task。
func ResolveTaskID(repoRoot, explicitTaskID string) (string, error) {
	if explicitTaskID != "" {
		return explicitTaskID, nil
	}

	taskDirs, err := listTaskDirs(repoRoot)
	if err != nil {
		return "", err
	}
	if len(taskDirs) == 0 {
		return "", fmt.Errorf("未找到任何 task，请先执行 coco-ext prd refine")
	}
	return taskDirs[0].id, nil
}

func readTaskMetadata(path string) (*TaskMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("缺少 task.json: %s", path)
		}
		return nil, fmt.Errorf("读取 task.json 失败: %w", err)
	}

	var meta TaskMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("解析 task.json 失败: %w", err)
	}
	return &meta, nil
}

func readSourceMetadata(path string) (*SourceMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta SourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func collectArtifacts(taskDir string) ([]ArtifactStatus, []string) {
	artifacts := make([]ArtifactStatus, 0, len(trackedArtifactNames))
	missing := make([]string, 0, len(trackedArtifactNames))
	for _, name := range trackedArtifactNames {
		path := filepath.Join(taskDir, name)
		_, err := os.Stat(path)
		exists := err == nil
		artifacts = append(artifacts, ArtifactStatus{
			Name:   name,
			Path:   path,
			Exists: exists,
		})
		if !exists {
			missing = append(missing, name)
		}
	}
	return artifacts, missing
}

func suggestNextCommand(taskDir, taskID, status string, artifacts []ArtifactStatus, repos *ReposMetadata) string {
	hasRefined := hasArtifact(artifacts, "prd-refined.md")
	hasDesign := hasArtifact(artifacts, "design.md")
	hasPlan := hasArtifact(artifacts, "plan.md")

	switch {
	case !hasRefined:
		return fmt.Sprintf("coco-ext prd refine --task %s --prd %s", taskID, filepath.Join(taskDir, "prd.source.md"))
	case status == TaskStatusPlanning:
		return "plan 正在执行，请稍候刷新任务详情。"
	case !hasDesign || !hasPlan:
		return fmt.Sprintf("coco-ext prd plan --task %s", taskID)
	case status == TaskStatusCoding || status == TaskStatusPartiallyCoded:
		if nextRepo := suggestNextRepo(repos); nextRepo != "" {
			return fmt.Sprintf("coco-ext prd code --task %s --repo %s", taskID, nextRepo)
		}
		return fmt.Sprintf("task 部分仓库已进入 code 阶段，请查看 `coco-ext prd status --task %s` 中的 repo 状态后继续执行 `coco-ext prd code --task %s --repo <repo_id>`", taskID, taskID)
	case status == TaskStatusPlanned:
		return fmt.Sprintf("coco-ext prd code --task %s", taskID)
	case status == TaskStatusFailed:
		if nextRepo := suggestNextRepo(repos); nextRepo != "" && hasDesign && hasPlan {
			return fmt.Sprintf("coco-ext prd code --task %s --repo %s", taskID, nextRepo)
		}
		if hasDesign && hasPlan {
			return fmt.Sprintf("coco-ext prd code --task %s", taskID)
		}
		return fmt.Sprintf("coco-ext prd plan --task %s", taskID)
	case status == TaskStatusCoded:
		return fmt.Sprintf("coco-ext prd archive --task %s", taskID)
	case status == TaskStatusArchived:
		return "task 已归档，无后续操作。"
	default:
		return "当前 task 无明确下一步，建议人工确认状态。"
	}
}

// TaskSummary 是 list 命令使用的精简 task 信息。
type TaskSummary struct {
	TaskID     string     `json:"task_id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	SourceType SourceType `json:"source_type"`
	RepoCount  int        `json:"repo_count,omitempty"`
}

// ListTasks 列出所有 task，按目录名排序（时间序）。可选按 status 过滤。
func ListTasks(repoRoot string, filterStatus string) ([]TaskSummary, error) {
	taskDirs, err := listTaskDirs(repoRoot)
	if err != nil {
		return nil, err
	}

	var tasks []TaskSummary
	for _, dir := range taskDirs {
		metaPath := filepath.Join(dir.path, "task.json")
		meta, err := readTaskMetadata(metaPath)
		if err != nil {
			continue // 跳过损坏的 task
		}
		if filterStatus != "" && meta.Status != filterStatus {
			continue
		}
		tasks = append(tasks, TaskSummary{
			TaskID:     meta.TaskID,
			Title:      meta.Title,
			Status:     meta.Status,
			CreatedAt:  meta.CreatedAt,
			UpdatedAt:  meta.UpdatedAt,
			SourceType: meta.SourceType,
			RepoCount:  maxInt(meta.RepoCount, 1),
		})
	}
	return tasks, nil
}

func hasArtifact(artifacts []ArtifactStatus, name string) bool {
	for _, artifact := range artifacts {
		if artifact.Name == name {
			return artifact.Exists
		}
	}
	return false
}

func maxInt(values ...int) int {
	max := 0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

func suggestNextRepo(repos *ReposMetadata) string {
	if repos == nil {
		return ""
	}

	for _, repo := range repos.Repos {
		switch repo.Status {
		case TaskStatusPlanned, TaskStatusInitialized, TaskStatusRefined, "", "pending", TaskStatusFailed:
			return repo.ID
		}
	}
	return ""
}

func buildRepoNextActions(taskID string, repos *ReposMetadata) []string {
	if repos == nil {
		return nil
	}

	actions := make([]string, 0, len(repos.Repos))
	for _, repo := range repos.Repos {
		switch repo.Status {
		case TaskStatusInitialized, TaskStatusRefined, TaskStatusPlanned, "", "pending", TaskStatusFailed:
			actions = append(actions, fmt.Sprintf("%s: coco-ext prd code --task %s --repo %s", repo.ID, taskID, repo.ID))
		case TaskStatusCoding:
			actions = append(actions, fmt.Sprintf("%s: 当前处于 coding，可等待完成或执行 reset -> coco-ext prd reset --task %s --repo %s", repo.ID, taskID, repo.ID))
		case TaskStatusCoded:
			actions = append(actions, fmt.Sprintf("%s: coco-ext prd archive --task %s --repo %s", repo.ID, taskID, repo.ID))
		}
	}
	return actions
}

// ReadTaskComplexity 从 task 目录下的 plan.md 提取复杂度等级与分数。
func ReadTaskComplexity(taskDir string) (level string, score int, err error) {
	data, err := os.ReadFile(filepath.Join(taskDir, "plan.md"))
	if err != nil {
		return "", 0, err
	}

	matches := reTaskPlanComplexity.FindStringSubmatch(string(data))
	if len(matches) != 3 {
		return "", 0, nil
	}

	score, err = strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, fmt.Errorf("解析复杂度分数失败: %w", err)
	}
	return matches[1], score, nil
}
