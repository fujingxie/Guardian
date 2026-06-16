import { cn } from '@/lib/cn'

interface Tab<T extends string> {
  key: T
  label: string
  badge?: number | string
}

interface Props<T extends string> {
  tabs: Tab<T>[]
  active: T
  onChange: (key: T) => void
  className?: string
}

export function Tabs<T extends string>({
  tabs,
  active,
  onChange,
  className,
}: Props<T>) {
  return (
    <div
      className={cn(
        'flex gap-6 border-b border-[var(--color-border)]',
        className,
      )}
    >
      {tabs.map((t) => {
        const on = t.key === active
        return (
          <button
            key={t.key}
            onClick={() => onChange(t.key)}
            className={cn(
              'relative -mb-px flex items-center gap-2 py-3 text-[14px] transition-colors',
              on
                ? 'font-semibold text-[var(--color-primary)]'
                : 'font-medium text-[var(--color-text-2)] hover:text-[var(--color-text)]',
            )}
          >
            {t.label}
            {t.badge !== undefined && (
              <span
                className={cn(
                  'inline-flex h-5 min-w-[20px] items-center justify-center rounded-full px-1.5 text-[11px] font-semibold',
                  on
                    ? 'bg-[var(--color-primary-soft)] text-[var(--color-primary)]'
                    : 'bg-[var(--color-bg-app)] text-[var(--color-text-2)]',
                )}
              >
                {t.badge}
              </span>
            )}
            {on && (
              <span className="absolute bottom-[-1px] left-0 right-0 h-[2px] rounded-full bg-[var(--color-primary)]" />
            )}
          </button>
        )
      })}
    </div>
  )
}
