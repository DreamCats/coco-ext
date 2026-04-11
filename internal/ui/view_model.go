package ui

type TaskListItem struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	SourceType string   `json:"sourceType"`
	UpdatedAt  string   `json:"updatedAt"`
	RepoCount  int      `json:"repoCount"`
	RepoIDs    []string `json:"repoIds"`
}

type RepoView struct {
	ID           string    `json:"id"`
	DisplayName  string    `json:"displayName"`
	Path         string    `json:"path"`
	Status       string    `json:"status"`
	Branch       string    `json:"branch,omitempty"`
	Worktree     string    `json:"worktree,omitempty"`
	Commit       string    `json:"commit,omitempty"`
	Build        string    `json:"build,omitempty"`
	FilesWritten []string  `json:"filesWritten,omitempty"`
	DiffSummary  *DiffView `json:"diffSummary,omitempty"`
}

type DiffView struct {
	RepoID    string   `json:"repoId"`
	Commit    string   `json:"commit"`
	Branch    string   `json:"branch"`
	Files     []string `json:"files"`
	Additions int      `json:"additions"`
	Deletions int      `json:"deletions"`
	Patch     string   `json:"patch"`
}

type TaskTimelineItem struct {
	Label  string `json:"label"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

type TaskDetail struct {
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Status     string             `json:"status"`
	SourceType string             `json:"sourceType"`
	UpdatedAt  string             `json:"updatedAt"`
	Owner      string             `json:"owner"`
	Complexity string             `json:"complexity"`
	NextAction string             `json:"nextAction"`
	RepoNext   []string           `json:"repoNext"`
	Repos      []RepoView         `json:"repos"`
	Timeline   []TaskTimelineItem `json:"timeline"`
	Artifacts  map[string]string  `json:"artifacts"`
}

type WorkspaceSummary struct {
	RepoRoot      string   `json:"repoRoot"`
	TasksRoot     string   `json:"tasksRoot"`
	ContextRoot   string   `json:"contextRoot"`
	WorktreeRoot  string   `json:"worktreeRoot"`
	ReposInvolved []string `json:"reposInvolved"`
	TaskCount     int      `json:"taskCount"`
}

type RepoCandidate struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Path        string `json:"path"`
	TaskCount   int    `json:"taskCount,omitempty"`
	LastSeenAt  string `json:"lastSeenAt,omitempty"`
}

type RemoteRoot struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type RemoteDirEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsGitRepo bool   `json:"isGitRepo"`
}

var uiArtifactOrder = []string{
	"prd.source.md",
	"prd-refined.md",
	"design.md",
	"plan.md",
	"code-result.json",
	"code.log",
}
