import { useEffect, useState } from 'react'
import { Card } from '@/components/ui/Card'
import { Pill } from '@/components/ui/Pill'
import { EmptyState } from '@/components/ui/EmptyState'
import { ShieldCheckIcon } from '@/components/icons'
import { api } from '@/api/client'
import type { Alert, Severity } from '@/api/types'
import { formatDateTime } from '@/lib/format'
import { cn } from '@/lib/cn'

interface Props {
  serverId: string
  onCount?: (n: number) => void
}

type Filter = 'all' | 'unhandled'

export function AlertsTab({ serverId, onCount }: Props) {
  const [alerts, setAlerts] = useState<Alert[] | null>(null)
  const [filter, setFilter] = useState<Filter>('all')

  useEffect(() => {
    api.getAlerts(serverId).then((d) => {
      setAlerts(d.alerts)
      onCount?.(d.alerts.filter((a) => !a.resolved).length)
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serverId])

  const unhandled = (alerts ?? []).filter((a) => !a.resolved)
  const list = filter === 'unhandled' ? unhandled : (alerts ?? [])

  if (alerts === null) {
    return (
      <div className="py-16 text-center text-[13px] text-[var(--color-text-muted)]">
        正在加载告警…
      </div>
    )
  }

  if (alerts.length === 0) {
    return (
      <Card padded={false} className="p-2">
        <EmptyState
          tone="teal"
          icon={<ShieldCheckIcon className="h-10 w-10" />}
          title="一切平静"
          description={
            <>
              过去 24 小时这台机器没有任何告警，
              <br />
              Guardian 正在持续替你盯着。
            </>
          }
        />
      </Card>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-1.5">
        <FilterChip active={filter === 'all'} onClick={() => setFilter('all')}>
          全部 · {alerts.length}
        </FilterChip>
        <FilterChip
          active={filter === 'unhandled'}
          onClick={() => setFilter('unhandled')}
        >
          未处理 · {unhandled.length}
        </FilterChip>
      </div>
      <Card padded={false} className="overflow-hidden">
        {list.map((a, i) => (
          <AlertRow key={a.id} alert={a} last={i === list.length - 1} />
        ))}
      </Card>
    </div>
  )
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active?: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'cursor-pointer rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors',
        active
          ? 'bg-[var(--color-primary-soft)] text-[var(--color-primary)]'
          : 'text-[var(--color-text-2)] hover:bg-[#F2F4F7]',
      )}
    >
      {children}
    </button>
  )
}

function severityTone(s: Severity) {
  return s === 'high' ? 'danger' : s === 'medium' ? 'warning' : 'neutral'
}

function severityLabel(s: Severity) {
  return s === 'high' ? '高危' : s === 'medium' ? '注意' : '提示'
}

function severityBorder(s: Severity) {
  return s === 'high'
    ? 'border-l-[var(--color-danger)]'
    : s === 'medium'
      ? 'border-l-[var(--color-warning)]'
      : 'border-l-[var(--color-text-faint)]'
}

function AlertRow({ alert, last }: { alert: Alert; last?: boolean }) {
  return (
    <div
      className={cn(
        'flex items-start gap-4 border-l-[3px] px-5 py-4 transition-colors',
        !alert.read && !alert.resolved
          ? severityBorder(alert.severity)
          : 'border-l-transparent',
        !last && 'border-b border-[var(--color-divider)]',
        alert.resolved && 'opacity-60',
        alert.read && !alert.resolved && 'opacity-90',
      )}
    >
      <div className="mono w-[88px] flex-shrink-0 text-[12.5px] text-[var(--color-text-muted)]">
        {formatDateTime(alert.ts)}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-[14px] font-semibold">{alert.title}</span>
          {!alert.read && !alert.resolved && (
            <span className="inline-block h-2 w-2 rounded-full bg-[var(--color-primary)]" />
          )}
        </div>
        {alert.sourceIp && (
          <div className="mono mt-0.5 text-[12px] text-[var(--color-text-muted)]">
            来源 IP {alert.sourceIp}
          </div>
        )}
        <div className="mt-1.5 text-[13px] leading-relaxed text-[var(--color-text-2)]">
          {alert.message}
        </div>
      </div>
      <div className="flex flex-shrink-0 flex-col items-end gap-1.5">
        <Pill tone={severityTone(alert.severity)}>
          {severityLabel(alert.severity)}
        </Pill>
        {alert.resolved && (
          <Pill tone="success">已解决</Pill>
        )}
      </div>
    </div>
  )
}
