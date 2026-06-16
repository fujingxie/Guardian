import { useEffect, useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { CheckIcon, CopyIcon } from '@/components/icons'
import { api } from '@/api/client'

interface Props {
  open: boolean
  onClose: () => void
  onAdded?: () => void
}

export function AddServerModal({ open, onClose, onAdded }: Props) {
  const [name, setName] = useState('')
  const [cmd, setCmd] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [polling, setPolling] = useState(false)

  useEffect(() => {
    if (!open) {
      setName('')
      setCmd(null)
      setCopied(false)
      setPolling(false)
    }
  }, [open])

  async function generate() {
    const data = await api.addServer(name.trim() || undefined)
    setCmd(data.installCommand)
    setPolling(true)
  }

  async function copy() {
    if (!cmd) return
    try {
      await navigator.clipboard.writeText(cmd)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      /* ignore */
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="添加服务器" width={520}>
      {!cmd ? (
        <div className="flex flex-col gap-4">
          <div>
            <label className="mb-1.5 block text-[13px] font-medium">
              名称 <span className="text-[var(--color-text-muted)]">（可选）</span>
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例如 web-prod-02"
              autoFocus
            />
          </div>
          <div className="rounded-[8px] bg-[var(--color-teal-soft)] p-3 text-[13px] leading-relaxed text-[var(--color-teal-deep)]">
            生成后会得到一行安装命令。把它粘到目标服务器的 SSH 会话里执行即可 ——
            Guardian Agent 会从服务器主动连回控制台，<strong>不需要在服务器上打开任何端口</strong>。
          </div>
          <div className="flex justify-end gap-2 pt-1">
            <Button variant="secondary" onClick={onClose}>
              取消
            </Button>
            <Button onClick={generate}>生成安装命令</Button>
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <div className="text-[13px] leading-relaxed text-[var(--color-text-2)]">
            把下面这一行粘进目标服务器的 SSH 会话执行：
          </div>
          <div className="relative">
            <pre className="m-0 max-h-32 overflow-auto rounded-[8px] bg-[#0F1117] px-4 py-3 pr-12 font-mono text-[12.5px] leading-relaxed text-[#E6EAF2] whitespace-pre-wrap break-all">
              {cmd}
            </pre>
            <button
              onClick={copy}
              className="absolute right-2 top-2 flex h-8 w-8 cursor-pointer items-center justify-center rounded-md bg-[rgba(255,255,255,.08)] text-white hover:bg-[rgba(255,255,255,.15)]"
              aria-label="复制"
            >
              {copied ? (
                <CheckIcon className="h-4 w-4" />
              ) : (
                <CopyIcon className="h-4 w-4" />
              )}
            </button>
          </div>
          {polling && (
            <div className="flex items-center gap-2 rounded-[8px] border border-dashed border-[var(--color-border)] bg-[var(--color-surface-soft)] px-3 py-3 text-[13px] text-[var(--color-text-2)]">
              <span className="inline-block h-3 w-3 animate-spin rounded-full border-2 border-[var(--color-primary)] border-t-transparent" />
              等待服务器连接… 通常 30 秒内会自动出现。
            </div>
          )}
          <div className="flex justify-end gap-2 pt-1">
            <Button variant="secondary" onClick={onClose}>
              取消
            </Button>
            <Button onClick={() => onAdded?.()}>完成</Button>
          </div>
        </div>
      )}
    </Modal>
  )
}
