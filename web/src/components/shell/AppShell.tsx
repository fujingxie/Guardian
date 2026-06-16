import { type ReactNode } from 'react'
import { Sidebar } from './Sidebar'
import { Topbar } from './Topbar'

interface Props {
  topbar?: ReactNode
  children: ReactNode
}

export function AppShell({ topbar, children }: Props) {
  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[var(--color-bg-app)]">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar>{topbar}</Topbar>
        <main className="flex-1 overflow-auto">{children}</main>
      </div>
    </div>
  )
}
