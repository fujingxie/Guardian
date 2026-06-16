import { cn } from '@/lib/cn'

type Tone = 'success' | 'danger' | 'warning' | 'neutral' | 'primary' | 'teal'

const tones: Record<Tone, string> = {
  success: 'bg-[var(--color-success)]',
  danger: 'bg-[var(--color-danger)]',
  warning: 'bg-[var(--color-warning)]',
  neutral: 'bg-[var(--color-text-muted)]',
  primary: 'bg-[var(--color-primary)]',
  teal: 'bg-[var(--color-teal)]',
}

export function StatusDot({
  tone = 'success',
  className,
}: {
  tone?: Tone
  className?: string
}) {
  return (
    <span
      className={cn(
        'inline-block h-[7px] w-[7px] flex-shrink-0 rounded-full',
        tones[tone],
        className,
      )}
    />
  )
}
