import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import { RequireUnlock } from '@/components/RequireUnlock'

const UnlockPage = lazy(() => import('@/pages/Unlock'))
const OverviewPage = lazy(() => import('@/pages/Overview'))
const ServerDetailPage = lazy(() => import('@/pages/ServerDetail'))
const SettingsPage = lazy(() => import('@/pages/Settings'))

function Loading() {
  return (
    <div className="flex h-screen w-screen items-center justify-center text-[13px] text-[var(--color-text-muted)]">
      加载中…
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <Suspense fallback={<Loading />}>
        <Routes>
          <Route path="/unlock" element={<UnlockPage />} />
          <Route
            path="/"
            element={
              <RequireUnlock>
                <OverviewPage />
              </RequireUnlock>
            }
          />
          <Route
            path="/server/:id"
            element={
              <RequireUnlock>
                <ServerDetailPage />
              </RequireUnlock>
            }
          />
          <Route
            path="/settings"
            element={
              <RequireUnlock>
                <SettingsPage />
              </RequireUnlock>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Suspense>
    </BrowserRouter>
  )
}
