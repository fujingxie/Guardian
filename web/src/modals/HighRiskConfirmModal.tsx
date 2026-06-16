import { useEffect, useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { ShieldCheckIcon } from '@/components/icons'

interface Props {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  itemTitle: string
  itemExplanation: string
  trialMinutes?: number
}

export function HighRiskConfirmModal({
  open,
  onClose,
  onConfirm,
  itemTitle,
  itemExplanation,
  trialMinutes = 5,
}: Props) {
  const [agreed, setAgreed] = useState(false)
  useEffect(() => {
    if (!open) setAgreed(false)
  }, [open])

  return (
    <Modal
      open={open}
      onClose={onClose}
      topAccent="orange"
      title={`确认${itemTitle}？`}
    >
      <div className="flex flex-col gap-4">
        <p className="m-0 text-[14px] leading-relaxed text-[var(--color-text-2)]">
          {itemExplanation}
          这是一项高风险加固，Guardian 会先<strong>试运行 {trialMinutes} 分钟</strong>
          —— 在这期间你需要打开一个新 SSH 会话验证仍能登录。
        </p>

        <div className="flex items-start gap-3 rounded-[8px] border border-[var(--color-teal-line)] bg-[var(--color-teal-soft)] px-4 py-3">
          <ShieldCheckIcon className="mt-0.5 h-5 w-5 flex-shrink-0 text-[var(--color-teal-deep)]" />
          <div className="text-[13px] leading-relaxed text-[var(--color-teal-deep)]">
            <strong>不会把你锁在外面。</strong>试运行期内如果你没确认能登录，
            Guardian 会在 {trialMinutes} 分钟到点时<strong>自动回滚</strong>本次改动 ——
            agent 失联也会立刻触发回滚。
          </div>
        </div>

        <label className="flex cursor-pointer items-start gap-2.5 text-[13px] text-[var(--color-text)]">
          <input
            type="checkbox"
            checked={agreed}
            onChange={(e) => setAgreed(e.target.checked)}
            className="mt-0.5 h-4 w-4 accent-[var(--color-warning)]"
          />
          <span>我已确认能用 SSH 密钥登录这台机器（在另一个窗口里测过）。</span>
        </label>

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="secondary" onClick={onClose}>
            取消
          </Button>
          <Button
            variant="warning"
            disabled={!agreed}
            onClick={onConfirm}
          >
            试运行并启动 {trialMinutes} 分钟倒计时
          </Button>
        </div>
      </div>
    </Modal>
  )
}
