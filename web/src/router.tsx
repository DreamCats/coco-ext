import {
  Link,
  Navigate,
  Outlet,
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
  useLocation,
  useParams,
} from '@tanstack/react-router'
import { useState } from 'react'
import {
  getTask,
  tasks,
  workspaceSummary,
  type RepoResult,
  type TaskArtifactName,
  type TaskRecord,
} from './mock-data'

function AppShell() {
  const location = useLocation()

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,_rgba(34,197,94,0.12),_transparent_32%),radial-gradient(circle_at_top_right,_rgba(249,115,22,0.15),_transparent_28%),linear-gradient(180deg,_#f6f4ee_0%,_#f2efe7_52%,_#ede7dc_100%)] text-stone-950">
      <div className="mx-auto flex min-h-screen max-w-[1600px] flex-col px-4 py-4 md:px-6 lg:px-8">
        <header className="mb-4 rounded-[28px] border border-stone-200/80 bg-white/80 px-5 py-4 shadow-[0_1px_0_rgba(255,255,255,0.7)_inset,0_24px_50px_rgba(54,47,35,0.06)] backdrop-blur">
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-700">
                  Local Mock UI
                </div>
                <h1 className="text-3xl font-semibold tracking-[-0.04em] text-stone-950">
                  coco-ext PRD Workflow Console
                </h1>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-stone-600">
                  用 mock 数据验证任务流、产物承载和 worktree 隔离表达。当前目标不是实现业务逻辑，而是先把
                  产品感和讲故事路径做对。
                </p>
              </div>
              <div className="grid gap-3 text-sm text-stone-600 sm:grid-cols-3">
                <MetricCard label="Active Tasks" value={`${tasks.length}`} tone="emerald" />
                <MetricCard
                  label="Coded Ready"
                  value={`${tasks.filter((task) => task.status === 'coded' || task.status === 'partially_coded').length}`}
                  tone="amber"
                />
                <MetricCard label="Repos Involved" value={`${workspaceSummary.reposInvolved.length}`} tone="sky" />
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2 border-t border-stone-200/80 pt-4">
              <TopNavItem
                description="任务流列表、详情、产物与 code 结果"
                isActive={location.pathname.startsWith('/tasks') || location.pathname === '/'}
                title="PRD"
                to="/tasks"
              />
              <TopNavItem
                description="路径、worktree 与分支绑定关系"
                isActive={location.pathname.startsWith('/workspace')}
                title="Workspace"
                to="/workspace"
              />
            </div>
          </div>
        </header>

        <main className="min-h-0 flex-1 rounded-[32px] border border-stone-200/80 bg-white/72 p-4 shadow-[0_1px_0_rgba(255,255,255,0.7)_inset,0_40px_80px_rgba(64,45,18,0.08)] backdrop-blur lg:p-5">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

function TasksLayout() {
  return (
    <div className="grid gap-4 lg:h-[calc(100vh-235px)] lg:grid-cols-[360px_minmax(0,1fr)]">
      <section className="overflow-hidden rounded-[24px] border border-stone-200 bg-stone-50/80 p-4">
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">
              PRD
            </div>
            <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-stone-950">任务流列表</h2>
          </div>
          <div className="rounded-2xl bg-stone-900 px-3 py-2 text-right text-white">
            <div className="text-[11px] uppercase tracking-[0.24em] text-stone-400">Latest</div>
            <div className="text-sm font-semibold">{tasks[0]?.updatedAt}</div>
          </div>
        </div>

        <div className="mb-4 grid grid-cols-3 gap-2 text-xs">
          <FilterChip label="All" value={`${tasks.length}`} />
          <FilterChip
            label="Coded"
            value={`${tasks.filter((task) => task.status === 'coded' || task.status === 'partially_coded').length}`}
          />
          <FilterChip label="Planned" value={`${tasks.filter((task) => task.status === 'planned').length}`} />
        </div>

        <div className="space-y-3 lg:max-h-[calc(100%-112px)] lg:overflow-y-auto lg:pr-1">
          {tasks.map((task) => (
            <TaskListItem key={task.id} task={task} />
          ))}
        </div>
      </section>

      <div className="min-h-0 overflow-hidden">
        <Outlet />
      </div>
    </div>
  )
}

function TasksIndexPage() {
  return <Navigate params={{ taskId: tasks[0]!.id }} replace to="/tasks/$taskId" />
}

function TaskDetailPage() {
  const { taskId } = useParams({ from: '/tasks/$taskId' })
  const task = getTask(taskId)
  const [artifact, setArtifact] = useState<TaskArtifactName>('prd-refined.md')

  if (!task) {
    return (
      <section className="flex min-h-[720px] items-center justify-center rounded-[24px] border border-dashed border-stone-300 bg-stone-50 p-8 text-center text-stone-500">
        未找到对应 task。
      </section>
    )
  }

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
                这是多仓 task 的主视图。你可以在同一页里看到阶段状态、产物文档、各仓库的执行结果以及下一步动作，
                不需要再把任务强行挂在某一个仓库里理解。
              </p>
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
              <CompactField label="task_id" value={task.id} />
              <CompactField label="updated" value={task.updatedAt} />
              <CompactField label="owner" value={task.owner} />
              <CompactField label="primary repo" value={task.primaryRepo} />
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
              多仓 task 下，“下一步”通常不是单一代码结果，而是某个 repo 还需要继续执行 code、reset 或 archive。
            </p>
            <div className="mt-4 rounded-2xl border border-emerald-200/20 bg-stone-950/50 px-4 py-3 font-mono text-sm text-emerald-100">
              {task.nextAction}
            </div>
          </div>
        </div>
      </section>

      <div className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_360px]">
        <section className="rounded-[24px] border border-stone-200 bg-stone-50/70 p-4">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Artifacts</div>
              <h4 className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-stone-950">文档与结果产物</h4>
            </div>
            <div className="text-sm text-stone-500">以 tab 承载 `.livecoding/tasks/&lt;task-id&gt;/` 里的核心文件</div>
          </div>

          <div className="mb-4 flex flex-wrap gap-2">
            {(Object.keys(task.artifacts) as TaskArtifactName[]).map((name) => (
              <button
                className={`rounded-full border px-3 py-2 text-sm font-medium transition ${
                  artifact === name
                    ? 'border-stone-900 bg-stone-900 text-white shadow-sm'
                    : 'border-stone-200 bg-white text-stone-600 hover:border-stone-400 hover:text-stone-950'
                }`}
                key={name}
                onClick={() => setArtifact(name)}
                type="button"
              >
                {name}
              </button>
            ))}
          </div>

          <div className="overflow-hidden rounded-[22px] border border-stone-200 bg-[#0d1014] shadow-[0_20px_60px_rgba(17,24,39,0.12)]">
            <div className="flex items-center justify-between border-b border-white/8 px-4 py-3 text-sm text-stone-300">
              <div className="font-semibold text-stone-100">{artifact}</div>
              <div className="font-mono text-xs text-stone-500">.livecoding/tasks/{task.id}/{artifact}</div>
            </div>
            <pre className="max-h-[520px] overflow-auto px-4 py-4 text-[13px] leading-6 text-stone-200">
              {task.artifacts[artifact]}
            </pre>
          </div>
        </section>

        <aside className="space-y-4">
          <RepoScopeCard task={task} />
          <CodeResultCard task={task} />
          <section className="rounded-[24px] border border-stone-200 bg-white p-4">
            <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Why This Matters</div>
            <h4 className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-stone-950">讲故事的关键区块</h4>
            <ul className="mt-4 space-y-3 text-sm leading-6 text-stone-600">
              <li>把 “有 task、有阶段、有产物、有 repo scope” 清晰放到同一页。</li>
              <li>把每个 repo 的 branch / worktree / commit 分开显示，强调任务主记录和代码执行现场不是同一个概念。</li>
              <li>把下一步命令固定下来，说明多仓任务也能继续推进，而不是停在报告层。</li>
            </ul>
          </section>
        </aside>
      </div>
    </div>
  )
}

function WorkspacePage() {
  return (
    <div className="space-y-4">
      <section className="rounded-[26px] border border-stone-200 bg-stone-50/70 p-5">
        <div className="max-w-4xl">
          <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Workspace</div>
          <h2 className="mt-2 text-[32px] font-semibold tracking-[-0.05em] text-stone-950">隔离执行与路径关系</h2>
          <p className="mt-3 text-sm leading-6 text-stone-600">
            这个页面不是给所有用户高频使用，而是为了把 `.livecoding/`、`.coco-ext-worktree/`、branch 和
            code-result 之间的绑定关系讲清楚。对排障和 demo 都非常有价值。
          </p>
        </div>
      </section>

      <div className="grid gap-4 xl:grid-cols-[1.2fr_0.8fr]">
        <section className="rounded-[24px] border border-stone-200 bg-white p-4">
          <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Paths</div>
          <div className="space-y-3">
            <PathCard label="Repo Root" value={workspaceSummary.repoRoot} />
            <PathCard label="Tasks Root" value={workspaceSummary.tasksRoot} />
            <PathCard label="Context Root" value={workspaceSummary.contextRoot} />
            <PathCard label="Worktree Root" value={workspaceSummary.worktreeRoot} />
          </div>
        </section>

        <section className="rounded-[24px] border border-stone-200 bg-[#161a1f] p-4 text-stone-200">
          <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Repos Involved</div>
          <div className="space-y-2">
            {workspaceSummary.reposInvolved.map((repo) => (
              <div className="rounded-2xl border border-white/8 bg-white/4 px-3 py-3 font-mono text-sm" key={repo}>
                {repo}
              </div>
            ))}
          </div>
        </section>
      </div>

      <section className="rounded-[24px] border border-stone-200 bg-white p-4">
        <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Recent Worktrees</div>
        <div className="space-y-3">
          {tasks
            .flatMap((task) => task.repos.filter((repo) => repo.worktree).map((repo) => ({ task, repo })))
            .map(({ repo, task }) => (
              <div
                className="grid gap-3 rounded-[22px] border border-stone-200 bg-stone-50/80 p-4 lg:grid-cols-[minmax(0,1fr)_260px_220px]"
                key={`${task.id}-${repo.id}`}
              >
                <div>
                  <div className="text-sm font-semibold text-stone-950">{task.title}</div>
                  <div className="mt-1 text-xs text-stone-500">
                    {task.id} · <span className="font-semibold text-stone-700">{repo.displayName}</span>
                  </div>
                  <div className="mt-3 rounded-2xl border border-stone-200 bg-white px-3 py-3 font-mono text-xs text-stone-600">
                    {repo.worktree}
                  </div>
                </div>
                <div className="space-y-2">
                  <MiniMeta label="branch" value={repo.branch ?? '-'} />
                  <MiniMeta label="build" value={repo.build ?? '-'} />
                </div>
                <div className="space-y-2">
                  <MiniMeta label="commit" value={repo.commit ?? '-'} />
                  <MiniMeta label="files" value={`${repo.filesWritten?.length ?? 0}`} />
                </div>
              </div>
            ))}
        </div>
      </section>
    </div>
  )
}

function CodeResultCard({ task }: { task: TaskRecord }) {
  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4">
      <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Repo Code Results</div>
      <div className="space-y-4">
        {task.repos.map((repo) => (
          <RepoCodeResult key={repo.id} repo={repo} />
        ))}
      </div>

      {task.summary ? (
        <div className="mt-5 rounded-[20px] border border-amber-300/40 bg-amber-50 px-4 py-4 text-sm leading-6 text-amber-950">
          {task.summary}
        </div>
      ) : null}
    </section>
  )
}

function TaskListItem({ task }: { task: TaskRecord }) {
  const location = useLocation()
  const active = location.pathname === `/tasks/${task.id}`

  return (
    <Link
      className={`block rounded-[22px] border px-4 py-4 transition ${
        active
          ? 'border-stone-900 bg-stone-900 text-white shadow-[0_16px_40px_rgba(15,23,42,0.2)]'
          : 'border-stone-200 bg-white text-stone-900 hover:border-stone-300 hover:bg-stone-100/80'
      }`}
      params={{ taskId: task.id }}
      to="/tasks/$taskId"
    >
      <div className="flex items-center justify-between gap-3">
        <StatusBadge status={task.status} />
        <div className={`text-xs ${active ? 'text-stone-300' : 'text-stone-500'}`}>{task.updatedAt}</div>
      </div>
      <div className="mt-3 text-[17px] font-semibold leading-6 tracking-[-0.03em]">{task.title}</div>
      <div className={`mt-2 text-xs font-mono ${active ? 'text-stone-400' : 'text-stone-500'}`}>{task.id}</div>
      <div className={`mt-4 flex items-center justify-between text-xs ${active ? 'text-stone-300' : 'text-stone-500'}`}>
        <span>{task.primaryRepo}</span>
        <span>{task.repos.length} repo(s)</span>
      </div>
    </Link>
  )
}

function RepoScopeCard({ task }: { task: TaskRecord }) {
  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4">
      <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Repo Scope</div>
      <div className="space-y-3">
        <KeyValue label="Primary Repo" value={task.primaryRepo} />
        <KeyValue label="Repos Involved" value={`${task.repos.length}`} />
      </div>

      <div className="mt-4 space-y-2">
        {task.repos.map((repo) => (
          <div className="rounded-[18px] border border-stone-200 bg-stone-50 px-3 py-3" key={repo.id}>
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-stone-950">{repo.displayName}</div>
                <div className="mt-1 font-mono text-[11px] text-stone-500">{repo.path}</div>
              </div>
              <RepoStatusBadge status={repo.status} />
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function MetricCard({
  label,
  tone,
  value,
}: {
  label: string
  value: string
  tone: 'emerald' | 'amber' | 'sky'
}) {
  const toneClass =
    tone === 'emerald'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-800'
      : tone === 'amber'
        ? 'border-amber-200 bg-amber-50 text-amber-800'
        : 'border-sky-200 bg-sky-50 text-sky-800'

  return (
    <div className={`rounded-[20px] border px-4 py-3 ${toneClass}`}>
      <div className="text-[11px] uppercase tracking-[0.2em] opacity-70">{label}</div>
      <div className="mt-1 text-lg font-semibold">{value}</div>
    </div>
  )
}

function TopNavItem({
  description,
  isActive,
  title,
  to,
}: {
  title: string
  description: string
  to: '/tasks' | '/workspace'
  isActive: boolean
}) {
  return (
    <Link
      className={`block min-w-[220px] rounded-[22px] border px-4 py-4 transition ${
        isActive
          ? 'border-stone-900 bg-stone-900 text-white shadow-[0_16px_30px_rgba(15,23,42,0.14)]'
          : 'border-stone-200 bg-stone-50 text-stone-700 hover:border-stone-300 hover:bg-white'
      }`}
      to={to}
    >
      <div className="text-sm font-semibold">{title}</div>
      <div className={`mt-2 text-xs leading-5 ${isActive ? 'text-stone-300' : 'text-stone-500'}`}>{description}</div>
    </Link>
  )
}

function FilterChip({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-white px-3 py-3">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500">{label}</div>
      <div className="mt-1 text-lg font-semibold text-stone-950">{value}</div>
    </div>
  )
}

function StatusBadge({ status }: { status: TaskRecord['status'] }) {
  const tone =
    status === 'coded'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : status === 'partially_coded'
        ? 'border-orange-200 bg-orange-50 text-orange-700'
      : status === 'planned'
        ? 'border-amber-200 bg-amber-50 text-amber-700'
        : status === 'archived'
          ? 'border-sky-200 bg-sky-50 text-sky-700'
          : 'border-stone-200 bg-stone-100 text-stone-700'

  return <span className={`rounded-full border px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.2em] ${tone}`}>{status}</span>
}

function RepoStatusBadge({ status }: { status: RepoResult['status'] }) {
  const tone =
    status === 'coded'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : status === 'planned'
        ? 'border-amber-200 bg-amber-50 text-amber-700'
        : status === 'failed'
          ? 'border-rose-200 bg-rose-50 text-rose-700'
          : status === 'archived'
            ? 'border-sky-200 bg-sky-50 text-sky-700'
            : 'border-stone-200 bg-stone-100 text-stone-700'

  return <span className={`rounded-full border px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.2em] ${tone}`}>{status}</span>
}

function TimelineCard({
  detail,
  label,
  state,
}: {
  label: string
  detail: string
  state: 'done' | 'current' | 'pending'
}) {
  const tone =
    state === 'done'
      ? 'border-emerald-300/20 bg-emerald-400/10'
      : state === 'current'
        ? 'border-amber-300/20 bg-amber-400/10'
        : 'border-white/8 bg-white/4'

  return (
    <div className={`rounded-[18px] border p-4 ${tone}`}>
      <div className="flex items-center justify-between">
        <div className="text-sm font-semibold text-white">{label}</div>
        <div className="text-[11px] uppercase tracking-[0.2em] text-stone-400">{state}</div>
      </div>
      <div className="mt-3 text-sm leading-6 text-stone-300">{detail}</div>
    </div>
  )
}

function CompactField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[18px] border border-white/8 bg-white/4 px-3 py-3">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-400">{label}</div>
      <div className="mt-2 text-sm text-white">{value}</div>
    </div>
  )
}

function KeyValue({ label, mono = false, value }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-[18px] border border-stone-200 bg-stone-50 px-4 py-3">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500">{label}</div>
      <div className={`mt-2 text-sm text-stone-950 ${mono ? 'font-mono text-xs leading-5' : 'font-medium'}`}>{value}</div>
    </div>
  )
}

function PathCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[20px] border border-stone-200 bg-stone-50/80 px-4 py-4">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500">{label}</div>
      <div className="mt-2 font-mono text-xs leading-5 text-stone-800">{value}</div>
    </div>
  )
}

function MiniMeta({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[18px] border border-stone-200 bg-white px-3 py-3">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500">{label}</div>
      <div className="mt-2 font-mono text-xs text-stone-900">{value}</div>
    </div>
  )
}

function RepoCodeResult({ repo }: { repo: RepoResult }) {
  return (
    <div className="rounded-[20px] border border-stone-200 bg-stone-50 p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <div className="text-sm font-semibold text-stone-950">{repo.displayName}</div>
          <div className="mt-1 text-[11px] uppercase tracking-[0.2em] text-stone-500">{repo.id}</div>
        </div>
        <RepoStatusBadge status={repo.status} />
      </div>
      <div className="space-y-3">
        <KeyValue label="Build" value={repo.build === 'passed' ? 'Passed' : repo.build === 'failed' ? 'Failed' : 'Pending'} />
        <KeyValue label="Branch" mono value={repo.branch ?? '尚未创建'} />
        <KeyValue label="Worktree" mono value={repo.worktree ?? '尚未创建'} />
        <KeyValue label="Commit" mono value={repo.commit ?? '尚未提交'} />
      </div>

      <div className="mt-4 rounded-[18px] border border-stone-200 bg-white px-3 py-3">
        <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500">Files Written</div>
        <div className="mt-2 space-y-2">
          {(repo.filesWritten ?? ['尚无写入结果']).map((file) => (
            <div className="font-mono text-xs text-stone-800" key={file}>
              {file}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

const rootRoute = createRootRoute({
  component: AppShell,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: () => <Navigate replace to="/tasks" />,
})

const tasksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'tasks',
  component: TasksLayout,
})

const tasksIndexRoute = createRoute({
  getParentRoute: () => tasksRoute,
  path: '/',
  component: TasksIndexPage,
})

const taskDetailRoute = createRoute({
  getParentRoute: () => tasksRoute,
  path: '$taskId',
  component: TaskDetailPage,
})

const workspaceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'workspace',
  component: WorkspacePage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  tasksRoute.addChildren([tasksIndexRoute, taskDetailRoute]),
  workspaceRoute,
])

const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

export function AppRouter() {
  return <RouterProvider router={router} />
}
