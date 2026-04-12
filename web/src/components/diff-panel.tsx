import { useDeferredValue, useEffect, useMemo, useState } from 'react'
import type { RepoResult } from '../api'
import { diffLineTone, parseDiffFiles } from '../lib/diff'
import { FilterChip, KeyValue } from './ui-primitives'

export function DiffPanel({
  repos,
  selectedRepo,
  onSelectRepo,
}: {
  repos: RepoResult[]
  selectedRepo: string
  onSelectRepo: (repoId: string) => void
}) {
  const reposWithDiff = repos.filter((repo) => repo.diffSummary)
  const deferredSelectedRepo = useDeferredValue(selectedRepo)
  const activeRepo = reposWithDiff.find((repo) => repo.id === deferredSelectedRepo) ?? reposWithDiff[0]
  const diffFiles = useMemo(() => parseDiffFiles(activeRepo?.diffSummary?.patch ?? ''), [activeRepo?.diffSummary?.patch])
  const parsedFiles = useMemo(() => diffFiles.filter((file) => file.path !== 'commit'), [diffFiles])
  const [selectedFile, setSelectedFile] = useState('')

  useEffect(() => {
    setSelectedFile(parsedFiles[0]?.path ?? '')
  }, [activeRepo?.id, parsedFiles])

  const fileButtons = parsedFiles.length > 0 ? parsedFiles : diffFiles.filter((file) => file.path)
  const activeFile = fileButtons.find((file) => file.path === selectedFile) ?? fileButtons[0]
  const additionsValue = activeFile?.additions ?? activeRepo?.diffSummary?.additions ?? 0
  const deletionsValue = activeFile?.deletions ?? activeRepo?.diffSummary?.deletions ?? 0

  return (
    <section className="rounded-[24px] border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/6">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div>
          <div className="text-xs font-semibold uppercase tracking-[0.22em] text-stone-500 dark:text-stone-400">变更对比</div>
          <h4 className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-stone-950 dark:text-stone-50">提交差异回看</h4>
        </div>
        <div className="text-xs text-stone-500 dark:text-stone-400">按仓库查看本次提交的 patch</div>
      </div>

      {reposWithDiff.length === 0 ? (
        <div className="rounded-[18px] border border-dashed border-stone-300 bg-stone-50 px-4 py-6 text-sm leading-6 text-stone-500 dark:border-white/15 dark:bg-white/5 dark:text-stone-400">
          暂时还没有可查看的提交差异。生成提交后，这里会自动展示对应 patch。
        </div>
      ) : (
        <>
          <div className="mb-4 flex flex-wrap gap-2">
            {reposWithDiff.map((repo) => (
              <button
                className={`rounded-full border px-3 py-2 text-sm font-medium transition ${
                  activeRepo?.id === repo.id
                    ? 'border-stone-900 bg-stone-900 text-white shadow-sm dark:border-stone-100 dark:bg-stone-100 dark:text-stone-950'
                    : 'border-stone-200 bg-stone-50 text-stone-700 hover:border-stone-300 hover:bg-white dark:border-white/10 dark:bg-white/5 dark:text-stone-300 dark:hover:border-white/20 dark:hover:bg-white/10'
                }`}
                key={repo.id}
                onClick={() => onSelectRepo(repo.id)}
                type="button"
              >
                {repo.id}
              </button>
            ))}
          </div>

          {activeRepo?.diffSummary ? (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-2">
                <FilterChip label="文件数" value={`${fileButtons.length || activeRepo.diffSummary.files.length}`} />
                <FilterChip
                  label="+ / -"
                  value={`${additionsValue} / ${deletionsValue}`}
                />
              </div>

              <div className="rounded-[18px] border border-stone-200 bg-stone-50 px-3 py-3 dark:border-white/10 dark:bg-white/5">
                <div className="grid gap-3 md:grid-cols-2">
                  <KeyValue label="分支" mono value={activeRepo.diffSummary.branch || '-'} />
                  <KeyValue label="提交" mono value={activeRepo.diffSummary.commit || '-'} />
                </div>
              </div>

              <div className="rounded-[18px] border border-stone-200 bg-white px-3 py-3 dark:border-white/10 dark:bg-white/5">
                <div className="mb-2 text-[11px] uppercase tracking-[0.2em] text-stone-500 dark:text-stone-400">涉及文件</div>
                <div className="space-y-2">
                  {fileButtons.map((file) => (
                    <button
                      className={`block w-full rounded-xl border px-3 py-2 text-left font-mono text-xs transition ${
                        activeFile?.path === file.path
                          ? 'border-stone-900 bg-stone-900 text-white dark:border-stone-100 dark:bg-stone-100 dark:text-stone-950'
                          : 'border-stone-200 bg-stone-50 text-stone-800 hover:border-stone-300 hover:bg-white dark:border-white/10 dark:bg-stone-950/70 dark:text-stone-200 dark:hover:border-white/20 dark:hover:bg-stone-900'
                      }`}
                      key={file.path}
                      onClick={() => setSelectedFile(file.path)}
                      type="button"
                    >
                      {file.path}
                    </button>
                  ))}
                </div>
              </div>

              <div className="overflow-hidden rounded-[18px] border border-stone-200 bg-[#0d1014] shadow-[0_12px_30px_rgba(17,24,39,0.08)]">
                <div className="flex items-center justify-between border-b border-white/8 px-4 py-3 text-sm text-stone-300">
                  <div className="font-semibold text-stone-100">
                    {activeFile?.path ? `Diff · ${activeFile.path}` : '提交差异'}
                  </div>
                  <div className="font-mono text-xs text-stone-500">diffs/{activeRepo.id}.patch</div>
                </div>
                <div className="max-h-[420px] overflow-auto px-4 py-4 text-[12px] leading-6">
                  {(activeFile?.lines ?? activeRepo.diffSummary.patch.split('\n')).map((line, index) => (
                    <div className={diffLineTone(line)} key={`${index}-${line}`}>
                      <code>{line || ' '}</code>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          ) : null}
        </>
      )}
    </section>
  )
}
