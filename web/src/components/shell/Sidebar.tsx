import { NavLink } from 'react-router-dom'
import { cn } from '@/lib/cn'
import { Logo } from '@/components/ui/Logo'
import { LayoutIcon, SettingsIcon } from '@/components/icons'
import { StatusDot } from '@/components/ui/StatusDot'

const navItems = [
  { to: '/', label: '总览', icon: LayoutIcon, end: true },
  { to: '/settings', label: '设置', icon: SettingsIcon, end: false },
]

export function Sidebar() {
  return (
    <aside className="hidden w-[240px] flex-shrink-0 flex-col border-r border-[var(--color-border)] bg-white px-4 py-5 md:flex">
      <div className="flex items-center gap-2.5 px-2 pb-6 pt-2">
        <Logo />
        <div className="text-[17px] font-semibold tracking-[-0.01em]">
          Guardian
        </div>
      </div>
      <nav className="flex flex-col gap-1">
        {navItems.map(({ to, label, icon: Icon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-[14px] transition-colors',
                isActive
                  ? 'bg-[var(--color-primary-soft)] font-semibold text-[var(--color-primary)]'
                  : 'font-medium text-[var(--color-text-2)] hover:bg-[#F2F4F7] hover:text-[var(--color-text)]',
              )
            }
          >
            <Icon className="h-[18px] w-[18px]" />
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="mt-auto flex items-center gap-2 rounded-lg border border-[var(--color-divider)] bg-[var(--color-bg-app)] px-3 py-2.5 text-[12px] text-[var(--color-text-muted)]">
        <StatusDot tone="success" />
        本地自部署 · v0.1
      </div>
    </aside>
  )
}
