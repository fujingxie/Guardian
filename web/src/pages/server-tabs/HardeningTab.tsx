import { useEffect, useMemo, useState } from 'react'
import { Card } from '@/components/ui/Card'
import { Pill } from '@/components/ui/Pill'
import { Toggle } from '@/components/ui/Toggle'
import { Button } from '@/components/ui/Button'
import {
  FlameIcon,
  KeyIcon,
  LockIcon,
  ShieldCheckIcon,
  TriangleAlertIcon,
} from '@/components/icons'
import { api } from '@/api/client'
import type {
  HardeningGroup,
  HardeningItem,
  Server,
} from '@/api/types'
import { useCountdown } from '@/hooks/useCountdown'
import { mmss } from '@/lib/format'
import { cn } from '@/lib/cn'
import { HighRiskConfirmModal } from '@/modals/HighRiskConfirmModal'
import { TrialTimeoutModal } from '@/modals/TrialTimeoutModal'

interface Props {
  server: Server
  onAlertChange?: (n: number) => void
}

const groupMeta: Record<
  HardeningGroup,
  { title: string; subtitle: string; icon: React.ReactNode }
> = {
  ssh: {
    title: 'SSH 安全',
    subtitle: '收紧 SSH 登录方式 —— 关掉密码、换个端口、禁掉 root。',
    icon: <KeyIcon className="h-[18px] w-[18px]" />,
  },
  firewall: {
    title: '防火墙',
    subtitle: '默认拒绝所有入站，只放行你真的在用的端口。',
    icon: <LockIcon className="h-[18px] w-[18px]" />,
  },
  bruteforce: {
    title: '防爆破 & 更新',
    subtitle: '及时封禁可疑 IP，并自动跟上安全补丁。',
    icon: <FlameIcon className="h-[18px] w-[18px]" />,
  },
}

export function HardeningTab({ server, onAlertChange }: Props) {
  const [items, setItems] = useState<HardeningItem[] | null>(null)
  const [confirmFor, setConfirmFor] = useState<HardeningItem | null>(null)
  const [timeoutFor, setTimeoutFor] = useState<HardeningItem | null>(null)

  async function load() {
    const data = await api.getHardening(server.id)
    setItems(data.items)
  }

  useEffect(() => {
    load()
  }, [server.id])

  const { enabledCount, total } = useMemo(() => {
    if (!items) return { enabledCount: 0, total: 0 }
    return {
      enabledCount: items.filter((i) => i.enabled).length,
      total: items.length,
    }
  }, [items])

  async function toggle(item: HardeningItem, next: boolean) {
    if (item.highRisk && next && !item.enabled) {
      setConfirmFor(item)
      return
    }
    if (item.highRisk && !next && item.status === 'trial') {
      // 显式回滚
      await api.rollbackHardening(server.id, item.key)
      await load()
      return
    }
    await api.applyHardening(server.id, item.key)
    await load()
    onAlertChange?.(0)
  }

  async function confirmTrial(item: HardeningItem) {
    await api.confirmHardening(server.id, item.key)
    await load()
  }

  async function rollback(item: HardeningItem) {
    await api.rollbackHardening(server.id, item.key)
    await load()
  }

  if (!items) {
    return (
      <div className="py-16 text-center text-[13px] text-[var(--color-text-muted)]">
        正在加载加固项…
      </div>
    )
  }

  const groups: HardeningGroup[] = ['ssh', 'firewall', 'bruteforce']

  return (
    <div className="flex flex-col gap-6">
      <Card padded={false} className="p-5">
        <div className="mb-3 flex items-center justify-between">
          <div>
            <div className="flex items-center gap-2 text-[13px] font-medium text-[var(--color-text-2)]">
              <ShieldCheckIcon className="h-4 w-4 text-[var(--color-teal-deep)]" />
              加固进度
            </div>
            <div className="mt-1.5 flex items-baseline gap-2">
              <span className="text-[24px] font-semibold tabular-num text-[var(--color-teal-deep)]">
                {enabledCount}
              </span>
              <span className="text-[13px] text-[var(--color-text-muted)]">
                / {total} 项已开启
              </span>
            </div>
          </div>
          <div className="text-right text-[13px] text-[var(--color-text-2)]">
            今日已挡下{' '}
            <span className="font-semibold tabular-num text-[var(--color-primary)]">
              {server.attacksBlockedToday}
            </span>{' '}
            次攻击
          </div>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-[var(--color-divider)]">
          <div
            className="h-full bg-[var(--color-teal)] transition-[width] duration-500"
            style={{ width: `${(enabledCount / Math.max(1, total)) * 100}%` }}
          />
        </div>
      </Card>

      {groups.map((g) => (
        <section key={g} className="flex flex-col gap-3">
          <div className="flex items-center gap-2.5">
            <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-[var(--color-primary-soft)] text-[var(--color-primary)]">
              {groupMeta[g].icon}
            </span>
            <div>
              <div className="text-[15px] font-semibold">
                {groupMeta[g].title}
              </div>
              <div className="text-[12px] text-[var(--color-text-2)]">
                {groupMeta[g].subtitle}
              </div>
            </div>
          </div>
          <Card padded={false} className="overflow-hidden">
            {items
              .filter((i) => i.group === g)
              .map((it, idx, arr) => (
                <HardeningRow
                  key={it.key}
                  item={it}
                  last={idx === arr.length - 1}
                  onToggle={(next) => toggle(it, next)}
                  onConfirm={() => confirmTrial(it)}
                  onRollback={() => rollback(it)}
                />
              ))}
          </Card>
        </section>
      ))}

      {confirmFor && (
        <HighRiskConfirmModal
          open
          onClose={() => setConfirmFor(null)}
          onConfirm={async () => {
            const target = confirmFor
            setConfirmFor(null)
            await api.applyHardening(server.id, target.key)
            await load()
          }}
          itemTitle={confirmFor.title}
          itemExplanation={confirmFor.plainExplanation}
        />
      )}

      {timeoutFor && (
        <TrialTimeoutModal
          open
          onClose={() => setTimeoutFor(null)}
          itemTitle={timeoutFor.title}
          onRetry={() => {
            setTimeoutFor(null)
            setConfirmFor(timeoutFor)
          }}
        />
      )}

      {/* 监听到点 */}
      {items
        .filter((i) => i.status === 'trial' && i.trial)
        .map((i) => (
          <TrialWatcher
            key={i.key}
            item={i}
            onExpired={async () => {
              await api.rollbackHardening(server.id, i.key)
              await load()
              setTimeoutFor(i)
            }}
          />
        ))}
    </div>
  )
}

function HardeningRow({
  item,
  last,
  onToggle,
  onConfirm,
  onRollback,
}: {
  item: HardeningItem
  last?: boolean
  onToggle: (next: boolean) => void
  onConfirm: () => void
  onRollback: () => void
}) {
  const onTrial = item.status === 'trial' && item.trial
  return (
    <div
      className={cn(
        'flex items-start gap-4 px-5 py-4',
        !last && 'border-b border-[var(--color-divider)]',
        onTrial &&
          'border-l-[3px] border-l-[var(--color-warning)] bg-[var(--color-warning-soft)]/30',
        item.highRisk && !onTrial && 'border-l-[3px] border-l-[var(--color-warning)]/0',
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-[14px] font-semibold">{item.title}</span>
          {item.highRisk && (
            <Pill tone="warning" icon={<TriangleAlertIcon className="h-3 w-3" />}>
              高风险
            </Pill>
          )}
          {item.value && (
            <span className="mono ml-1 inline-flex items-center rounded bg-[var(--color-bg-app)] px-1.5 py-0.5 text-[12px] text-[var(--color-text-2)]">
              {item.value}
            </span>
          )}
        </div>
        <div className="mt-1 text-[13px] leading-relaxed text-[var(--color-text-2)]">
          {item.plainExplanation}
        </div>
        {onTrial && (
          <TrialBar item={item} onConfirm={onConfirm} onRollback={onRollback} />
        )}
      </div>
      <Toggle
        checked={item.enabled}
        onChange={onToggle}
        tone={item.highRisk ? 'warning' : 'primary'}
        ariaLabel={item.title}
      />
    </div>
  )
}

function TrialBar({
  item,
  onConfirm,
  onRollback,
}: {
  item: HardeningItem
  onConfirm: () => void
  onRollback: () => void
}) {
  const { secondsLeft } = useCountdown(item.trial?.rollbackAt)
  return (
    <div className="mt-3 flex items-center gap-3 rounded-[8px] border border-[var(--color-warning-line)] bg-white px-3 py-2.5 text-[13px]">
      <span className="flex items-center gap-1.5 text-[var(--color-warning-deep)]">
        <TriangleAlertIcon className="h-4 w-4" />
        <strong>试运行中</strong>
      </span>
      <span className="text-[var(--color-text-2)]">
        自动回滚于 <span className="mono font-semibold text-[var(--color-warning-deep)]">{mmss(secondsLeft)}</span> 后
      </span>
      <div className="ml-auto flex gap-1.5">
        <Button size="sm" variant="secondary" onClick={onRollback}>
          回滚
        </Button>
        <Button size="sm" variant="warning" onClick={onConfirm}>
          我已能登录 · 确认
        </Button>
      </div>
    </div>
  )
}

function TrialWatcher({
  item,
  onExpired,
}: {
  item: HardeningItem
  onExpired: () => void
}) {
  const { expired } = useCountdown(item.trial?.rollbackAt)
  // 只触发一次：用 useEffect
  useEffect(() => {
    if (expired) onExpired()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [expired])
  return null
}
