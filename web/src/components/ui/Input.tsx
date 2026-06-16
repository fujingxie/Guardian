import { type InputHTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/cn'

interface Props extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean
}

export const Input = forwardRef<HTMLInputElement, Props>(function Input(
  { className, invalid, ...rest },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        'h-10 w-full rounded-[6px] border bg-white px-3.5 text-sm leading-none text-[var(--color-text)] placeholder:text-[var(--color-text-muted)] outline-none transition-[box-shadow,border-color]',
        invalid
          ? 'border-[1.5px] border-[var(--color-danger)] focus:shadow-[0_0_0_3px_rgba(220,38,38,.18)]'
          : 'border-[var(--color-border)] focus:border-[var(--color-primary)] focus:shadow-[0_0_0_3px_rgba(47,111,237,.20)]',
        className,
      )}
      {...rest}
    />
  )
})
