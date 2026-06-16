import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { CheckIcon } from '@/components/icons'

interface Props {
  open: boolean
  onClose: () => void
  onRetry?: () => void
  itemTitle: string
}

export function TrialTimeoutModal({
  open,
  onClose,
  onRetry,
  itemTitle,
}: Props) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      topAccent="red"
      title="没收到你的确认，已自动回滚"
    >
      <div className="flex flex-col gap-4">
        <p className="m-0 text-[14px] leading-relaxed text-[var(--color-text-2)]">
          「{itemTitle}」的试运行倒计时已到，且 Guardian 没收到你「我能登录」的确认，
          因此本次改动已被自动回滚 —— 你的登录方式回到了之前的状态。
        </p>
        <div className="rounded-[8px] border border-[var(--color-divider)] bg-[var(--color-surface-soft)] p-3 text-[13px]">
          <div className="flex items-center gap-2 text-[var(--color-success)]">
            <CheckIcon className="h-4 w-4" />
            密码登录已恢复
          </div>
          <div className="mt-2 flex items-center gap-2 text-[var(--color-success)]">
            <CheckIcon className="h-4 w-4" />
            SSH 连通性自检通过
          </div>
        </div>
        <p className="m-0 text-[13px] leading-relaxed text-[var(--color-text-2)]">
          排查一下密钥是否真的能用，再来重试就好。
        </p>
        <div className="flex justify-end gap-2 pt-1">
          <Button variant="secondary" onClick={onClose}>
            知道了
          </Button>
          {onRetry && <Button onClick={onRetry}>重试</Button>}
        </div>
      </div>
    </Modal>
  )
}
