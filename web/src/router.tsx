import {
  Navigate,
  Outlet,
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
  useLocation,
} from '@tanstack/react-router'
import { AppDataProvider, useAppData } from './hooks/use-app-data'
import { MetricCard, TopNavItem } from './components/ui-primitives'
import { TasksIndexPage, TasksLayout, TaskDetailPage } from './routes/tasks'
import { WorkspacePage } from './routes/workspace'

function AppShell() {
  const location = useLocation()
  const { tasks, workspace, loading, error } = useAppData()
  const codedCount = tasks.filter((task) => task.status === 'coded' || task.status === 'partially_coded').length

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,_rgba(34,197,94,0.12),_transparent_32%),radial-gradient(circle_at_top_right,_rgba(249,115,22,0.15),_transparent_28%),linear-gradient(180deg,_#f6f4ee_0%,_#f2efe7_52%,_#ede7dc_100%)] text-stone-950">
      <div className="mx-auto flex min-h-screen max-w-[1600px] flex-col px-4 py-4 md:px-6 lg:px-8">
        <header className="mb-4 rounded-[28px] border border-stone-200/80 bg-white/80 px-5 py-4 shadow-[0_1px_0_rgba(255,255,255,0.7)_inset,0_24px_50px_rgba(54,47,35,0.06)] backdrop-blur">
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-700">
                  Local Readonly UI
                </div>
                <h1 className="text-3xl font-semibold tracking-[-0.04em] text-stone-950">
                  coco-ext PRD Workflow Console
                </h1>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-stone-600">
                  第一阶段接入真实 task 数据，只读展示全局 task、多仓 repo scope、产物文档和 code 结果。
                </p>
                {error ? <p className="mt-3 text-sm text-rose-600">{error}</p> : null}
              </div>
              <div className="grid gap-3 text-sm text-stone-600 sm:grid-cols-3">
                <MetricCard label="Active Tasks" value={loading ? '...' : `${tasks.length}`} tone="emerald" />
                <MetricCard label="Coded Ready" value={loading ? '...' : `${codedCount}`} tone="amber" />
                <MetricCard
                  label="Repos Involved"
                  value={loading ? '...' : `${workspace?.reposInvolved.length ?? 0}`}
                  tone="sky"
                />
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
  return (
    <AppDataProvider>
      <RouterProvider router={router} />
    </AppDataProvider>
  )
}
