import type { TaskRecord } from '../api'

export function TaskPrimaryAction({
  task,
  actionError,
  actionBusy,
  batchCodeStarting,
  canArchiveCode,
  canResetCode,
  canStartCode,
  canStartPlan,
  canStartRemainingCode,
  codeActionLabel,
  codeStarting,
  planActionLabel,
  planStarting,
  remainingReposCount,
  resetting,
  archiving,
  onArchive,
  onReset,
  onStartCode,
  onStartPlan,
  onStartRemainingCode,
}: {
  task: TaskRecord
  actionError: string
  actionBusy: boolean
  batchCodeStarting: boolean
  canArchiveCode: boolean
  canResetCode: boolean
  canStartCode: boolean
  canStartPlan: boolean
  canStartRemainingCode: boolean
  codeActionLabel: string
  codeStarting: boolean
  planActionLabel: string
  planStarting: boolean
  remainingReposCount: number
  resetting: boolean
  archiving: boolean
  onArchive: () => void
  onReset: () => void
  onStartCode: () => void
  onStartPlan: () => void
  onStartRemainingCode: () => void
}) {
  const repoCount = task.repos.length
  const codedCount = task.repos.filter((repo) => repo.status === 'coded' || repo.status === 'archived').length
  const failedCount = task.repos.filter((repo) => repo.status === 'failed').length
  const runningCount = task.repos.filter((repo) => repo.status === 'coding').length

  return (
    <section className="rounded-[24px] border border-emerald-300/20 bg-emerald-400/10 p-5">
      <div className="text-xs font-semibold uppercase tracking-[0.22em] text-emerald-200">主行动区</div>
      <div className="mt-3 text-[26px] font-semibold tracking-[-0.05em] text-white">{primaryHeadline(task)}</div>
      <p className="mt-3 text-sm leading-6 text-emerald-100/85">{primaryNarrative(task)}</p>

      <div className="mt-4 grid gap-2 sm:grid-cols-3">
        <MiniStat label="仓库总数" value={`${repoCount}`} />
        <MiniStat label="已完成" value={`${codedCount}`} />
        <MiniStat label="处理中断" value={`${failedCount + runningCount}`} />
      </div>

      {task.status === 'initialized' ? (
        <NoticeBox tone="amber">需求正在整理中。若停留时间过长，可先查看 `refine.log`。</NoticeBox>
      ) : null}
      {task.status === 'planning' ? (
        <NoticeBox tone="sky">正在分析代码并生成方案。若停留时间过长，可先查看 `plan.log`。</NoticeBox>
      ) : null}
      {task.status === 'coding' ? (
        <NoticeBox tone="emerald">后台正在生成实现并验证结果。若停留时间过长，可先查看 `code.log`。</NoticeBox>
      ) : null}
      {task.repos.length > 1 && canStartRemainingCode ? (
        <NoticeBox tone="amber">这是一条多仓任务。建议优先一键推进剩余仓库，再到下方逐个处理例外情况。</NoticeBox>
      ) : null}
      {actionError ? <NoticeBox tone="rose">{actionError}</NoticeBox> : null}

      <div className="mt-5 flex flex-wrap gap-3">
        {canStartRemainingCode ? (
          <>
            <PrimaryButton disabled={actionBusy} onClick={onStartRemainingCode}>
              {batchCodeStarting ? '批量推进中...' : `依次推进剩余仓库 (${remainingReposCount})`}
            </PrimaryButton>
            <InlineHint>会按仓库顺序逐个执行，某个仓库失败后立即停止。</InlineHint>
          </>
        ) : null}

        {canStartCode ? (
          <>
            <PrimaryButton disabled={actionBusy} onClick={onStartCode}>
              {codeStarting ? '实现进行中...' : codeActionLabel}
            </PrimaryButton>
            <InlineHint>会创建隔离工作区、生成改动并尝试完成构建验证。</InlineHint>
          </>
        ) : null}

        {canResetCode ? (
          <>
            <SecondaryButton disabled={actionBusy} onClick={onReset} tone="rose">
              {resetting ? '回退中...' : '回退实现'}
            </SecondaryButton>
            <InlineHint>会删除本次生成的分支、worktree、diff 与结果记录。</InlineHint>
          </>
        ) : null}

        {canArchiveCode ? (
          <>
            <SecondaryButton disabled={actionBusy} onClick={onArchive} tone="sky">
              {archiving ? '归档中...' : '归档任务'}
            </SecondaryButton>
            <InlineHint>会清理分支和工作区，并把任务标记为已归档。</InlineHint>
          </>
        ) : null}

        {canStartPlan ? (
          <>
            <SecondaryButton disabled={actionBusy} onClick={onStartPlan} tone="neutral">
              {planStarting ? '方案生成中...' : planActionLabel}
            </SecondaryButton>
            <InlineHint>
              {task.status === 'planned' ? '会重新分析代码，并覆盖 `design.md` / `plan.md`。' : '会在后台生成 `design.md` / `plan.md`。'}
            </InlineHint>
          </>
        ) : null}
      </div>

      <div className="mt-5 rounded-2xl border border-emerald-200/20 bg-stone-950/50 px-4 py-3 font-mono text-sm text-emerald-100">
        {task.nextAction}
      </div>
    </section>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[18px] border border-white/8 bg-white/4 px-4 py-3">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-400">{label}</div>
      <div className="mt-2 text-lg font-semibold text-white">{value}</div>
    </div>
  )
}

function NoticeBox({
  children,
  tone,
}: {
  children: string
  tone: 'amber' | 'emerald' | 'rose' | 'sky'
}) {
  const toneClass =
    tone === 'amber'
      ? 'mt-4 rounded-2xl border border-amber-300/20 bg-amber-400/10 px-4 py-3 text-sm leading-6 text-amber-100'
      : tone === 'sky'
        ? 'mt-4 rounded-2xl border border-sky-300/20 bg-sky-400/10 px-4 py-3 text-sm leading-6 text-sky-100'
        : tone === 'rose'
          ? 'mt-4 rounded-2xl border border-rose-300/20 bg-rose-400/10 px-4 py-3 text-sm leading-6 text-rose-100'
          : 'mt-4 rounded-2xl border border-emerald-300/20 bg-emerald-400/10 px-4 py-3 text-sm leading-6 text-emerald-100'

  return <div className={toneClass}>{children}</div>
}

function PrimaryButton({
  children,
  disabled,
  onClick,
}: {
  children: string
  disabled?: boolean
  onClick: () => void
}) {
  return (
    <button
      className="rounded-2xl border border-emerald-200/30 bg-emerald-500/25 px-4 py-3 text-sm font-semibold text-emerald-50 transition hover:border-emerald-100/40 hover:bg-emerald-500/35 disabled:cursor-not-allowed disabled:opacity-60"
      disabled={disabled}
      onClick={onClick}
      type="button"
    >
      {children}
    </button>
  )
}

function SecondaryButton({
  children,
  disabled,
  onClick,
  tone,
}: {
  children: string
  disabled?: boolean
  onClick: () => void
  tone: 'neutral' | 'rose' | 'sky'
}) {
  const toneClass =
    tone === 'rose'
      ? 'border-rose-200/30 bg-rose-400/15 text-rose-50 hover:border-rose-100/40 hover:bg-rose-400/25'
      : tone === 'sky'
        ? 'border-sky-200/30 bg-sky-400/15 text-sky-50 hover:border-sky-100/40 hover:bg-sky-400/25'
        : 'border-white/20 bg-white/8 text-white hover:border-white/30 hover:bg-white/12'

  return (
    <button
      className={`rounded-2xl border px-4 py-3 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-60 ${toneClass}`}
      disabled={disabled}
      onClick={onClick}
      type="button"
    >
      {children}
    </button>
  )
}

function InlineHint({ children }: { children: string }) {
  return <span className="self-center text-xs text-emerald-100/80">{children}</span>
}

function primaryHeadline(task: TaskRecord) {
  switch (task.status) {
    case 'coded':
      return '结果已产出，准备收尾'
    case 'partially_coded':
      return '还有仓库待继续推进'
    case 'coding':
      return '实现正在推进'
    case 'planning':
      return '方案正在生成'
    case 'planned':
      return task.repos.length > 1 ? '方案已完成，等待批量实现' : '方案已完成，准备进入实现'
    case 'failed':
      return '这次推进中断了'
    case 'refined':
      return '需求已整理，等待生成方案'
    default:
      return '继续推进当前任务'
  }
}

function primaryNarrative(task: TaskRecord) {
  const repoCount = task.repos.length
  const codedCount = task.repos.filter((repo) => repo.status === 'coded' || repo.status === 'archived').length
  const failedCount = task.repos.filter((repo) => repo.status === 'failed').length

  if (task.status === 'partially_coded') {
    return `${codedCount} 个仓库已经完成，仍有 ${Math.max(repoCount - codedCount, 0)} 个仓库需要继续处理。`
  }
  if (task.status === 'failed' && repoCount > 1) {
    return `${failedCount} 个仓库在推进中失败，建议先查看日志，再决定重试还是回退。`
  }
  if (task.status === 'coded') {
    return '实现结果已经准备好，接下来更适合确认产物、查看 Diff，并决定是否归档。'
  }
  if (task.status === 'planned') {
    return repoCount > 1 ? '方案已经生成完毕，现在更适合按仓库顺序推进实现。' : '方案已经生成完毕，现在可以直接开始实现。'
  }
  if (task.status === 'planning') {
    return '系统正在调研代码和生成方案，完成后会自动进入下一步可执行状态。'
  }
  if (task.status === 'coding') {
    return '后台正在生成实现并验证结果。你可以先查看日志，确认当前执行是否正常。'
  }
  if (task.status === 'refined') {
    return '需求已经整理成可执行任务，下一步最值得做的是生成方案。'
  }
  return '你可以在这里集中推进任务、查看结果，并决定接下来的动作。'
}
