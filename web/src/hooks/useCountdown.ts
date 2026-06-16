import { useEffect, useState } from 'react'

/** 给定目标 ISO 时刻，返回距离它还剩多少秒（>=0）；同时报告是否已到点。 */
export function useCountdown(targetIso?: string) {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!targetIso) return
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [targetIso])

  if (!targetIso) return { secondsLeft: 0, expired: true }
  const target = new Date(targetIso).getTime()
  const secondsLeft = Math.max(0, Math.round((target - now) / 1000))
  return { secondsLeft, expired: secondsLeft <= 0 }
}
