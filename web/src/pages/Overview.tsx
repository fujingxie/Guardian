import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AppShell } from '@/components/shell/AppShell'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Meter } from '@/components/ui/Meter'
import { StatusDot } from '@/components/ui/StatusDot'
import { SecurityBadge } from '@/components/ui/SecurityBadge'
import { EmptyState } from '@/components/ui/EmptyState'
import { ChevronRightIcon, PlusIcon, ServerStackIcon } from '@/components/icons'
import { api } from '@/api/client'
import type { Server, SummaryStats } from '@/api/types'
import { cn } from '@/lib/cn'
import { AddServerModal } from '@/modals/AddServerModal'
import { formatTime } from '@/lib/format'

type Filter = 'all' | 'attention'

export default function OverviewPage() {
  const navigate = useNavigate()
  const [servers, setServers] = useState<Server[] | null>(null)
  const [summary, setSummary] = useState<SummaryStats | null>(null)
  const [filter, setFilter] = useState<Filter>('all')
  const [addOpen, setAddOpen] = useState(false)
  const [updatedAt, setUpdatedAt] = useState(formatTime(new Date().toISOString()))

  async function load() {
    const data = await api.listServers()
    setServers(data.servers)
    setSummary(data.summary)
    setUpdatedAt(formatTime(new Date().toISOString()))
  }

  useEffect(() => {
    load()
  }, [])

  const list =
    servers?.filter((s) =>
      filter === 'attention'
        ? s.security !== 'protected' || s.status === 'offline'
        : true,
    ) ?? []

  return (
    <AppShell>
      <div className="mx-auto max-w-[1080px] px-8 py-8">
        <div className="mb-6 flex items-end justify-between">
          <div>
            <h1 className="m-0 text-[28px] font-semibold tracking-[-0.01em]">
              我的服务器
            </h1>
            {summary && (
              <p className="mt-2 text-[14px] text-[var(--color-text-2)]">
                {summary.total} 台 · {summary.online} 在线 · 今日拦截{' '}
                {summary.todayBlocked} 次 · 最近更新 {updatedAt}
              </p>
            )}
          </div>
          <Button onClick={() => setAddOpen(true)}>
            <PlusIcon className="h-4 w-4" />
            添加服务器
          </Button>
        </div>

        {/* 汇总指标卡 */}
        {summary && (
          <div className="mb-6 grid grid-cols-4 gap-4">
            <SummaryCard
              label="服务器总数"
              value={summary.total}
              hint={`${summary.online} 在线 · ${summary.offline} 离线`}
            />
            <SummaryCard
              label="已受保护"
              value={summary.protected}
              hint={`占在线服务器 ${Math.round((summary.protected / Math.max(1, summary.online)) * 100)}%`}
              tone="teal"
            />
            <SummaryCard
              label="待处理项"
              value={summary.pending}
              hint={`分布在 ${summary.pendingServers} 台服务器`}
              tone="warning"
            />
            <SummaryCard
              label="今日拦截"
              value={summary.todayBlocked}
              hint={`较昨日 +${summary.yesterdayDelta} 次`}
              tone="primary"
            />
          </div>
        )}

        {/* 列表卡 */}
        <Card padded={false} className="overflow-hidden">
          <div className="flex items-center justify-between border-b border-[var(--color-divider)] px-6 py-4">
            <div className="text-[16px] font-semibold">服务器列表</div>
            <div className="flex gap-1.5">
              <FilterChip
                active={filter === 'all'}
                onClick={() => setFilter('all')}
              >
                全部
              </FilterChip>
              <FilterChip
                active={filter === 'attention'}
                onClick={() => setFilter('attention')}
              >
                需关注
              </FilterChip>
            </div>
          </div>

          {servers === null ? (
            <div className="px-6 py-16 text-center text-[13px] text-[var(--color-text-muted)]">
              正在加载…
            </div>
          ) : servers.length === 0 ? (
            <EmptyState
              tone="primary"
              icon={<ServerStackIcon className="h-10 w-10" />}
              title="还没有服务器"
              description={
                <>
                  在自己服务器上运行 Guardian 安装命令，
                  <br />
                  它会自动出现在这里 —— 不需要打开任何端口。
                </>
              }
              action={
                <Button onClick={() => setAddOpen(true)}>
                  <PlusIcon className="h-4 w-4" />
                  添加第一台服务器
                </Button>
              }
            />
          ) : list.length === 0 ? (
            <div className="px-6 py-16 text-center text-[14px] text-[var(--color-text-2)]">
              全部服务器都在受保护状态 🎉
            </div>
          ) : (
            <div>
              <ListHeader />
              {list.map((s, i) => (
                <ListRow
                  key={s.id}
                  server={s}
                  last={i === list.length - 1}
                  onClick={() => navigate(`/server/${s.id}`)}
                />
              ))}
            </div>
          )}
        </Card>
      </div>

      <AddServerModal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onAdded={() => {
          setAddOpen(false)
          load()
        }}
      />
    </AppShell>
  )
}

function SummaryCard({
  label,
  value,
  hint,
  tone,
}: {
  label: string
  value: number
  hint: string
  tone?: 'teal' | 'warning' | 'primary'
}) {
  const dot =
    tone === 'teal'
      ? 'bg-[var(--color-teal)]'
      : tone === 'warning'
        ? 'bg-[var(--color-warning)]'
        : tone === 'primary'
          ? 'bg-[var(--color-primary)]'
          : ''
  const numColor =
    tone === 'teal'
      ? 'text-[var(--color-teal-deep)]'
      : tone === 'warning'
        ? 'text-[var(--color-warning-deep)]'
        : tone === 'primary'
          ? 'text-[var(--color-primary)]'
          : ''
  return (
    <Card padded={false} className="p-5">
      <div className="mb-2.5 flex items-center gap-1.5 text-[13px] text-[var(--color-text-2)]">
        {dot && <span className={cn('h-[7px] w-[7px] rounded-full', dot)} />}
        {label}
      </div>
      <div
        className={cn(
          'text-[32px] font-semibold leading-none tabular-num',
          numColor,
        )}
      >
        {value}
      </div>
      <div className="mt-2.5 text-[12px] text-[var(--color-text-muted)]">
        {hint}
      </div>
    </Card>
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

function ListHeader() {
  const cols =
    'grid items-center gap-3 px-6 py-2.5 grid-cols-[1.7fr_.9fr_.9fr_.9fr_.9fr_1.2fr_.9fr_24px]'
  return (
    <div
      className={cn(
        cols,
        'border-b border-[var(--color-divider)] bg-[var(--color-surface-soft)] text-[11px] font-semibold uppercase tracking-[.04em] text-[var(--color-text-muted)]',
      )}
    >
      <span>服务器</span>
      <span>状态</span>
      <span>CPU</span>
      <span>内存</span>
      <span>磁盘</span>
      <span>安全</span>
      <span>今日拦截</span>
      <span />
    </div>
  )
}

function ListRow({
  server,
  last,
  onClick,
}: {
  server: Server
  last?: boolean
  onClick: () => void
}) {
  const offline = server.status === 'offline'
  return (
    <div
      onClick={onClick}
      className={cn(
        'grid cursor-pointer grid-cols-[1.7fr_.9fr_.9fr_.9fr_.9fr_1.2fr_.9fr_24px] items-center gap-3 px-6 py-4 transition-colors hover:bg-[#F8FAFF]',
        !last && 'border-b border-[var(--color-divider)]',
        offline && 'bg-[#FCFCFD]',
      )}
    >
      <div>
        <div
          className={cn(
            'text-[14px] font-semibold',
            offline && 'text-[var(--color-text-2)]',
          )}
        >
          {server.name}
        </div>
        <div className="mono mt-0.5 text-[12px] text-[var(--color-text-muted)]">
          {server.ip}
        </div>
      </div>
      <span
        className={cn(
          'inline-flex items-center gap-1.5 text-[13px] font-medium',
          offline ? 'text-[var(--color-text-muted)]' : 'text-[var(--color-success)]',
        )}
      >
        <StatusDot tone={offline ? 'neutral' : 'success'} />
        {offline ? '离线' : '在线'}
      </span>
      {offline ? (
        <>
          <Em />
          <Em />
          <Em />
        </>
      ) : (
        <>
          <Meter value={server.metrics.cpu} className="max-w-[100px]" />
          <Meter value={server.metrics.mem} className="max-w-[100px]" />
          <Meter value={server.metrics.disk} className="max-w-[100px]" />
        </>
      )}
      <div className="self-start">
        <SecurityBadge
          state={server.security}
          pendingCount={server.pendingCount}
        />
      </div>
      <span
        className={cn(
          'tabular-num text-[13px]',
          offline
            ? 'text-[var(--color-text-faint)]'
            : server.attacksBlockedToday > 0
              ? 'text-[var(--color-text-2)]'
              : 'text-[var(--color-text-muted)]',
        )}
      >
        {offline ? '—' : `${server.attacksBlockedToday} 次`}
      </span>
      <ChevronRightIcon className="h-4 w-4 text-[var(--color-text-faint)]" />
    </div>
  )
}

function Em() {
  return (
    <span className="text-[13px] text-[var(--color-text-faint)]">—</span>
  )
}
