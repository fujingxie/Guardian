import { cn } from '@/lib/cn'

export function Logo({
  size = 34,
  className,
}: {
  size?: number
  className?: string
}) {
  return (
    <div
      className={cn(
        'flex items-center justify-center rounded-[9px] bg-[var(--color-primary)] shadow-[0_2px_6px_rgba(47,111,237,.35)]',
        className,
      )}
      style={{ width: size, height: size }}
    >
      <svg
        width={Math.round(size * 0.56)}
        height={Math.round(size * 0.56)}
        viewBox="0 0 24 24"
        fill="none"
        stroke="white"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
        <path d="m9 12 2 2 4-4" />
      </svg>
    </div>
  )
}
