import type { TaskRecord } from '../api'

export function ActionPanel({ task }: { task: TaskRecord }) {
  const commands = [task.nextAction, ...task.repoNext].filter((item, index, array) => item && array.indexOf(item) === index)

  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/6">
      <div className="mb-4 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">终端命令</div>
      <div className="space-y-3">
        {commands.map((command, index) => (
          <div className="rounded-[18px] border border-stone-200 bg-stone-50 px-3 py-3 dark:border-white/10 dark:bg-white/5" key={`${command}-${index}`}>
            <div className="mb-2 text-[11px] uppercase tracking-[0.2em] text-stone-500 dark:text-stone-400">
              {index === 0 ? '当前建议' : `补充命令 ${index}`}
            </div>
            <div className="rounded-2xl border border-stone-200 bg-white px-3 py-3 font-mono text-xs leading-6 text-stone-800 dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-200">
              {command}
            </div>
          </div>
        ))}
      </div>
      <p className="mt-4 text-xs leading-5 text-stone-500 dark:text-stone-400">
        如果你更习惯在终端继续推进，可以直接使用这些命令。
      </p>
    </section>
  )
}
