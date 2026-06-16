import { type ReactNode, useEffect } from 'react'
import { cn } from '@/lib/cn'

interface Props {
  open: boolean
  onClose: () => void
  title?: ReactNode
  topAccent?: 'orange' | 'red' | 'teal' | null
  width?: number
  children: ReactNode
}

export function Modal({
  open,
  onClose,
  title,
  topAccent = null,
  width = 460,
  children,
}: Props) {
  useEffect(() => {
    if (!open) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = prev
      window.removeEventListener('keydown', onKey)
    }
  }, [open, onClose])

  if (!open) return null

  const accentBg =
    topAccent === 'orange'
      ? 'bg-[var(--color-warning)]'
      : topAccent === 'red'
        ? 'bg-[var(--color-danger)]'
        : topAccent === 'teal'
          ? 'bg-[var(--color-teal)]'
          : ''

  return (
    <div
      className="fixed inset-0 z-50 flex animate-overlay-in items-center justify-center bg-[rgba(16,24,40,0.45)] px-4"
      onClick={onClose}
    >
      <div
        className="relative max-h-[88vh] w-full animate-modal-in overflow-hidden rounded-[12px] bg-white shadow-[var(--shadow-modal)]"
        style={{ maxWidth: width }}
        onClick={(e) => e.stopPropagation()}
      >
        {topAccent && (
          <div className={cn('h-1 w-full flex-shrink-0', accentBg)} />
        )}
        {title && (
          <div className="border-b border-[var(--color-divider)] px-6 py-5">
            <h3 className="m-0 text-[17px] font-semibold leading-tight">
              {title}
            </h3>
          </div>
        )}
        <div className="px-6 py-5">{children}</div>
      </div>
    </div>
  )
}
