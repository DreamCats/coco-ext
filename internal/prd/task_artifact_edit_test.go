package prd

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateTaskArtifactSourceInvalidatesDownstream(t *testing.T) {
	setTestHome(t)
	taskID := "task-source-edit"
	taskDir := createTaskFixtureDir(t, taskID)
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	writeTaskFixture(t, taskDir, TaskStatusPlanned)
	writeArtifactFixture(t, taskDir, "prd.source.md", "# source\nold\n")
	writeArtifactFixture(t, taskDir, "prd-refined.md", "# refined\n")
	writeArtifactFixture(t, taskDir, "design.md", "# design\n")
	writeArtifactFixture(t, taskDir, "plan.md", "# plan\n")
	writeArtifactFixture(t, taskDir, "refine.log", "refine ok\n")
	writeArtifactFixture(t, taskDir, "plan.log", "plan ok\n")

	if err := UpdateTaskArtifact(t.TempDir(), taskID, "prd.source.md", "# source\nnew\n", now); err != nil {
		t.Fatalf("UpdateTaskArtifact() error = %v", err)
	}

	assertTaskStatus(t, taskDir, TaskStatusInitialized)
	assertFileContent(t, filepath.Join(taskDir, "prd.source.md"), "# source\nnew\n")
	assertFileMissing(t, filepath.Join(taskDir, "prd-refined.md"))
	assertFileMissing(t, filepath.Join(taskDir, "design.md"))
	assertFileMissing(t, filepath.Join(taskDir, "plan.md"))
	assertFileMissing(t, filepath.Join(taskDir, "refine.log"))
	assertFileMissing(t, filepath.Join(taskDir, "plan.log"))
}

func TestUpdateTaskArtifactRefinedResetsToRefined(t *testing.T) {
	taskID := "task-refined-edit"
	setTestHome(t)
	taskDir := createTaskFixtureDir(t, taskID)
	now := time.Date(2026, 4, 13, 11, 0, 0, 0, time.UTC)

	writeTaskFixture(t, taskDir, TaskStatusPlanned)
	writeArtifactFixture(t, taskDir, "prd-refined.md", "# refined\nold\n")
	writeArtifactFixture(t, taskDir, "design.md", "# design\n")
	writeArtifactFixture(t, taskDir, "plan.md", "# plan\n")
	writeArtifactFixture(t, taskDir, "plan.log", "plan ok\n")

	if err := UpdateTaskArtifact(t.TempDir(), taskID, "prd-refined.md", "# refined\nnew\n", now); err != nil {
		t.Fatalf("UpdateTaskArtifact() error = %v", err)
	}

	assertTaskStatus(t, taskDir, TaskStatusRefined)
	assertFileContent(t, filepath.Join(taskDir, "prd-refined.md"), "# refined\nnew\n")
	assertFileMissing(t, filepath.Join(taskDir, "design.md"))
	assertFileMissing(t, filepath.Join(taskDir, "plan.md"))
	assertFileMissing(t, filepath.Join(taskDir, "plan.log"))
}

func TestUpdateTaskArtifactPlanRequiresPlanned(t *testing.T) {
	taskID := "task-plan-edit"
	setTestHome(t)
	taskDir := createTaskFixtureDir(t, taskID)
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)

	writeTaskFixture(t, taskDir, TaskStatusRefined)
	writeArtifactFixture(t, taskDir, "plan.md", "# plan\n")

	err := UpdateTaskArtifact(t.TempDir(), taskID, "plan.md", "# plan\nchanged\n", now)
	if err == nil {
		t.Fatalf("expected status validation error")
	}
}

func setTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
}

func createTaskFixtureDir(t *testing.T, taskID string) string {
	t.Helper()
	taskDir := filepath.Join(globalTasksRoot(), taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("mkdir task fixture: %v", err)
	}
	return taskDir
}

func writeTaskFixture(t *testing.T, taskDir, status string) {
	t.Helper()
	meta := TaskMetadata{
		TaskID:      filepath.Base(taskDir),
		Title:       "task",
		Status:      status,
		CreatedAt:   time.Date(2026, 4, 13, 9, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 13, 9, 0, 0, 0, time.UTC),
		SourceType:  SourceTypeText,
		SourceValue: "source",
		RepoCount:   1,
	}
	if err := writeJSONFile(filepath.Join(taskDir, "task.json"), meta); err != nil {
		t.Fatalf("write task fixture: %v", err)
	}
	repos := ReposMetadata{
		Repos: []RepoBinding{{
			ID:     "coco-ext",
			Path:   "/tmp/coco-ext",
			Status: status,
		}},
	}
	if err := writeJSONFile(filepath.Join(taskDir, "repos.json"), repos); err != nil {
		t.Fatalf("write repos fixture: %v", err)
	}
}

func writeArtifactFixture(t *testing.T, taskDir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(taskDir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write artifact fixture %s: %v", name, err)
	}
}

func assertTaskStatus(t *testing.T, taskDir, want string) {
	t.Helper()
	meta, err := readTaskMetadata(filepath.Join(taskDir, "task.json"))
	if err != nil {
		t.Fatalf("readTaskMetadata() error = %v", err)
	}
	if meta.Status != want {
		t.Fatalf("task status = %s, want %s", meta.Status, want)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("file %s = %q, want %q", path, string(data), want)
	}
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, err=%v", path, err)
	}
}
