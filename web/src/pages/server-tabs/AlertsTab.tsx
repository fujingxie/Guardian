import { useEffect, useState } from 'react'
import {
  Bar,
  BarChart,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Card } from '@/components/ui/Card'
import { Pill } from '@/components/ui/Pill'
import { EmptyState } from '@/components/ui/EmptyState'
import { ShieldCheckIcon } from '@/components/icons'
import { api } from '@/api/client'
import type { Alert, AlertStats, Severity, TimelinePoint } from '@/api/types'
import { formatDateTime } from '@/lib/format'
import { cn } from '@/lib/cn'

interface Props {
  serverId: string
  onCount?: (n: number) => void
}

type Filter = 'all' | 'unhandled'

const PIE_COLORS = [
  '#2F6FED', '#14B8A6', '#F59E0B', '#EF4444', '#8B5CF6',
  '#EC4899', '#06B6D4', '#84CC16', '#F97316', '#6366F1',
]

export function AlertsTab({ serverId, onCount }: Props) {
  const [alerts, setAlerts] = useState<Alert[] | null>(null)
  const [filter, setFilter] = useState<Filter>('all')
  const [typeFilter, setTypeFilter] = useState<string>('all')
  const [timeline, setTimeline] = useState<TimelinePoint[] | null>(null)
  const [stats, setStats] = useState<AlertStats | null>(null)

  useEffect(() => {
    api.getAlerts(serverId).then((d) => {
      setAlerts(d.alerts)
      onCount?.(d.alerts.filter((a) => !a.resolved).length)
    })
    api.getAlertsTimeline(serverId).then((d) => setTimeline(d.timeline))
    api.getAlertsStats(serverId).then(setStats)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serverId])

  const unhandled = (alerts ?? []).filter((a) => !a.resolved)
  const list = (filter === 'unhandled' ? unhandled : (alerts ?? [])).filter((a) =>
    typeFilter === 'all' ? true : a.kind === typeFilter
  )

  if (alerts === null) {
    return (
      <div className="py-16 text-center text-[13px] text-[var(--color-text-muted)]">
        正在加载告警…
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      {/* 图表行 */}
      <div className="grid grid-cols-2 gap-4">
        {/* 7 天告警堆叠柱状 */}
        <Card padded={false} className="p-5">
          <div className="mb-3 text-[14px] font-semibold">近 7 天告警分布</div>
          <div className="h-[220px]">
            {timeline === null ? (
              <div className="flex h-full items-center justify-center text-[12px] text-[var(--color-text-muted)]">加载中…</div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={timeline}>
                  <XAxis
                    dataKey="date"
                    tickFormatter={(d: string) => d.slice(5)}
                    tick={{ fontSize: 11, fill: 'var(--color-text-muted)' }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <YAxis
                    allowDecimals={false}
                    width={28}
                    axisLine={false}
                    tickLine={false}
                    tick={{ fontSize: 11, fill: 'var(--color-text-muted)' }}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'var(--color-surface)',
                      border: '1px solid var(--color-border)',
                      borderRadius: 8,
                      fontSize: 12,
                      boxShadow: 'var(--shadow-sm)',
                    }}
                    labelFormatter={(d) => `${d}`}
                  />
                  <Legend
                    iconType="circle"
                    iconSize={8}
                    wrapperStyle={{ fontSize: 12 }}
                  />
                  <Bar dataKey="high" name="高危" stackId="a" fill="#EF4444" radius={[0, 0, 0, 0]} />
                  <Bar dataKey="medium" name="注意" stackId="a" fill="#F59E0B" radius={[0, 0, 0, 0]} />
                  <Bar dataKey="info" name="提示" stackId="a" fill="#2F6FED" radius={[2, 2, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </Card>

        {/* 30 天攻击源国家饼图 */}
        <Card padded={false} className="p-5">
          <div className="mb-3 text-[14px] font-semibold">近 30 天攻击源国家</div>
          <div className="h-[220px]">
            {stats === null ? (
              <div className="flex h-full items-center justify-center text-[12px] text-[var(--color-text-muted)]">加载中…</div>
            ) : stats.countries && stats.countries.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={stats.countries}
                    dataKey="count"
                    nameKey="country"
                    cx="50%"
                    cy="50%"
                    outerRadius={80}
                    innerRadius={40}
                    paddingAngle={2}
                    label={(props: any) => {
                      const { name, percent } = props
                      return `${name} ${(percent * 100).toFixed(0)}%`
                    }}
                    labelLine={false}
                  >
                    {stats.countries.map((_entry, index) => (
                      <Cell key={`cell-${index}`} fill={PIE_COLORS[index % PIE_COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'var(--color-surface)',
                      border: '1px solid var(--color-border)',
                      borderRadius: 8,
                      fontSize: 12,
                      boxShadow: 'var(--shadow-sm)',
                    }}
                    formatter={(value: any, name: any) => [`${value} 次`, name]}
                  />
                </PieChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-full items-center justify-center text-[12px] text-[var(--color-text-muted)]">
                暂无攻击源数据
              </div>
            )}
          </div>
        </Card>
      </div>

      {/* Top 10 攻击 IP 表格 */}
      {stats && stats.topIPs && stats.topIPs.length > 0 && (
        <Card padded={false} className="overflow-hidden">
          <div className="border-b border-[var(--color-divider)] px-5 py-3 text-[14px] font-semibold">
            近 30 天 Top 10 攻击源 IP
          </div>
          <div className="grid grid-cols-[1fr_1fr_100px] gap-2 border-b border-[var(--color-divider)] bg-[var(--color-surface-soft)] px-5 py-2 text-[11px] font-semibold uppercase tracking-[.04em] text-[var(--color-text-muted)]">
            <span>IP</span>
            <span>国家</span>
            <span className="text-right">次数</span>
          </div>
          {stats.topIPs.map((ip, i) => (
            <div
              key={ip.ip}
              className={cn(
                'grid grid-cols-[1fr_1fr_100px] gap-2 px-5 py-2.5 text-[13px]',
                i !== stats.topIPs.length - 1 && 'border-b border-[var(--color-divider)]',
              )}
            >
              <span className="mono font-medium">{ip.ip}</span>
              <span className="text-[var(--color-text-2)]">{ip.country}</span>
              <span className="tabular-num text-right font-semibold">{ip.count}</span>
            </div>
          ))}
        </Card>
      )}

      {/* 告警列表 */}
      {alerts.length === 0 ? (
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
      ) : (
        <>
          <div className="flex items-center justify-between gap-1.5">
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

            <div className="flex items-center gap-2">
              <span className="text-[12px] text-[var(--color-text-muted)] font-medium">告警类型:</span>
              <select
                value={typeFilter}
                onChange={(e) => setTypeFilter(e.target.value)}
                className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2.5 py-1.5 text-[13px] font-medium text-[var(--color-text-2)] shadow-sm outline-none focus:border-[var(--color-primary)]"
              >
                <option value="all">全部类型</option>
                <option value="bruteforce">恶意登录</option>
                <option value="port_scan">端口扫描</option>
                <option value="new_login">新登录提醒</option>
                <option value="metric_threshold">指标超限</option>
                <option value="offline">服务器离线</option>
                <option value="unknown">系统告警</option>
              </select>
            </div>
          </div>
          <Card padded={false} className="overflow-hidden">
            {list.map((a, i) => (
              <AlertRow key={a.id} alert={a} last={i === list.length - 1} />
            ))}
          </Card>
        </>
      )}
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
            {alert.country && <span className="ml-1.5 text-[var(--color-text-2)]">({alert.country})</span>}
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
