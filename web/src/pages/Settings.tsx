import { useEffect, useState } from 'react'
import { AppShell } from '@/components/shell/AppShell'
import { Card } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Toggle } from '@/components/ui/Toggle'
import { Pill } from '@/components/ui/Pill'
import { ExternalLinkIcon, ShieldCheckIcon } from '@/components/icons'
import { api } from '@/api/client'
import type { Settings } from '@/api/types'
import { useAccessToken } from '@/hooks/useAccessToken'
import { cn } from '@/lib/cn'

const channels = [
  { key: 'email' as const, label: 'Email', placeholder: 'you@example.com' },
  {
    key: 'telegram' as const,
    label: 'Telegram',
    placeholder: 'Bot token / chat id',
  },
  {
    key: 'serverChan' as const,
    label: 'Server酱',
    placeholder: 'SCT...',
  },
]

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [saving, setSaving] = useState(false)
  const [testFor, setTestFor] = useState<string | null>(null)
  const [testResult, setTestResult] = useState<{
    key: string
    ok: boolean
    message: string
  } | null>(null)
  const { clear } = useAccessToken()

  // 访问口令
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  const [next2, setNext2] = useState('')
  const [pwdMsg, setPwdMsg] = useState<string | null>(null)

  useEffect(() => {
    api.getSettings().then(setSettings)
  }, [])

  async function save(updated: Settings) {
    setSaving(true)
    try {
      const s = await api.updateSettings(updated)
      setSettings(s)
    } finally {
      setSaving(false)
    }
  }

  function patchChannel<K extends keyof Settings['notify']>(
    key: K,
    value: Settings['notify'][K],
  ) {
    if (!settings) return
    save({
      ...settings,
      notify: { ...settings.notify, [key]: value },
    })
  }

  function toggleChannel(key: keyof Settings['notify']['enabled'], on: boolean) {
    if (!settings) return
    save({
      ...settings,
      notify: {
        ...settings.notify,
        enabled: { ...settings.notify.enabled, [key]: on },
      },
    })
  }

  async function testChannel(key: 'email' | 'telegram' | 'serverChan') {
    setTestFor(key)
    setTestResult(null)
    try {
      const r = await api.testNotification(key)
      setTestResult({ key, ok: r.ok, message: r.message })
    } catch {
      setTestResult({ key, ok: false, message: '测试失败' })
    } finally {
      setTestFor(null)
      window.setTimeout(() => setTestResult(null), 3000)
    }
  }

  function updatePassword() {
    setPwdMsg(null)
    if (!cur || !next || !next2) {
      setPwdMsg('请填写所有字段')
      return
    }
    if (next !== next2) {
      setPwdMsg('两次新口令不一致')
      return
    }
    setPwdMsg('已更新（演示）：下次进入控制台请使用新口令。')
    setCur('')
    setNext('')
    setNext2('')
  }

  return (
    <AppShell>
      <div className="mx-auto max-w-[720px] px-8 py-8">
        <div className="mb-6 flex items-end justify-between">
          <div>
            <h1 className="m-0 text-[24px] font-semibold tracking-[-0.01em]">
              设置
            </h1>
            <p className="mt-2 text-[14px] text-[var(--color-text-2)]">
              自部署单用户 —— 只有通知、口令和"关于"三块。
            </p>
          </div>
          <Button variant="ghost" onClick={clear}>
            退出（清除本地口令）
          </Button>
        </div>

        {/* 通知配置 */}
        <Card padded={false} className="mb-6">
          <div className="border-b border-[var(--color-divider)] px-6 py-4">
            <div className="text-[16px] font-semibold">通知配置</div>
            <div className="mt-0.5 text-[13px] text-[var(--color-text-2)]">
              告警发生时主动推给你 —— 配多个通道更稳。
            </div>
          </div>
          {settings ? (
            channels.map((c, i) => {
              const value = (settings.notify[c.key] as string | undefined) ?? ''
              const enabled = settings.notify.enabled?.[c.key] ?? false
              const canTest = !!value
              return (
                <div
                  key={c.key}
                  className={cn(
                    'flex items-center gap-3 px-6 py-4',
                    i !== channels.length - 1 &&
                      'border-b border-[var(--color-divider)]',
                    !canTest && 'opacity-90',
                  )}
                >
                  <div className="w-[88px] flex-shrink-0 text-[14px] font-medium">
                    {c.label}
                  </div>
                  <Input
                    className="flex-1"
                    value={value}
                    placeholder={c.placeholder}
                    onChange={(e) => patchChannel(c.key, e.target.value)}
                  />
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={!canTest || testFor === c.key}
                    onClick={() => testChannel(c.key)}
                  >
                    {testFor === c.key ? '发送中…' : '测试'}
                  </Button>
                  <Toggle
                    checked={!!enabled}
                    onChange={(v) => toggleChannel(c.key, v)}
                    disabled={!canTest}
                    ariaLabel={`${c.label} 开关`}
                  />
                </div>
              )
            })
          ) : (
            <div className="px-6 py-6 text-[13px] text-[var(--color-text-muted)]">
              加载中…
            </div>
          )}
          {testResult && (
            <div className="border-t border-[var(--color-divider)] bg-[var(--color-success-soft)] px-6 py-2.5 text-[13px] text-[var(--color-success)]">
              {testResult.message}
            </div>
          )}
          {saving && (
            <div className="border-t border-[var(--color-divider)] px-6 py-2 text-[12px] text-[var(--color-text-muted)]">
              已保存
            </div>
          )}
        </Card>

        {/* 阈值告警配置 */}
        <Card padded={false} className="mb-6">
          <div className="border-b border-[var(--color-divider)] px-6 py-4">
            <div className="text-[16px] font-semibold">阈值告警配置</div>
            <div className="mt-0.5 text-[13px] text-[var(--color-text-2)]">
              当服务器资源用量持续超过阈值时，自动产生告警并推送通知。
            </div>
          </div>
          {settings ? (
            <div className="grid grid-cols-1 gap-4 px-6 py-5">
              <ThresholdRow
                label="CPU"
                pctValue={settings.notify.cpuPctThreshold ?? 85}
                durValue={settings.notify.cpuDurationMin ?? 5}
                onPctChange={(v) => patchChannel('cpuPctThreshold' as any, v)}
                onDurChange={(v) => patchChannel('cpuDurationMin' as any, v)}
                durUnit="分钟"
              />
              <ThresholdRow
                label="内存"
                pctValue={settings.notify.memPctThreshold ?? 95}
                durValue={settings.notify.memDurationMin ?? 3}
                onPctChange={(v) => patchChannel('memPctThreshold' as any, v)}
                onDurChange={(v) => patchChannel('memDurationMin' as any, v)}
                durUnit="分钟"
              />
              <ThresholdRow
                label="磁盘"
                pctValue={settings.notify.diskPctThreshold ?? 95}
                onPctChange={(v) => patchChannel('diskPctThreshold' as any, v)}
              />
            </div>
          ) : (
            <div className="px-6 py-6 text-[13px] text-[var(--color-text-muted)]">
              加载中…
            </div>
          )}
        </Card>

        {/* 访问口令 */}
        <Card padded={false} className="mb-6">
          <div className="border-b border-[var(--color-divider)] px-6 py-4">
            <div className="text-[16px] font-semibold">访问口令</div>
            <div className="mt-0.5 text-[13px] text-[var(--color-text-2)]">
              进入控制台时输入的那一个口令。
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 px-6 py-5">
            <PasswordRow label="当前口令" value={cur} onChange={setCur} />
            <PasswordRow label="新口令" value={next} onChange={setNext} />
            <PasswordRow label="确认新口令" value={next2} onChange={setNext2} />
            {pwdMsg && (
              <div className="text-[13px] text-[var(--color-text-2)]">{pwdMsg}</div>
            )}
            <div className="flex justify-end">
              <Button onClick={updatePassword}>更新口令</Button>
            </div>
          </div>
        </Card>

        {/* 关于 */}
        {settings && (
          <Card padded={false}>
            <div className="border-b border-[var(--color-divider)] px-6 py-4">
              <div className="text-[16px] font-semibold">关于</div>
            </div>
            <div className="grid grid-cols-2 gap-y-3 px-6 py-5 text-[14px]">
              <span className="text-[var(--color-text-2)]">控制台版本</span>
              <span className="mono text-right">v{settings.version.console}</span>
              <span className="text-[var(--color-text-2)]">
                Agent 版本（在线数）
              </span>
              <span className="mono text-right">
                v{settings.version.agent}{' '}
                <span className="text-[var(--color-text-muted)]">
                  · {settings.version.agentsOnline}
                </span>
              </span>
              <span className="text-[var(--color-text-2)]">开源仓库</span>
              <a
                href="#"
                className="inline-flex items-center justify-end gap-1 text-right text-[var(--color-primary)] hover:underline"
              >
                guardian / server-guardian
                <ExternalLinkIcon className="h-3.5 w-3.5" />
              </a>
            </div>
            <div className="m-5 mt-0 flex items-center gap-2 rounded-[8px] bg-[var(--color-teal-soft)] px-3 py-2.5 text-[13px] text-[var(--color-teal-deep)]">
              <ShieldCheckIcon className="h-4 w-4" />
              <span>
                数据全部留在你的服务器 —— 控制台不持有任何 SSH 凭据，agent 主动出站连接。
              </span>
              <Pill tone="teal" className="ml-auto">
                自部署
              </Pill>
            </div>
          </Card>
        )}
      </div>
    </AppShell>
  )
}

function PasswordRow({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (v: string) => void
}) {
  return (
    <div className="flex items-center gap-3">
      <div className="w-[88px] flex-shrink-0 text-[14px] font-medium">{label}</div>
      <Input
        type="password"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="flex-1 font-mono"
      />
    </div>
  )
}

function ThresholdRow({
  label,
  pctValue,
  durValue,
  onPctChange,
  onDurChange,
  durUnit,
}: {
  label: string
  pctValue: number
  durValue?: number
  onPctChange: (v: number) => void
  onDurChange?: (v: number) => void
  durUnit?: string
}) {
  return (
    <div className="flex items-center gap-3">
      <div className="w-[56px] flex-shrink-0 text-[14px] font-medium">{label}</div>
      <span className="text-[13px] text-[var(--color-text-2)]">超过</span>
      <Input
        type="number"
        value={String(pctValue)}
        onChange={(e) => {
          const v = parseFloat(e.target.value)
          if (!isNaN(v) && v > 0 && v <= 100) onPctChange(v)
        }}
        className="w-[72px] text-center tabular-num"
      />
      <span className="text-[13px] text-[var(--color-text-2)]">%</span>
      {durValue != null && onDurChange && (
        <>
          <span className="text-[13px] text-[var(--color-text-2)]">持续</span>
          <Input
            type="number"
            value={String(durValue)}
            onChange={(e) => {
              const v = parseInt(e.target.value, 10)
              if (!isNaN(v) && v > 0 && v <= 60) onDurChange(v)
            }}
            className="w-[60px] text-center tabular-num"
          />
          <span className="text-[13px] text-[var(--color-text-2)]">{durUnit ?? '分钟'}</span>
        </>
      )}
      {durValue == null && (
        <span className="text-[12px] text-[var(--color-text-muted)]">（立即告警）</span>
      )}
    </div>
  )
}
