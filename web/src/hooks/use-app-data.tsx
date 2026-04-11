import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { getWorkspace, listTasks, type TaskListItem, type WorkspaceSummary } from '../api'

type AppDataContextValue = {
  tasks: TaskListItem[]
  workspace: WorkspaceSummary | null
  loading: boolean
  error: string
}

const AppDataContext = createContext<AppDataContextValue>({
  tasks: [],
  workspace: null,
  loading: true,
  error: '',
})

export function AppDataProvider({ children }: { children: ReactNode }) {
  const [tasks, setTasks] = useState<TaskListItem[]>([])
  const [workspace, setWorkspace] = useState<WorkspaceSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        setLoading(true)
        const [taskItems, workspaceSummary] = await Promise.all([listTasks(), getWorkspace()])
        if (cancelled) {
          return
        }
        setTasks(taskItems)
        setWorkspace(workspaceSummary)
        setError('')
      } catch (err) {
        if (cancelled) {
          return
        }
        setError(err instanceof Error ? err.message : '加载数据失败')
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    void load()
    return () => {
      cancelled = true
    }
  }, [])

  const value = useMemo(
    () => ({ tasks, workspace, loading, error }),
    [tasks, workspace, loading, error],
  )

  return <AppDataContext.Provider value={value}>{children}</AppDataContext.Provider>
}

export function useAppData() {
  return useContext(AppDataContext)
}
