import { useEffect, useState } from 'react'
import { getTask, type TaskRecord } from '../api'
import { MiniMeta, PanelMessage, PathCard } from '../components/ui-primitives'
import { useAppData } from '../hooks/use-app-data'

export function WorkspacePage() {
  const { tasks, workspace, loading, error } = useAppData()

  if (loading) {
    return <PanelMessage>正在加载 workspace...</PanelMessage>
  }
  if (error) {
    return <PanelMessage>{error}</PanelMessage>
  }
  if (!workspace) {
    return <PanelMessage>未加载到 workspace 数据。</PanelMessage>
  }

  return (
    <div className="space-y-4">
      <section className="rounded-[26px] border border-stone-200 bg-stone-50/70 p-5">
        <div className="max-w-4xl">
          <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Workspace</div>
          <h2 className="mt-2 text-[32px] font-semibold tracking-[-0.05em] text-stone-950">隔离执行与路径关系</h2>
          <p className="mt-3 text-sm leading-6 text-stone-600">
            这个页面用于解释 tasks、context、worktree 与 repo 的真实路径绑定关系，适合做排障和 demo。
          </p>
        </div>
      </section>

      <div className="grid gap-4 xl:grid-cols-[1.2fr_0.8fr]">
        <section className="rounded-[24px] border border-stone-200 bg-white p-4">
          <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Paths</div>
          <div className="space-y-3">
            <PathCard label="Repo Root" value={workspace.repoRoot} />
            <PathCard label="Tasks Root" value={workspace.tasksRoot} />
            <PathCard label="Context Root" value={workspace.contextRoot} />
            <PathCard label="Worktree Root" value={workspace.worktreeRoot} />
          </div>
        </section>

        <section className="rounded-[24px] border border-stone-200 bg-[#161a1f] p-4 text-stone-200">
          <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500">Repos Involved</div>
          <div className="space-y-2">
            {workspace.reposInvolved.map((repo) => (
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
          {tasks.map((task) => (
            <WorkspaceTaskRow key={task.id} taskID={task.id} />
          ))}
        </div>
      </section>
    </div>
  )
}

function WorkspaceTaskRow({ taskID }: { taskID: string }) {
  const [task, setTask] = useState<TaskRecord | null>(null)

  useEffect(() => {
    let cancelled = false
    void getTask(taskID)
      .then((detail) => {
        if (!cancelled) {
          setTask(detail)
        }
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [taskID])

  if (!task) {
    return null
  }

  const reposWithWorktree = task.repos.filter((repo) => repo.worktree)
  if (reposWithWorktree.length === 0) {
    return null
  }

  return (
    <>
      {reposWithWorktree.map((repo) => (
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
    </>
  )
}
