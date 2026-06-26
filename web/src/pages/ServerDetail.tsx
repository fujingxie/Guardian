import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { AppShell } from '@/components/shell/AppShell'
import { Tabs } from '@/components/ui/Tabs'
import { StatusDot } from '@/components/ui/StatusDot'
import { ChevronLeftIcon } from '@/components/icons'
import { api } from '@/api/client'
import type { Server } from '@/api/types'
import { OverviewTab } from './server-tabs/OverviewTab'
import { HardeningTab } from './server-tabs/HardeningTab'
import { AlertsTab } from './server-tabs/AlertsTab'

type TabKey = 'overview' | 'hardening' | 'alerts'

export default function ServerDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [server, setServer] = useState<Server | null>(null)
  const [tab, setTab] = useState<TabKey>('overview')
  const [alertCount, setAlertCount] = useState<number | null>(null)

  useEffect(() => {
    api.getServer(id).then((d) => setServer(d.server))
    api.getAlerts(id).then((d) =>
      setAlertCount(d.alerts.filter((a) => !a.resolved).length),
    )
  }, [id])

  async function renameServer() {
    if (!server) return
    const newName = window.prompt('请输入新的服务器名称：', server.name)
    if (newName === null) return
    const trimmed = newName.trim()
    if (!trimmed) {
      alert('名称不能为空')
      return
    }
    try {
      await api.updateServer(server.id, trimmed)
      setServer({ ...server, name: trimmed })
    } catch (e: any) {
      alert('重命名失败: ' + e.message)
    }
  }

  async function deleteServer() {
    if (!server) return
    const confirmed = window.confirm(`确定要删除服务器 "${server.name}" 吗？这将会断开其长连接并删除此服务器的所有历史指标、加固记录和安全告警！`)
    if (!confirmed) return
    try {
      await api.deleteServer(server.id)
      navigate('/')
    } catch (e: any) {
      alert('删除失败: ' + e.message)
    }
  }

  return (
    <AppShell
      topbar={
        <button
          onClick={() => navigate('/')}
          className="flex cursor-pointer items-center gap-1.5 rounded-md px-2 py-1.5 text-[14px] text-[var(--color-text-2)] hover:bg-[#F2F4F7] hover:text-[var(--color-text)]"
        >
          <ChevronLeftIcon className="h-[16px] w-[16px]" />
          所有服务器
        </button>
      }
    >
      <div className="mx-auto max-w-[1080px] px-8 py-8">
        {!server ? (
          <div className="py-16 text-center text-[13px] text-[var(--color-text-muted)]">
            正在加载…
          </div>
        ) : (
          <>
            <div className="mb-5 flex items-center justify-between gap-4">
              <div className="flex items-baseline gap-3">
                <h1 className="m-0 text-[24px] font-semibold tracking-[-0.01em]">
                  {server.name}
                </h1>
                <span className="mono text-[13px] text-[var(--color-text-muted)]">
                  {server.ip}
                </span>
                <span className="text-[var(--color-text-muted)]">·</span>
                <button
                  onClick={renameServer}
                  className="cursor-pointer text-[13px] text-[var(--color-text-muted)] hover:text-[var(--color-text)] hover:underline"
                >
                  重命名
                </button>
                <span className="text-[var(--color-text-muted)]">·</span>
                <button
                  onClick={deleteServer}
                  className="cursor-pointer text-[13px] text-red-500 hover:text-red-700 hover:underline"
                >
                  删除
                </button>
              </div>
              <span
                className={
                  'inline-flex items-center gap-1.5 text-[13px] font-medium ' +
                  (server.status === 'online'
                    ? 'text-[var(--color-success)]'
                    : 'text-[var(--color-text-muted)]')
                }
              >
                <StatusDot
                  tone={server.status === 'online' ? 'success' : 'neutral'}
                />
                {server.status === 'online' ? '在线' : '离线'}
              </span>
            </div>

            {server.status === 'offline' && (
              <div className="mb-5 flex items-start gap-3 rounded-[8px] border border-[var(--color-warning-line)] bg-[var(--color-warning-soft)] px-4 py-3 text-[13px] text-[var(--color-warning-deep)]">
                <span className="mt-0.5">⚠</span>
                <div>
                  这台服务器目前失联，{server.lastSeen ? '上次见到是 ' : ''}下方数据为最后已知值，仅供参考。Guardian 会持续重连。
                </div>
              </div>
            )}

            <Tabs
              tabs={[
                { key: 'overview', label: '概览' },
                { key: 'hardening', label: '安全加固' },
                {
                  key: 'alerts',
                  label: '告警',
                  badge: alertCount ?? undefined,
                },
              ]}
              active={tab}
              onChange={(k) => setTab(k as TabKey)}
              className="mb-7"
            />

            {tab === 'overview' && <OverviewTab server={server} />}
            {tab === 'hardening' && (
              <HardeningTab server={server} onAlertChange={setAlertCount} />
            )}
            {tab === 'alerts' && (
              <AlertsTab serverId={server.id} onCount={setAlertCount} />
            )}
          </>
        )}
      </div>
    </AppShell>
  )
}
