import { type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAccessToken } from '@/hooks/useAccessToken'

export function RequireUnlock({ children }: { children: ReactNode }) {
  const { token } = useAccessToken()
  const location = useLocation()
  if (!token) {
    return <Navigate to="/unlock" replace state={{ from: location }} />
  }
  return <>{children}</>
}
