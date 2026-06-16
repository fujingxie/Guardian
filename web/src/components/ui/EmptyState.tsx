import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'

interface Props {
  icon: ReactNode
  title: ReactNode
  description?: ReactNode
  tone?: 'teal' | 'neutral' | 'primary'
  action?: ReactNode
  className?: string
}

export function EmptyState({
  icon,
  title,
  description,
  tone = 'neutral',
  action,
  className,
}: Props) {
  const iconBg =
    tone === 'teal'
      ? 'bg-[var(--color-teal-soft)] text-[var(--color-teal-deep)]'
      : tone === 'primary'
        ? 'bg-[var(--color-primary-soft)] text-[var(--color-primary)]'
        : 'bg-[var(--color-bg-app)] text-[var(--color-text-2)]'
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-4 py-16 text-center',
        className,
      )}
    >
      <div
        className={cn(
          'flex h-24 w-24 items-center justify-center rounded-[22px]',
          iconBg,
        )}
      >
        {icon}
      </div>
      <div className="text-[17px] font-semibold text-[var(--color-text)]">
        {title}
      </div>
      {description && (
        <div className="max-w-sm text-[13px] leading-relaxed text-[var(--color-text-2)]">
          {description}
        </div>
      )}
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}
