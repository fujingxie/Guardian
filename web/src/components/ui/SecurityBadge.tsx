import { ShieldCheckIcon, TriangleAlertIcon } from '@/components/icons'
import { Pill } from './Pill'
import type { SecurityState } from '@/api/types'

export function SecurityBadge({
  state,
  pendingCount,
}: {
  state: SecurityState
  pendingCount?: number
}) {
  if (state === 'protected') {
    return (
      <Pill tone="success" icon={<ShieldCheckIcon className="h-3 w-3" />}>
        受保护
      </Pill>
    )
  }
  if (state === 'pending') {
    return (
      <Pill tone="warning" icon={<TriangleAlertIcon className="h-3 w-3" />}>
        {pendingCount ? `${pendingCount} 项待处理` : '待处理'}
      </Pill>
    )
  }
  return (
    <Pill
      tone="danger"
      icon={<span className="inline-block h-1.5 w-1.5 rounded-full bg-current" />}
    >
      高风险
    </Pill>
  )
}
