import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

interface Props extends HTMLAttributes<HTMLDivElement> {
  interactive?: boolean
  padded?: boolean
  accent?: 'orange' | 'red' | 'teal' | null
}

export function Card({
  className,
  interactive,
  padded = true,
  accent = null,
  children,
  ...rest
}: Props) {
  return (
    <div
      className={cn(
        'rounded-[10px] border border-[var(--color-border)] bg-white shadow-[var(--shadow-sm)] transition-all',
        padded && 'p-6',
        interactive &&
          'cursor-pointer hover:-translate-y-0.5 hover:border-[var(--color-text-faint)] hover:shadow-[0_4px_12px_rgba(16,24,40,.08)]',
        accent === 'orange' && 'border-l-[3px] border-l-[var(--color-warning)]',
        accent === 'red' && 'border-l-[3px] border-l-[var(--color-danger)]',
        accent === 'teal' && 'border-l-[3px] border-l-[var(--color-teal)]',
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  )
}
