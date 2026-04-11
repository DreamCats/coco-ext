import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import type { RepoResult, TaskStatus } from '../api'

export function MetricCard({
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

export function TopNavItem({
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

export function FilterChip({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-white px-3 py-3">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500">{label}</div>
      <div className="mt-1 text-lg font-semibold text-stone-950">{value}</div>
    </div>
  )
}

export function StatusBadge({ status }: { status: TaskStatus }) {
  const tone =
    status === 'coded'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : status === 'partially_coded'
        ? 'border-orange-200 bg-orange-50 text-orange-700'
        : status === 'planned'
          ? 'border-amber-200 bg-amber-50 text-amber-700'
          : status === 'archived'
            ? 'border-sky-200 bg-sky-50 text-sky-700'
            : status === 'failed'
              ? 'border-rose-200 bg-rose-50 text-rose-700'
              : 'border-stone-200 bg-stone-100 text-stone-700'

  return <span className={`rounded-full border px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.2em] ${tone}`}>{status}</span>
}

export function RepoStatusBadge({ status }: { status: RepoResult['status'] }) {
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

export function TimelineCard({
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

export function CompactField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[18px] border border-white/8 bg-white/4 px-3 py-3">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-400">{label}</div>
      <div className="mt-2 text-sm text-white">{value}</div>
    </div>
  )
}

export function KeyValue({ label, mono, value }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-[18px] border border-stone-200 bg-stone-50 px-3 py-3">
      <div className="text-[11px] uppercase tracking-[0.2em] text-stone-500">{label}</div>
      <div className={`mt-2 text-sm text-stone-900 ${mono ? 'font-mono text-xs' : ''}`}>{value}</div>
    </div>
  )
}

export function PathCard({ label, value }: { label: string; value: string }) {
  return <KeyValue label={label} mono value={value} />
}

export function MiniMeta({ label, value }: { label: string; value: string }) {
  return <KeyValue label={label} mono value={value} />
}

export function PanelMessage({ children }: { children: ReactNode }) {
  return (
    <section className="flex min-h-[720px] items-center justify-center rounded-[24px] border border-dashed border-stone-300 bg-stone-50 p-8 text-center text-stone-500">
      {children}
    </section>
  )
}
