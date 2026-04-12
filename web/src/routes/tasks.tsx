import { Link, Navigate, Outlet, useLocation, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useMemo, useRef, useState } from 'react'
import {
  createTask,
  deleteTask,
  getTask,
  startCode,
  startPlan,
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
  const createDialogRef = useRef<HTMLDivElement | null>(null)

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

  useEffect(() => {
    if (!showCreateForm) {
      return
    }

    const frame = window.requestAnimationFrame(() => {
      const block = window.matchMedia('(min-width: 640px)').matches ? 'center' : 'end'
      createDialogRef.current?.scrollIntoView({
        behavior: 'smooth',
        block,
        inline: 'nearest',
      })
    })

    return () => {
      window.cancelAnimationFrame(frame)
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
            <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">任务推进</div>
            <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-stone-950 dark:text-stone-50">任务队列</h2>
            <p className="mt-2 max-w-[280px] text-sm leading-6 text-stone-500 dark:text-stone-400">
              在这里查看每条需求的当前阶段、涉及仓库和下一步动作。
            </p>
          </div>
          <div className="flex flex-col gap-2">
            <button
              className="rounded-2xl bg-stone-900 px-4 py-3 text-sm font-semibold text-white transition hover:bg-stone-800"
              onClick={() => setShowCreateForm((current) => !current)}
              type="button"
            >
              {showCreateForm ? '收起新建入口' : '新建任务'}
            </button>
            <div className="rounded-2xl bg-stone-900 px-3 py-2 text-right text-white dark:bg-stone-100 dark:text-stone-950">
              <div className="text-[11px] uppercase tracking-[0.24em] text-stone-400 dark:text-stone-500">最近更新</div>
              <div className="text-sm font-semibold">{tasks[0]?.updatedAt ?? '-'}</div>
            </div>
          </div>
        </div>

        <div className="mb-4 grid grid-cols-3 gap-2 text-xs">
          <FilterChip label="全部任务" value={loading ? '...' : `${tasks.length}`} />
          <FilterChip
            label="已有结果"
            value={loading ? '...' : `${tasks.filter((task) => task.status === 'coded' || task.status === 'partially_coded').length}`}
          />
          <FilterChip
            label="待推进"
            value={loading ? '...' : `${tasks.filter((task) => task.status === 'planned' || task.status === 'planning').length}`}
          />
        </div>

        <div className="mb-4 space-y-3">
          <input
            className="w-full rounded-2xl border border-stone-200 bg-white px-3 py-3 text-sm text-stone-900 outline-none transition placeholder:text-stone-400 focus:border-stone-400 dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-100 dark:placeholder:text-stone-500 dark:focus:border-white/20"
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索任务标题、仓库或任务编号"
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
              <option value="planned">待进入实现</option>
              <option value="planning">方案生成中</option>
              <option value="coding">实现进行中</option>
              <option value="partially_coded">部分已完成</option>
              <option value="coded">已产出结果</option>
              <option value="archived">已归档</option>
              <option value="failed">处理中断</option>
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
            ref={createDialogRef}
          >
            <div className="flex items-start justify-between gap-4 border-b border-stone-200 px-5 py-5 dark:border-white/10 md:px-6">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">新建任务</div>
                <h3 className="mt-2 text-[30px] font-semibold tracking-[-0.05em] text-stone-950 dark:text-stone-50">新建需求任务</h3>
                <p className="mt-2 text-sm leading-6 text-stone-500 dark:text-stone-400">
                  先确认需求内容和涉及仓库，系统会在后台整理需求，生成可继续推进的任务。
                </p>
              </div>
              <button
                className="rounded-full border border-stone-200 px-3 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-stone-500 transition hover:border-stone-300 hover:text-stone-900 dark:border-white/10 dark:text-stone-400 dark:hover:border-white/20 dark:hover:text-stone-100"
                onClick={closeCreateForm}
                type="button"
              >
                关闭
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
                  {creating ? '创建中...' : '创建任务'}
                </button>
                <button
                  className="rounded-2xl border border-stone-200 px-5 py-3 text-sm text-stone-700 transition hover:border-stone-300 hover:bg-stone-50 dark:border-white/10 dark:text-stone-300 dark:hover:border-white/20 dark:hover:bg-white/10"
                  onClick={closeCreateForm}
                  type="button"
                >
                  取消
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
    return <PanelMessage>正在加载任务...</PanelMessage>
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
  const [planStarting, setPlanStarting] = useState(false)
  const [codeStarting, setCodeStarting] = useState(false)
  const [actionError, setActionError] = useState('')

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
        setActionError('')
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
    if (!task || !new Set<TaskStatus>(['initialized', 'planning', 'coding']).has(task.status)) {
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
          if (detail.status !== 'initialized' && detail.status !== 'planning' && detail.status !== 'coding') {
            setPlanStarting(false)
            setCodeStarting(false)
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
    return <PanelMessage>正在加载任务详情...</PanelMessage>
  }
  if (error) {
    return <PanelMessage>{error}</PanelMessage>
  }
  if (!task) {
    return <PanelMessage>未找到对应 task。</PanelMessage>
  }
  const hasGeneratedPlan = hasActionableArtifact(task.artifacts['design.md']) && hasActionableArtifact(task.artifacts['plan.md'])
  const deletableStatuses = new Set<TaskStatus>(['initialized', 'refined', 'planned', 'failed'])
  const canDelete = deletableStatuses.has(task.status)
  const canStartPlan = task.status === 'refined' || task.status === 'planned'
  const canStartCode = task.repos.length === 1 && (task.status === 'planned' || (task.status === 'failed' && hasGeneratedPlan))
  const planActionLabel = task.status === 'planned' ? '重新 Plan' : '开始 Plan'
  const codeActionLabel = task.status === 'failed' ? '重试实现' : '开始实现'

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
                在这里查看需求文档、实施方案、涉及仓库和最新结果，让推进路径始终清晰。
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
                    删除任务
                  </button>
                  <span className="text-xs text-stone-400">仅限尚未进入实现阶段的任务</span>
                </div>
              ) : null}
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
              <CompactField label="任务编号" value={task.id} />
              <CompactField label="最近更新" value={task.updatedAt} />
              <CompactField label="负责人" value={task.owner} />
              <CompactField label="涉及仓库" value={`${task.repos.length}`} />
            </div>
          </div>
        </div>
        <div className="grid gap-4 px-5 py-5 lg:grid-cols-[minmax(0,1fr)_360px]">
          <div className="rounded-[20px] border border-white/8 bg-white/3 p-4">
            <div className="mb-3 text-xs font-semibold uppercase tracking-[0.22em] text-stone-400">推进阶段</div>
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              {task.timeline.map((step) => (
                <TimelineCard key={step.label} detail={step.detail} label={step.label} state={step.state} />
              ))}
            </div>
          </div>

          <div className="rounded-[20px] border border-emerald-300/20 bg-emerald-400/10 p-4">
            <div className="text-xs font-semibold uppercase tracking-[0.22em] text-emerald-200">下一步建议</div>
            <div className="mt-3 text-xl font-semibold tracking-[-0.04em] text-white">
              {task.status === 'coded'
                ? '确认结果后归档'
                : task.status === 'partially_coded'
                  ? '继续推进剩余仓库'
                  : task.status === 'coding'
                    ? '等待实现完成'
                  : task.status === 'planning'
                    ? '等待方案生成完成'
                  : task.status === 'planned'
                    ? '开始生成实现'
                    : task.status === 'failed' && canStartCode
                      ? '调整后重新实现'
                    : task.status === 'refined'
                      ? '开始生成方案'
                      : '继续推进当前任务'}
            </div>
            <p className="mt-3 text-sm leading-6 text-emerald-100/85">
              系统会根据当前阶段给出建议动作，帮助你把任务继续往前推进。
            </p>
            {task.status === 'initialized' ? (
              <div className="mt-3 rounded-2xl border border-amber-300/20 bg-amber-400/10 px-4 py-3 text-sm leading-6 text-amber-100">
                需求正在整理成可执行任务。若停留时间过长，可打开 `refine.log` 查看进度。
              </div>
            ) : null}
            {task.status === 'planning' ? (
              <div className="mt-3 rounded-2xl border border-sky-300/20 bg-sky-400/10 px-4 py-3 text-sm leading-6 text-sky-100">
                正在分析代码并生成方案。若停留时间过长，可打开 `plan.log` 查看进度。
              </div>
            ) : null}
            {task.status === 'coding' ? (
              <div className="mt-3 rounded-2xl border border-emerald-300/20 bg-emerald-400/10 px-4 py-3 text-sm leading-6 text-emerald-100">
                正在后台生成实现并验证结果。若停留时间过长，可打开 `code.log` 查看进度。
              </div>
            ) : null}
            {task.status === 'planned' && task.repos.length > 1 ? (
              <div className="mt-3 rounded-2xl border border-amber-300/20 bg-amber-400/10 px-4 py-3 text-sm leading-6 text-amber-100">
                当前 Web 端先支持单仓实现。多仓任务请先在终端按 repo 逐个推进。
              </div>
            ) : null}
            {actionError ? (
              <div className="mt-3 rounded-2xl border border-rose-300/20 bg-rose-400/10 px-4 py-3 text-sm leading-6 text-rose-100">
                {actionError}
              </div>
            ) : null}
            {canStartCode || canStartPlan ? (
              <div className="mt-4 flex flex-wrap items-center gap-3">
                {canStartCode ? (
                  <>
                    <button
                      className="rounded-2xl border border-emerald-200/30 bg-emerald-400/20 px-4 py-3 text-sm font-semibold text-emerald-50 transition hover:border-emerald-100/40 hover:bg-emerald-400/30 disabled:cursor-not-allowed disabled:opacity-60"
                      disabled={codeStarting || planStarting}
                      onClick={async () => {
                        try {
                          setCodeStarting(true)
                          setActionError('')
                          const result = await startCode(task.id)
                          setTask({
                            ...task,
                            status: result.status as TaskStatus,
                            nextAction: '实现正在执行，请稍候刷新任务详情。',
                          })
                          setArtifact('code.log')
                          await reload()
                        } catch (err) {
                          setActionError(err instanceof Error ? err.message : '启动实现失败')
                          setCodeStarting(false)
                        }
                      }}
                      type="button"
                    >
                      {codeStarting ? '实现进行中...' : codeActionLabel}
                    </button>
                    <span className="text-xs text-emerald-100/80">会在后台创建隔离工作区、生成改动并尝试完成构建验证</span>
                  </>
                ) : null}
                {canStartPlan ? (
                  <>
                    <button
                      className="rounded-2xl border border-white/20 bg-white/8 px-4 py-3 text-sm font-semibold text-white transition hover:border-white/30 hover:bg-white/12 disabled:cursor-not-allowed disabled:opacity-60"
                      disabled={planStarting || codeStarting}
                      onClick={async () => {
                        try {
                          setPlanStarting(true)
                          setActionError('')
                          const result = await startPlan(task.id)
                          setTask({
                            ...task,
                            status: result.status as TaskStatus,
                            nextAction: '方案正在生成，请稍候刷新任务详情。',
                          })
                          setArtifact('plan.log')
                          await reload()
                        } catch (err) {
                          setActionError(err instanceof Error ? err.message : '启动 plan 失败')
                          setPlanStarting(false)
                        }
                      }}
                      type="button"
                    >
                      {planStarting ? '方案生成中...' : planActionLabel}
                    </button>
                    <span className="text-xs text-emerald-100/80">
                      {task.status === 'planned'
                        ? '重新分析代码，并覆盖 `design.md` / `plan.md`'
                        : '在后台生成 `design.md` / `plan.md`'}
                    </span>
                  </>
                ) : null}
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
            <div className="text-sm text-stone-500 dark:text-stone-400">查看需求、方案、日志和结果产物</div>
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
          <DeletePolicyCard canDelete={canDelete} />
          <RepoScopeCard task={task} />
          <CodeResultCard task={task} />
          <DiffPanel repos={task.repos} selectedRepo={selectedDiffRepo} onSelectRepo={setSelectedDiffRepo} />
        </aside>
      </div>
    </div>
  )
}

function hasActionableArtifact(content?: string) {
  if (!content) {
    return false
  }
  return !content.includes("当前没有") && !content.includes("当前为空")
}

function DeletePolicyCard({
  canDelete,
}: {
  canDelete: boolean
}) {
  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/6">
      <div className="mb-3 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">Delete Policy</div>
      {canDelete ? (
        <div className="rounded-[18px] border border-emerald-200 bg-emerald-50 px-3 py-3 text-sm leading-6 text-emerald-800 dark:border-emerald-300/20 dark:bg-emerald-400/10 dark:text-emerald-100">
          当前阶段支持直接删除。如果这条需求不再继续，可以在这里移除。
        </div>
      ) : (
        <div className="rounded-[18px] border border-amber-200 bg-amber-50 px-3 py-3 text-sm leading-6 text-amber-900 dark:border-amber-300/20 dark:bg-amber-400/10 dark:text-amber-100">
          当前任务已经进入实现流程。如需回退，建议先处理已有结果后再操作。
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
        <span>
          {task.status === 'initialized'
            ? 'Refining…'
            : task.status === 'planning'
              ? 'Planning…'
              : task.repoIds.slice(0, 2).join(', ') || '-'}
        </span>
        <span>{task.repoCount} 个仓库</span>
      </div>
    </Link>
  )
}

function RepoScopeCard({ task }: { task: TaskRecord }) {
  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/6">
      <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">涉及仓库</div>
      <div className="space-y-3">
        <KeyValue label="仓库数量" value={`${task.repos.length}`} />
        <KeyValue label="仓库标识" value={task.repos.map((repo) => repo.id).join(', ')} />
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
      <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">仓库结果</div>
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
        <KeyValue label="构建结果" value={repo.build === 'passed' ? '已通过' : repo.build === 'failed' ? '未通过' : '待生成'} />
        <KeyValue label="分支" mono value={repo.branch ?? '尚未创建'} />
        <KeyValue label="工作区" mono value={repo.worktree ?? '尚未创建'} />
        <KeyValue label="提交" mono value={repo.commit ?? '尚未提交'} />
      </div>

      <div className="mt-4 rounded-[18px] border border-stone-200 bg-white px-3 py-3 dark:border-white/10 dark:bg-stone-950/70">
        <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500 dark:text-stone-400">变更文件</div>
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
