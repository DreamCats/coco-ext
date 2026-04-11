import { Link, Navigate, Outlet, useLocation, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import {
  createTask,
  deleteTask,
  getTask,
  type RepoCandidate,
  type TaskArtifactName,
  type TaskListItem,
  type TaskRecord,
  type TaskStatus,
} from '../api'
import { ActionPanel } from '../components/action-panel'
import { ArtifactViewer, artifactLabel } from '../components/artifact-viewer'
import { DiffPanel } from '../components/diff-panel'
import { RepoPicker } from '../components/repo-picker'
import {
  CompactField,
  FilterChip,
  KeyValue,
  PanelMessage,
  RepoStatusBadge,
  StatusBadge,
  TimelineCard,
} from '../components/ui-primitives'
import { useAppData } from '../hooks/use-app-data'

export function TasksLayout() {
  const { tasks, loading, error, reload } = useAppData()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | TaskStatus>('all')
  const [repoFilter, setRepoFilter] = useState('all')
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [createInput, setCreateInput] = useState('')
  const [createTitle, setCreateTitle] = useState('')
  const [selectedRepos, setSelectedRepos] = useState<RepoCandidate[]>([])

  const repoOptions = useMemo(() => {
    const set = new Set<string>()
    for (const task of tasks) {
      for (const repoId of task.repoIds) {
        set.add(repoId)
      }
    }
    return Array.from(set).sort()
  }, [tasks])

  const filteredTasks = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    return tasks.filter((task) => {
      if (statusFilter !== 'all' && task.status !== statusFilter) {
        return false
      }
      if (repoFilter !== 'all' && !task.repoIds.includes(repoFilter)) {
        return false
      }
      if (!keyword) {
        return true
      }
      return (
        task.title.toLowerCase().includes(keyword) ||
        task.id.toLowerCase().includes(keyword) ||
        task.repoIds.some((repoId) => repoId.toLowerCase().includes(keyword))
      )
    })
  }, [tasks, query, statusFilter, repoFilter])

  useEffect(() => {
    if (!showCreateForm) {
      return
    }
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setShowCreateForm(false)
        setCreateError('')
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => {
      document.body.style.overflow = previousOverflow
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [showCreateForm])

  async function submitCreateTask() {
    try {
      setCreating(true)
      setCreateError('')
      const result = await createTask({
        input: createInput.trim(),
        title: createTitle.trim() || undefined,
        repos: selectedRepos.map((repo) => repo.path),
      })
      await reload()
      setShowCreateForm(false)
      setCreateInput('')
      setCreateTitle('')
      setSelectedRepos([])
      void navigate({ to: '/tasks/$taskId', params: { taskId: result.task_id } })
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : '创建 task 失败')
    } finally {
      setCreating(false)
    }
  }

  function closeCreateForm() {
    setShowCreateForm(false)
    setCreateError('')
    setSelectedRepos([])
  }

  return (
    <>
      <div className="grid gap-4 lg:h-[calc(100vh-235px)] lg:min-h-0 lg:grid-cols-[360px_minmax(0,1fr)]">
      <section
        className="rounded-[24px] border border-stone-200 bg-stone-50/80 p-4 dark:border-white/10 dark:bg-white/5 lg:flex lg:min-h-0 lg:flex-col lg:overflow-hidden"
      >
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">Delivery Console</div>
            <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-stone-950 dark:text-stone-50">任务流与交付队列</h2>
            <p className="mt-2 max-w-[280px] text-sm leading-6 text-stone-500 dark:text-stone-400">
              从这里看当前有哪些需求在流转、哪些已经进入多仓 code、哪些还在等待下一步。
            </p>
          </div>
          <div className="flex flex-col gap-2">
            <button
              className="rounded-2xl bg-stone-900 px-4 py-3 text-sm font-semibold text-white transition hover:bg-stone-800"
              onClick={() => setShowCreateForm((current) => !current)}
              type="button"
            >
              {showCreateForm ? '收起 New Task' : 'New Task'}
            </button>
            <div className="rounded-2xl bg-stone-900 px-3 py-2 text-right text-white dark:bg-stone-100 dark:text-stone-950">
              <div className="text-[11px] uppercase tracking-[0.24em] text-stone-400 dark:text-stone-500">Latest</div>
              <div className="text-sm font-semibold">{tasks[0]?.updatedAt ?? '-'}</div>
            </div>
          </div>
        </div>

        <div className="mb-4 grid grid-cols-3 gap-2 text-xs">
          <FilterChip label="All" value={loading ? '...' : `${tasks.length}`} />
          <FilterChip
            label="Coded"
            value={loading ? '...' : `${tasks.filter((task) => task.status === 'coded' || task.status === 'partially_coded').length}`}
          />
          <FilterChip
            label="Planned"
            value={loading ? '...' : `${tasks.filter((task) => task.status === 'planned').length}`}
          />
        </div>

        <div className="mb-4 space-y-3">
          <input
            className="w-full rounded-2xl border border-stone-200 bg-white px-3 py-3 text-sm text-stone-900 outline-none transition placeholder:text-stone-400 focus:border-stone-400 dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-100 dark:placeholder:text-stone-500 dark:focus:border-white/20"
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索 task / repo / task_id"
            type="text"
            value={query}
          />
          <div className="grid grid-cols-2 gap-2">
            <select
              className="rounded-2xl border border-stone-200 bg-white px-3 py-3 text-sm text-stone-700 outline-none focus:border-stone-400 dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-200 dark:focus:border-white/20"
              onChange={(event) => setStatusFilter(event.target.value as 'all' | TaskStatus)}
              value={statusFilter}
            >
              <option value="all">全部状态</option>
              <option value="planned">planned</option>
              <option value="coding">coding</option>
              <option value="partially_coded">partially_coded</option>
              <option value="coded">coded</option>
              <option value="archived">archived</option>
              <option value="failed">failed</option>
            </select>
            <select
              className="rounded-2xl border border-stone-200 bg-white px-3 py-3 text-sm text-stone-700 outline-none focus:border-stone-400 dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-200 dark:focus:border-white/20"
              onChange={(event) => setRepoFilter(event.target.value)}
              value={repoFilter}
            >
              <option value="all">全部仓库</option>
              {repoOptions.map((repoId) => (
                <option key={repoId} value={repoId}>
                  {repoId}
                </option>
              ))}
            </select>
          </div>
        </div>

        {loading ? (
          <PanelMessage>正在加载 task 列表...</PanelMessage>
        ) : error ? (
          <PanelMessage>{error}</PanelMessage>
        ) : filteredTasks.length === 0 ? (
          <PanelMessage>当前过滤条件下没有 task。</PanelMessage>
        ) : tasks.length === 0 ? (
          <PanelMessage>当前没有 task。</PanelMessage>
        ) : (
          <div className="space-y-3 lg:min-h-0 lg:flex-1 lg:overflow-y-auto lg:pr-1">
            {filteredTasks.map((task) => (
              <TaskListItemCard key={task.id} task={task} />
            ))}
          </div>
        )}
      </section>

      <div className="min-h-0 overflow-hidden lg:min-h-0">
        <Outlet />
      </div>
      </div>

      {showCreateForm ? (
        <div
          className="absolute inset-4 z-50 flex items-end rounded-[28px] bg-stone-950/30 backdrop-blur-sm transition dark:bg-black/55 sm:items-center sm:justify-center lg:inset-5"
          onClick={closeCreateForm}
        >
          <div
            className="mx-4 my-4 flex max-h-[calc(100%-32px)] w-full flex-col overflow-hidden rounded-[28px] border border-stone-200 bg-white shadow-[0_30px_80px_rgba(15,23,42,0.18)] transition duration-200 ease-out dark:border-white/10 dark:bg-[#14171c] dark:shadow-[0_30px_80px_rgba(0,0,0,0.35)] sm:mx-6 sm:my-6 sm:max-w-[900px] sm:max-h-[calc(100%-48px)]"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="flex items-start justify-between gap-4 border-b border-stone-200 px-5 py-5 dark:border-white/10 md:px-6">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">Create Task</div>
                <h3 className="mt-2 text-[30px] font-semibold tracking-[-0.05em] text-stone-950 dark:text-stone-50">新建需求任务</h3>
                <p className="mt-2 text-sm leading-6 text-stone-500 dark:text-stone-400">
                  这里创建的是全局 task。先明确需求输入和 repo scope，再由后台异步执行 refine。
                </p>
              </div>
              <button
                className="rounded-full border border-stone-200 px-3 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-stone-500 transition hover:border-stone-300 hover:text-stone-900 dark:border-white/10 dark:text-stone-400 dark:hover:border-white/20 dark:hover:text-stone-100"
                onClick={closeCreateForm}
                type="button"
              >
                Close
              </button>
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5 md:px-6">
              <div className="space-y-4">
              <textarea
                className="min-h-32 w-full rounded-[22px] border border-stone-200 px-4 py-4 text-sm text-stone-900 outline-none focus:border-stone-400 dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-100 dark:placeholder:text-stone-500 dark:focus:border-white/20"
                onChange={(event) => setCreateInput(event.target.value)}
                placeholder="输入需求描述、PRD 文本或飞书链接"
                value={createInput}
              />
              <input
                className="w-full rounded-[22px] border border-stone-200 px-4 py-4 text-sm text-stone-900 outline-none focus:border-stone-400 dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-100 dark:placeholder:text-stone-500 dark:focus:border-white/20"
                onChange={(event) => setCreateTitle(event.target.value)}
                placeholder="可选标题"
                type="text"
                value={createTitle}
              />
              <RepoPicker onChange={setSelectedRepos} selectedRepos={selectedRepos} />
              {createError ? <div className="text-sm text-rose-600">{createError}</div> : null}
              </div>
            </div>
            <div className="border-t border-stone-200 px-5 py-4 dark:border-white/10 md:px-6">
              <div className="flex flex-wrap gap-2">
                <button
                  className="rounded-2xl bg-stone-900 px-5 py-3 text-sm font-semibold text-white transition hover:bg-stone-800 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={creating || !createInput.trim() || selectedRepos.length === 0}
                  onClick={() => void submitCreateTask()}
                  type="button"
                >
                  {creating ? 'Creating...' : 'Create'}
                </button>
                <button
                  className="rounded-2xl border border-stone-200 px-5 py-3 text-sm text-stone-700 transition hover:border-stone-300 hover:bg-stone-50 dark:border-white/10 dark:text-stone-300 dark:hover:border-white/20 dark:hover:bg-white/10"
                  onClick={closeCreateForm}
                  type="button"
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </>
  )
}

export function TasksIndexPage() {
  const { tasks, loading, error } = useAppData()
  if (loading) {
    return <PanelMessage>正在加载 task...</PanelMessage>
  }
  if (error) {
    return <PanelMessage>{error}</PanelMessage>
  }
  if (tasks.length === 0) {
    return <PanelMessage>当前没有 task。</PanelMessage>
  }
  return <Navigate params={{ taskId: tasks[0]!.id }} replace to="/tasks/$taskId" />
}

export function TaskDetailPage() {
  const navigate = useNavigate()
  const { reload } = useAppData()
  const { taskId } = useParams({ from: '/tasks/$taskId' })
  const [task, setTask] = useState<TaskRecord | null>(null)
  const [artifact, setArtifact] = useState<TaskArtifactName>('prd-refined.md')
  const [selectedDiffRepo, setSelectedDiffRepo] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        setLoading(true)
        const detail = await getTask(taskId)
        if (cancelled) {
          return
        }
        setTask(detail)
        const firstDiffRepo = detail.repos.find((repo) => repo.diffSummary)?.id ?? detail.repos[0]?.id ?? ''
        setSelectedDiffRepo(firstDiffRepo)
        setError('')
      } catch (err) {
        if (cancelled) {
          return
        }
        setError(err instanceof Error ? err.message : '加载 task 详情失败')
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    setArtifact('prd-refined.md')
    void load()
    return () => {
      cancelled = true
    }
  }, [taskId])

  useEffect(() => {
    if (!task || task.status !== 'initialized') {
      return
    }

    let cancelled = false
    const timer = window.setInterval(() => {
      void getTask(taskId)
        .then((detail) => {
          if (cancelled) {
            return
          }
          setTask(detail)
          const firstDiffRepo = detail.repos.find((repo) => repo.diffSummary)?.id ?? detail.repos[0]?.id ?? ''
          setSelectedDiffRepo(firstDiffRepo)
          if (detail.status !== 'initialized') {
            void reload()
          }
        })
        .catch(() => {})
    }, 2500)

    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [task, taskId, reload])

  if (loading) {
    return <PanelMessage>正在加载 task 详情...</PanelMessage>
  }
  if (error) {
    return <PanelMessage>{error}</PanelMessage>
  }
  if (!task) {
    return <PanelMessage>未找到对应 task。</PanelMessage>
  }
  const deletableStatuses = new Set<TaskStatus>(['initialized', 'refined', 'planned', 'failed'])
  const canDelete = deletableStatuses.has(task.status)

  return (
    <div className="space-y-4 lg:h-full lg:overflow-y-auto lg:pr-1">
      <section className="overflow-hidden rounded-[24px] border border-stone-200 bg-[#111317] text-white shadow-[0_30px_80px_rgba(15,23,42,0.18)]">
        <div className="border-b border-white/8 bg-[linear-gradient(135deg,_rgba(16,185,129,0.18),_transparent_38%),linear-gradient(180deg,_rgba(255,255,255,0.04),_rgba(255,255,255,0))] px-5 py-5">
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <StatusBadge status={task.status} />
            <span className="rounded-full border border-white/12 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-stone-300">
              {task.sourceType}
            </span>
            <span className="rounded-full border border-white/12 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-stone-300">
              {task.complexity}
            </span>
          </div>
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]">
            <div>
              <div className="text-xs font-semibold uppercase tracking-[0.24em] text-stone-400">Task Detail</div>
              <h3 className="mt-2 text-[30px] font-semibold tracking-[-0.05em] text-white">{task.title}</h3>
              <p className="mt-3 max-w-3xl text-sm leading-6 text-stone-300">
                真实数据视图下，task 主记录、多仓 repo 状态、设计文档和 code 日志都在同一页汇总，方便回看和排障。
              </p>
              {canDelete ? (
                <div className="mt-4 flex flex-wrap items-center gap-3">
                  <button
                    className="rounded-2xl border border-rose-300/30 bg-rose-400/10 px-4 py-3 text-sm font-semibold text-rose-100 transition hover:border-rose-200/40 hover:bg-rose-400/20"
                    onClick={async () => {
                      const confirmed = window.confirm(`确认删除 task ${task.id}？仅允许删除未进入 code 的 task。`)
                      if (!confirmed) {
                        return
                      }
                      try {
                        await deleteTask(task.id)
                        await reload()
                        void navigate({ to: '/tasks' })
                      } catch (err) {
                        window.alert(err instanceof Error ? err.message : '删除 task 失败')
                      }
                    }}
                    type="button"
                  >
                    Delete Task
                  </button>
                  <span className="text-xs text-stone-400">只适用于未进入 code 的 task</span>
                </div>
              ) : null}
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
              <CompactField label="task_id" value={task.id} />
              <CompactField label="updated" value={task.updatedAt} />
              <CompactField label="owner" value={task.owner} />
              <CompactField label="repos" value={`${task.repos.length}`} />
            </div>
          </div>
        </div>
        <div className="grid gap-4 px-5 py-5 lg:grid-cols-[minmax(0,1fr)_360px]">
          <div className="rounded-[20px] border border-white/8 bg-white/3 p-4">
            <div className="mb-3 text-xs font-semibold uppercase tracking-[0.22em] text-stone-400">Workflow Timeline</div>
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              {task.timeline.map((step) => (
                <TimelineCard key={step.label} detail={step.detail} label={step.label} state={step.state} />
              ))}
            </div>
          </div>

          <div className="rounded-[20px] border border-emerald-300/20 bg-emerald-400/10 p-4">
            <div className="text-xs font-semibold uppercase tracking-[0.22em] text-emerald-200">Recommended Next</div>
            <div className="mt-3 text-xl font-semibold tracking-[-0.04em] text-white">
              {task.status === 'coded'
                ? '确认结果后归档'
                : task.status === 'partially_coded'
                  ? '继续推进剩余仓库'
                  : task.status === 'planned'
                    ? '进入隔离 code 阶段'
                    : '继续推进 task'}
            </div>
            <p className="mt-3 text-sm leading-6 text-emerald-100/85">
              下一步来源于 task 当前状态和 repo 子状态聚合，不再是静态 mock 文案。
            </p>
            {task.status === 'initialized' ? (
              <div className="mt-3 rounded-2xl border border-amber-300/20 bg-amber-400/10 px-4 py-3 text-sm leading-6 text-amber-100">
                Refine 正在后台执行。页面会自动轮询；如果长时间停留在当前状态，可以直接打开 `refine.log` 查看卡在哪一阶段。
              </div>
            ) : null}
            <div className="mt-4 rounded-2xl border border-emerald-200/20 bg-stone-950/50 px-4 py-3 font-mono text-sm text-emerald-100">
              {task.nextAction}
            </div>
          </div>
        </div>
      </section>

      <div className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_360px]">
        <section className="rounded-[24px] border border-stone-200 bg-stone-50/70 p-4 dark:border-white/10 dark:bg-white/5">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">Artifacts</div>
              <h4 className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-stone-950 dark:text-stone-50">文档与结果产物</h4>
            </div>
            <div className="text-sm text-stone-500 dark:text-stone-400">task 主目录下的真实文件内容与 code 日志</div>
          </div>

          <div className="mb-4 flex flex-wrap gap-2">
            {(Object.keys(task.artifacts) as TaskArtifactName[]).map((name) => (
              <button
                className={`rounded-full border px-3 py-2 text-sm font-medium transition ${
                  artifact === name
                    ? 'border-stone-900 bg-stone-900 text-white shadow-sm dark:border-stone-100 dark:bg-stone-100 dark:text-stone-950'
                    : 'border-stone-200 bg-white text-stone-600 hover:border-stone-400 hover:text-stone-950 dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-300 dark:hover:border-white/20 dark:hover:text-stone-100'
                }`}
                key={name}
                onClick={() => setArtifact(name)}
                type="button"
              >
                {artifactLabel(name)}
              </button>
            ))}
          </div>

          <ArtifactViewer artifact={artifact} content={task.artifacts[artifact] || ''} taskID={task.id} />
        </section>

        <aside className="space-y-4">
          <ActionPanel task={task} />
          <DeletePolicyCard canDelete={canDelete} status={task.status} />
          <RepoScopeCard task={task} />
          <CodeResultCard task={task} />
          <DiffPanel repos={task.repos} selectedRepo={selectedDiffRepo} onSelectRepo={setSelectedDiffRepo} />
          <section className="rounded-[24px] border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/6">
            <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">Why This Matters</div>
            <h4 className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-stone-950 dark:text-stone-50">这版带来的直接价值</h4>
            <ul className="mt-4 space-y-3 text-sm leading-6 text-stone-600 dark:text-stone-300">
              <li>用户可以在一个页面里查看多仓 task 的关键状态，减少来回切换和沟通成本。</li>
              <li>接入真实 task 数据后，页面展示和实际流程保持一致，更容易发现 repo scope、分支、worktree、commit 和 code.log 的问题。</li>
              <li>先以只读工作台验证方案，投入更小、试错更快，再决定是否追加触发操作和控制能力。</li>
            </ul>
          </section>
        </aside>
      </div>
    </div>
  )
}

function DeletePolicyCard({
  canDelete,
  status,
}: {
  canDelete: boolean
  status: TaskStatus
}) {
  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/6">
      <div className="mb-3 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">Delete Policy</div>
      {canDelete ? (
        <div className="rounded-[18px] border border-emerald-200 bg-emerald-50 px-3 py-3 text-sm leading-6 text-emerald-800 dark:border-emerald-300/20 dark:bg-emerald-400/10 dark:text-emerald-100">
          当前状态为 `{status}`，允许删除。删除动作只适用于未进入 code 的 task。
        </div>
      ) : (
        <div className="rounded-[18px] border border-amber-200 bg-amber-50 px-3 py-3 text-sm leading-6 text-amber-900 dark:border-amber-300/20 dark:bg-amber-400/10 dark:text-amber-100">
          当前状态为 `{status}`，已进入或完成 code 阶段，不允许直接删除。
        </div>
      )}
    </section>
  )
}

function TaskListItemCard({ task }: { task: TaskListItem }) {
  const location = useLocation()
  const active = location.pathname === `/tasks/${task.id}`

  return (
    <Link
      className={`block rounded-[22px] border px-4 py-4 transition ${
        active
          ? 'border-stone-900 bg-stone-900 text-white shadow-[0_16px_40px_rgba(15,23,42,0.2)] dark:border-stone-100 dark:bg-stone-100 dark:text-stone-950'
          : 'border-stone-200 bg-white text-stone-900 hover:border-stone-300 hover:bg-stone-100/80 dark:border-white/10 dark:bg-white/6 dark:text-stone-100 dark:hover:border-white/20 dark:hover:bg-white/10'
      }`}
      params={{ taskId: task.id }}
      to="/tasks/$taskId"
    >
      <div className="flex items-center justify-between gap-3">
        <StatusBadge status={task.status} />
        <div className={`text-xs ${active ? 'text-stone-300 dark:text-stone-500' : 'text-stone-500 dark:text-stone-400'}`}>{task.updatedAt}</div>
      </div>
      <div className="mt-3 text-[17px] font-semibold leading-6 tracking-[-0.03em]">{task.title}</div>
      <div className={`mt-2 text-xs font-mono ${active ? 'text-stone-400 dark:text-stone-500' : 'text-stone-500 dark:text-stone-400'}`}>{task.id}</div>
      <div className={`mt-4 flex items-center justify-between text-xs ${active ? 'text-stone-300 dark:text-stone-500' : 'text-stone-500 dark:text-stone-400'}`}>
        <span>{task.status === 'initialized' ? 'Refining…' : task.repoIds.slice(0, 2).join(', ') || '-'}</span>
        <span>{task.repoCount} repo(s)</span>
      </div>
    </Link>
  )
}

function RepoScopeCard({ task }: { task: TaskRecord }) {
  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/6">
      <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">Repo Scope</div>
      <div className="space-y-3">
        <KeyValue label="Repos Involved" value={`${task.repos.length}`} />
        <KeyValue label="Repo IDs" value={task.repos.map((repo) => repo.id).join(', ')} />
      </div>

      <div className="mt-4 space-y-2">
        {task.repos.map((repo) => (
          <div className="rounded-[18px] border border-stone-200 bg-stone-50 px-3 py-3 dark:border-white/10 dark:bg-white/5" key={repo.id}>
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-stone-950 dark:text-stone-50">{repo.displayName}</div>
                <div className="mt-1 font-mono text-[11px] text-stone-500 dark:text-stone-400">{repo.path}</div>
              </div>
              <RepoStatusBadge status={repo.status} />
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function CodeResultCard({ task }: { task: TaskRecord }) {
  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/6">
      <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">Repo Code Results</div>
      <div className="space-y-4">
        {task.repos.map((repo) => (
          <RepoCodeResult key={repo.id} repo={repo} />
        ))}
      </div>
    </section>
  )
}

function RepoCodeResult({ repo }: { repo: TaskRecord['repos'][number] }) {
  return (
    <div className="rounded-[20px] border border-stone-200 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/5">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <div className="text-sm font-semibold text-stone-950 dark:text-stone-50">{repo.displayName}</div>
          <div className="mt-1 text-[11px] uppercase tracking-[0.2em] text-stone-500 dark:text-stone-400">{repo.id}</div>
        </div>
        <RepoStatusBadge status={repo.status} />
      </div>
      <div className="space-y-3">
        <KeyValue label="Build" value={repo.build === 'passed' ? 'Passed' : repo.build === 'failed' ? 'Failed' : 'Pending'} />
        <KeyValue label="Branch" mono value={repo.branch ?? '尚未创建'} />
        <KeyValue label="Worktree" mono value={repo.worktree ?? '尚未创建'} />
        <KeyValue label="Commit" mono value={repo.commit ?? '尚未提交'} />
      </div>

      <div className="mt-4 rounded-[18px] border border-stone-200 bg-white px-3 py-3 dark:border-white/10 dark:bg-stone-950/70">
        <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500 dark:text-stone-400">Files Written</div>
        <div className="mt-2 space-y-2">
          {(repo.filesWritten && repo.filesWritten.length > 0 ? repo.filesWritten : ['尚无写入结果']).map((file) => (
            <div className="font-mono text-xs text-stone-800 dark:text-stone-200" key={file}>
              {file}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
