export type TaskStatus =
  | 'initialized'
  | 'refined'
  | 'planned'
  | 'coding'
  | 'partially_coded'
  | 'coded'
  | 'archived'
  | 'failed'

export type RepoTaskStatus = 'pending' | 'planned' | 'coding' | 'coded' | 'failed' | 'archived' | 'initialized' | 'refined'
export type SourceType = 'text' | 'file' | 'lark_doc'

export type TaskArtifactName =
  | 'prd.source.md'
  | 'prd-refined.md'
  | 'design.md'
  | 'plan.md'
  | 'code-result.json'
  | 'code.log'

export type RepoResult = {
  id: string
  displayName: string
  path: string
  status: RepoTaskStatus
  branch?: string
  worktree?: string
  commit?: string
  build?: 'passed' | 'failed' | 'n/a'
  filesWritten?: string[]
  diffSummary?: {
    repoId: string
    commit: string
    branch: string
    files: string[]
    additions: number
    deletions: number
    patch: string
  }
}

export type TaskTimelineItem = {
  label: string
  state: 'done' | 'current' | 'pending'
  detail: string
}

export type TaskListItem = {
  id: string
  title: string
  status: TaskStatus
  sourceType: SourceType
  updatedAt: string
  repoCount: number
  repoIds: string[]
}

export type TaskRecord = {
  id: string
  title: string
  status: TaskStatus
  sourceType: SourceType
  updatedAt: string
  owner: string
  complexity: string
  nextAction: string
  repoNext: string[]
  repos: RepoResult[]
  timeline: TaskTimelineItem[]
  artifacts: Record<string, string>
}

export type WorkspaceSummary = {
  repoRoot: string
  tasksRoot: string
  contextRoot: string
  worktreeRoot: string
  reposInvolved: string[]
  taskCount: number
}

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(path)
  if (!response.ok) {
    const text = await response.text()
    throw new Error(text || `Request failed: ${response.status}`)
  }
  return response.json() as Promise<T>
}

export async function listTasks() {
  const response = await fetchJSON<{ tasks: TaskListItem[] }>('/api/tasks')
  return response.tasks
}

export async function getTask(taskId: string) {
  return fetchJSON<TaskRecord>(`/api/tasks/${taskId}`)
}

export async function getWorkspace() {
  return fetchJSON<WorkspaceSummary>('/api/workspace')
}
