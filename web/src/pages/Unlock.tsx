import { type FormEvent, useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { Logo } from '@/components/ui/Logo'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { StatusDot } from '@/components/ui/StatusDot'
import { EyeIcon, EyeOffIcon } from '@/components/icons'
import { api } from '@/api/client'
import { useAccessToken } from '@/hooks/useAccessToken'

export default function UnlockPage() {
  const { token, save } = useAccessToken()
  const navigate = useNavigate()
  const [value, setValue] = useState('')
  const [show, setShow] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  if (token) return <Navigate to="/" replace />

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!value) return
    setBusy(true)
    setError(null)
    try {
      save(value) // 临时存入，让 api 调用带上
      await api.unlock(value)
      navigate('/', { replace: true })
    } catch {
      save('') // 清空错误 token
      setError('访问口令不正确，请重试')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-bg-canvas)] px-4">
      <div className="flex h-[560px] w-full max-w-[520px] items-center justify-center rounded-[14px] border border-[#DFE3E8] bg-[var(--color-bg-app)] p-10 shadow-[var(--shadow-lg)]">
        <form
          onSubmit={onSubmit}
          className="flex w-[340px] flex-col items-center text-center"
        >
          <Logo size={52} className="!rounded-[13px] shadow-[0_4px_12px_rgba(47,111,237,.35)]" />
          <h2 className="mt-5 text-[24px] font-semibold tracking-[-0.01em]">
            Guardian
          </h2>
          <p className="mt-2 text-[14px] leading-snug text-[var(--color-text-2)]">
            替你盯着服务器，绝不把你锁在门外
          </p>
          <div className="mt-8 w-full text-left">
            <label className="mb-1.5 block text-[13px] font-medium text-[var(--color-text)]">
              访问口令
            </label>
            <div className="relative flex items-center">
              <Input
                type={show ? 'text' : 'password'}
                value={value}
                onChange={(e) => {
                  setValue(e.target.value)
                  if (error) setError(null)
                }}
                placeholder="输入部署时设置的访问口令"
                invalid={!!error}
                className="font-mono tracking-[0.06em] pr-11 h-11 text-[15px]"
                autoFocus
              />
              <button
                type="button"
                onClick={() => setShow((s) => !s)}
                className="absolute right-3 flex cursor-pointer text-[var(--color-text-muted)] hover:text-[var(--color-text-2)]"
                aria-label={show ? '隐藏' : '显示'}
              >
                {show ? (
                  <EyeOffIcon className="h-[18px] w-[18px]" />
                ) : (
                  <EyeIcon className="h-[18px] w-[18px]" />
                )}
              </button>
            </div>
            {error && (
              <div className="mt-2 flex items-center gap-1.5 text-[13px] text-[var(--color-danger)]">
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <circle cx="12" cy="12" r="10" />
                  <path d="M12 8v4M12 16h.01" />
                </svg>
                {error}
              </div>
            )}
          </div>
          <Button
            type="submit"
            loading={busy}
            className="mt-3.5 h-12 w-full text-[15px]"
          >
            进入
          </Button>
          <div className="mt-6 flex items-center gap-2 text-[12px] text-[var(--color-text-muted)]">
            <StatusDot tone="success" />
            自部署 · 单用户 · 无账号
          </div>
          <div className="mt-3 text-[11px] text-[var(--color-text-faint)]">
            演示口令：<span className="font-mono">guardian-demo-2026</span>
          </div>
        </form>
      </div>
    </div>
  )
}
