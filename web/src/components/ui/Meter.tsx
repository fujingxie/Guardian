import { cn } from '@/lib/cn'

interface Props {
  value: number // 0–100
  label?: string
  className?: string
  showValue?: boolean
  size?: 'sm' | 'md'
}

export function Meter({
  value,
  label,
  className,
  showValue = true,
  size = 'sm',
}: Props) {
  const high = value >= 80
  const trackH = size === 'md' ? 'h-1.5' : 'h-[5px]'
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      {showValue && (
        <div className="flex items-center justify-between gap-2">
          {label && (
            <span className="text-[12px] text-[var(--color-text-2)]">
              {label}
            </span>
          )}
          <span
            className={cn(
              'text-[13px] font-medium tabular-num',
              high && 'font-semibold text-[var(--color-warning)]',
            )}
          >
            {Math.round(value)}%
          </span>
        </div>
      )}
      <div
        className={cn(
          'overflow-hidden rounded-full bg-[var(--color-divider)]',
          trackH,
        )}
      >
        <div
          className={cn(
            'h-full rounded-full transition-[width] duration-500',
            high
              ? 'bg-[var(--color-warning)]'
              : 'bg-[var(--color-primary)]',
          )}
          style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
        />
      </div>
    </div>
  )
}
