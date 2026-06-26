import { useEffect, useState } from 'react'
import {
  Area,
  AreaChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Card } from '@/components/ui/Card'
import { Pill } from '@/components/ui/Pill'
import { Meter } from '@/components/ui/Meter'
import {
  CpuIcon,
  HardDriveIcon,
  MemoryIcon,
  NetworkIcon,
} from '@/components/icons'
import { api } from '@/api/client'
import type { MetricPoint, Server, Settings } from '@/api/types'
import { cn } from '@/lib/cn'

interface Props {
  server: Server
}

export function OverviewTab({ server }: Props) {
  const [points, setPoints] = useState<MetricPoint[] | null>(null)
  const [settings, setSettings] = useState<Settings | null>(null)
  useEffect(() => {
    api.getMetrics(server.id).then((d) => setPoints(d.points))
    api.getSettings().then(setSettings)
  }, [server.id])

  const offline = server.status === 'offline'

  const cpuThreshold = settings?.notify?.cpuPctThreshold ?? 85
  const memThreshold = settings?.notify?.memPctThreshold ?? 95
  const diskThreshold = settings?.notify?.diskPctThreshold ?? 95

  return (
    <div className={cn('flex flex-col gap-6', offline && 'opacity-70')}>
      <div className="grid grid-cols-4 gap-4">
        <MetricCard
          icon={<CpuIcon className="h-4 w-4" />}
          label="CPU"
          value={server.metrics.cpu}
          unit="%"
          threshold={cpuThreshold}
        />
        <MetricCard
          icon={<MemoryIcon className="h-4 w-4" />}
          label="内存"
          value={server.metrics.mem}
          unit="%"
          threshold={memThreshold}
        />
        <MetricCard
          icon={<HardDriveIcon className="h-4 w-4" />}
          label="磁盘"
          value={server.metrics.disk}
          unit="%"
          threshold={diskThreshold}
        />
        <NetCard
          netUp={server.metrics.netUp}
          netDown={server.metrics.netDown}
        />
      </div>

      {server.system && (
        <Card padded={false} className="p-5">
          <div className="grid grid-cols-4 gap-6">
            <SysInfo label="发行版" value={server.system.distro} />
            <SysInfo label="内核" value={server.system.kernel} />
            <SysInfo label="运行时长" value={server.system.uptime} />
            <SysInfo label="Agent 版本" value={server.system.agent} />
          </div>
        </Card>
      )}

      <div className="grid grid-cols-2 gap-4">
        <TrendCard
          title="CPU 使用率 · 24 小时"
          points={points}
          dataKey="cpu"
          stroke="var(--color-primary)"
          fill="rgba(47,111,237,0.18)"
          threshold={cpuThreshold}
        />
        <TrendCard
          title="内存使用率 · 24 小时"
          points={points}
          dataKey="mem"
          stroke="var(--color-teal-deep)"
          fill="rgba(20,184,166,0.18)"
          threshold={memThreshold}
        />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <TrendCard
          title="磁盘使用率 · 24 小时"
          points={points}
          dataKey="disk"
          stroke="#F59E0B"
          fill="rgba(245,158,11,0.18)"
          threshold={diskThreshold}
        />
      </div>
    </div>
  )
}

function MetricCard({
  icon,
  label,
  value,
  unit,
  threshold,
}: {
  icon: React.ReactNode
  label: string
  value: number
  unit: string
  threshold?: number
}) {
  const high = threshold ? value >= threshold : value >= 80
  return (
    <Card padded={false} className="p-5">
      <div className="mb-3 flex items-center justify-between text-[13px] text-[var(--color-text-2)]">
        <span className="flex items-center gap-1.5">
          {icon}
          {label}
        </span>
        <Pill tone={high ? 'warning' : 'success'}>
          {high ? '偏高' : '正常'}
        </Pill>
      </div>
      <div
        className={cn(
          'flex items-baseline gap-0.5 text-[30px] font-semibold leading-none tabular-num',
          high && 'text-[var(--color-warning-deep)]',
        )}
      >
        {Math.round(value)}
        <span className="text-[16px] font-medium text-[var(--color-text-2)]">
          {unit}
        </span>
      </div>
      <div className="mt-4">
        <Meter value={value} showValue={false} />
      </div>
    </Card>
  )
}

function NetCard({ netUp, netDown }: { netUp: number; netDown: number }) {
  return (
    <Card padded={false} className="p-5">
      <div className="mb-3 flex items-center justify-between text-[13px] text-[var(--color-text-2)]">
        <span className="flex items-center gap-1.5">
          <NetworkIcon className="h-4 w-4" />
          网络
        </span>
        <Pill tone="success">正常</Pill>
      </div>
      <div className="flex flex-col gap-2.5">
        <div className="flex items-baseline justify-between">
          <span className="text-[12px] text-[var(--color-text-2)]">↑ 出流量</span>
          <span className="text-[22px] font-semibold tabular-num leading-none">
            {netUp.toFixed(1)}
            <span className="ml-1 text-[12px] font-medium text-[var(--color-text-2)]">
              MB/s
            </span>
          </span>
        </div>
        <div className="flex items-baseline justify-between">
          <span className="text-[12px] text-[var(--color-text-2)]">↓ 入流量</span>
          <span className="text-[22px] font-semibold tabular-num leading-none">
            {netDown.toFixed(1)}
            <span className="ml-1 text-[12px] font-medium text-[var(--color-text-2)]">
              MB/s
            </span>
          </span>
        </div>
      </div>
    </Card>
  )
}

function SysInfo({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[12px] text-[var(--color-text-muted)]">{label}</div>
      <div className="mono mt-1 text-[14px] font-medium">{value}</div>
    </div>
  )
}

function TrendCard({
  title,
  points,
  dataKey,
  stroke,
  fill,
  threshold,
}: {
  title: string
  points: MetricPoint[] | null
  dataKey: 'cpu' | 'mem' | 'disk'
  stroke: string
  fill: string
  threshold?: number
}) {
  const peak =
    points && points.length
      ? Math.max(...points.map((p) => p[dataKey]))
      : 0
  const avg =
    points && points.length
      ? Math.round(points.reduce((a, p) => a + p[dataKey], 0) / points.length)
      : 0
  return (
    <Card padded={false} className="p-5">
      <div className="mb-2 flex items-center justify-between">
        <div className="text-[14px] font-semibold">{title}</div>
        <div className="text-[12px] text-[var(--color-text-muted)]">
          峰值 <span className="tabular-num font-medium text-[var(--color-text-2)]">{peak}%</span>{' '}
          · 均值 <span className="tabular-num font-medium text-[var(--color-text-2)]">{avg}%</span>
        </div>
      </div>
      <div className="h-[200px]">
        {points === null ? (
          <div className="flex h-full items-center justify-center text-[12px] text-[var(--color-text-muted)]">
            正在加载…
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={points}>
              <defs>
                <linearGradient id={`grad-${dataKey}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={fill} stopOpacity={1} />
                  <stop offset="100%" stopColor={fill} stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="ts"
                hide
              />
              <YAxis
                domain={[0, 100]}
                tickCount={5}
                width={28}
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 11, fill: 'var(--color-text-muted)' }}
              />
              {threshold != null && (
                <ReferenceLine
                  y={threshold}
                  stroke="#EF4444"
                  strokeDasharray="6 3"
                  strokeWidth={1.5}
                  label={{
                    value: `${threshold}%`,
                    position: 'right',
                    fill: '#EF4444',
                    fontSize: 10,
                    fontWeight: 600,
                  }}
                />
              )}
              <Tooltip
                cursor={{
                  stroke: 'var(--color-border)',
                  strokeDasharray: '3 3',
                }}
                contentStyle={{
                  backgroundColor: 'var(--color-surface)',
                  border: '1px solid var(--color-border)',
                  borderRadius: 8,
                  fontSize: 12,
                  boxShadow: 'var(--shadow-sm)',
                }}
                labelFormatter={(ts) =>
                  new Date(ts as string).toLocaleTimeString('zh-CN', {
                    hour: '2-digit',
                    minute: '2-digit',
                  })
                }
                formatter={(v) => [`${Math.round(Number(v))}%`, title]}
              />
              <Area
                type="monotone"
                dataKey={dataKey}
                stroke={stroke}
                strokeWidth={2}
                fill={`url(#grad-${dataKey})`}
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>
    </Card>
  )
}
