import { type ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { BellIcon, ChevronDownIcon, ServerStackIcon } from '@/components/icons'
import { api } from '@/api/client'

interface Props {
  children?: ReactNode
}

export function Topbar({ children }: Props) {
  const [summary, setSummary] = useState<{ total: number; online: number } | null>(
    null,
  )
  const [hasUnread, setHasUnread] = useState(true)

  useEffect(() => {
    api.listServers()
      .then((d) =>
        setSummary({ total: d.summary.total, online: d.summary.online }),
      )
      .catch(() => null)
    setHasUnread(true)
  }, [])

  return (
    <header className="flex h-16 flex-shrink-0 items-center justify-between border-b border-[var(--color-border)] bg-white px-8">
      <div className="flex items-center gap-3">
        {children ?? (
          <button className="flex cursor-pointer items-center gap-2 rounded-[6px] border border-[var(--color-border)] px-3 py-2 text-[14px] font-medium transition-colors hover:border-[var(--color-text-faint)] hover:bg-[var(--color-surface-soft)]">
            <ServerStackIcon className="h-4 w-4 text-[var(--color-text-2)]" />
            全部服务器
            {summary && (
              <span className="text-[12px] font-medium text-[var(--color-text-muted)]">
                {summary.total} 台
              </span>
            )}
            <ChevronDownIcon className="h-[15px] w-[15px] text-[var(--color-text-muted)]" />
          </button>
        )}
      </div>
      <div className="flex items-center gap-5">
        {summary && (
          <span className="text-[13px] text-[var(--color-text-2)]">
            {summary.online} 台在线
          </span>
        )}
        <button className="relative flex h-[38px] w-[38px] cursor-pointer items-center justify-center rounded-lg transition-colors hover:bg-[#F2F4F7]">
          <BellIcon className="h-[19px] w-[19px] text-[var(--color-text-2)]" />
          {hasUnread && (
            <span className="absolute right-[9px] top-2 inline-block h-[7px] w-[7px] rounded-full border-[1.5px] border-white bg-[var(--color-danger)]" />
          )}
        </button>
      </div>
    </header>
  )
}
