import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'

type Tone = 'success' | 'warning' | 'danger' | 'info' | 'neutral' | 'teal'

interface Props {
  tone?: Tone
  icon?: ReactNode
  children: ReactNode
  className?: string
}

const tones: Record<Tone, string> = {
  success:
    'bg-[var(--color-success-soft)] text-[var(--color-success)]',
  warning:
    'bg-[var(--color-warning-soft)] text-[var(--color-warning-deep)]',
  danger: 'bg-[var(--color-danger-soft)] text-[var(--color-danger)]',
  info: 'bg-[var(--color-primary-soft)] text-[var(--color-primary)]',
  neutral: 'bg-[var(--color-bg-app)] text-[var(--color-text-2)]',
  teal: 'bg-[var(--color-teal-soft)] text-[var(--color-teal-deep)]',
}

export function Pill({ tone = 'neutral', icon, children, className }: Props) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-semibold leading-none',
        tones[tone],
        className,
      )}
    >
      {icon}
      {children}
    </span>
  )
}
