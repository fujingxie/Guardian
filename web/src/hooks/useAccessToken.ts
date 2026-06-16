import { useEffect, useState } from 'react'
import { clearAccessToken, getAccessToken, setAccessToken } from '@/api/client'

export function useAccessToken() {
  const [token, setTokenState] = useState<string | null>(() => getAccessToken())

  useEffect(() => {
    const handler = () => setTokenState(getAccessToken())
    window.addEventListener('storage', handler)
    window.addEventListener('guardian:auth-change', handler)
    return () => {
      window.removeEventListener('storage', handler)
      window.removeEventListener('guardian:auth-change', handler)
    }
  }, [])

  return {
    token,
    save(t: string) {
      setAccessToken(t)
      setTokenState(t)
      window.dispatchEvent(new Event('guardian:auth-change'))
    },
    clear() {
      clearAccessToken()
      setTokenState(null)
      window.dispatchEvent(new Event('guardian:auth-change'))
    },
  }
}
