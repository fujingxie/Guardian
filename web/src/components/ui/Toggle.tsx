import { cn } from '@/lib/cn'

interface Props {
  checked: boolean
  onChange: (next: boolean) => void
  disabled?: boolean
  ariaLabel?: string
  tone?: 'primary' | 'warning'
}

export function Toggle({
  checked,
  onChange,
  disabled,
  ariaLabel,
  tone = 'primary',
}: Props) {
  const onBg =
    tone === 'warning' ? 'bg-[var(--color-warning)]' : 'bg-[var(--color-primary)]'
  return (
    <button
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        'relative inline-flex h-6 w-[42px] flex-shrink-0 cursor-pointer items-center rounded-full transition-colors',
        checked ? onBg : 'bg-[var(--color-border)]',
        disabled && 'cursor-not-allowed opacity-50',
      )}
    >
      <span
        className={cn(
          'inline-block h-5 w-5 transform rounded-full bg-white shadow-[0_1px_2px_rgba(16,24,40,.18)] transition-transform',
          checked ? 'translate-x-[20px]' : 'translate-x-[2px]',
        )}
      />
    </button>
  )
}
