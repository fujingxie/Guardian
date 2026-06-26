import React, { useEffect, useState, useMemo } from 'react'
import { FixedSizeList as List } from 'react-window'
import { api } from '@/api/client'
import type { Server, InventoryData, PortItem } from '@/api/types'

interface Props {
  server: Server
}

export function InventoryTab({ server }: Props) {
  const [data, setData] = useState<InventoryData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [serviceSearch, setServiceSearch] = useState('')
  const [packageSearch, setPackageSearch] = useState('')

  useEffect(() => {
    setLoading(true)
    setError(null)
    api.getInventory(server.id)
      .then((d) => {
        setData(d)
        setLoading(false)
      })
      .catch((err) => {
        setError(err.message || '获取系统画像失败')
        setLoading(false)
      })
  }, [server.id])

  // 1. 过滤服务
  const filteredServices = useMemo(() => {
    if (!data?.services) return []
    const q = serviceSearch.trim().toLowerCase()
    if (!q) return data.services
    return data.services.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q),
    )
  }, [data?.services, serviceSearch])

  // 2. 过滤包
  const filteredPackages = useMemo(() => {
    if (!data?.packages) return []
    const q = packageSearch.trim().toLowerCase()
    if (!q) return data.packages
    return data.packages.filter((p) => p.name.toLowerCase().includes(q))
  }, [data?.packages, packageSearch])

  if (loading) {
    return (
      <div className="py-16 text-center text-[13px] text-[var(--color-text-muted)] animate-pulse">
        正在拉取最新系统画像（包含端口、服务与已装包清单）…
      </div>
    )
  }

  if (error) {
    return (
      <div className="my-4 rounded-[8px] border border-red-200 bg-red-50 p-4 text-[13px] text-red-700">
        ❌ 加载画像失败：{error}
      </div>
    )
  }

  // 监听端口的“公网监听且非22端口”警告判定
  const isDangerPort = (item: PortItem) => {
    const isPublic = item.addr === '0.0.0.0' || item.addr === '::' || item.addr === '*'
    return isPublic && item.port !== 22
  }

  // react-window 的 Row 渲染器
  const Row = ({ index, style }: { index: number; style: React.CSSProperties }) => {
    const pkg = filteredPackages[index]
    if (!pkg) return null
    return (
      <div
        style={style}
        className="flex items-center justify-between border-b border-[var(--color-divider)] px-4 py-2 hover:bg-[var(--color-surface-soft)]"
      >
        <span className="truncate font-medium text-[var(--color-text)] text-[13px]" title={pkg.name}>
          {pkg.name}
        </span>
        <span className="mono truncate text-[var(--color-text-2)] text-[12px]" title={pkg.version}>
          {pkg.version}
        </span>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
      {/* 左侧：监听端口与服务（各占7列） */}
      <div className="flex flex-col gap-6 lg:col-span-7">
        
        {/* 端口卡片 */}
        <div className="rounded-[var(--radius-card)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm transition-all hover:shadow-[0_4px_16px_rgba(16,24,40,0.04)]">
          <div className="mb-4 flex items-center justify-between border-b border-[var(--color-divider)] pb-3">
            <h2 className="m-0 text-[15px] font-semibold text-[var(--color-text)]">
              🔌 监听端口 ({data?.ports?.length || 0})
            </h2>
            <span className="text-[11px] text-[var(--color-text-muted)]">
              (数据更新于 {data?.ts ? new Date(data.ts).toLocaleTimeString() : '—'})
            </span>
          </div>

          <div className="flex flex-col gap-2 max-h-[300px] overflow-y-auto pr-1">
            {(!data?.ports || data.ports.length === 0) ? (
              <div className="py-6 text-center text-[12px] text-[var(--color-text-muted)]">
                暂无端口监听数据
              </div>
            ) : (
              data.ports.map((p, idx) => {
                const danger = isDangerPort(p)
                return (
                  <div
                    key={idx}
                    className={`flex flex-col justify-between rounded-[6px] border p-2.5 transition-colors ${
                      danger
                        ? 'border-[var(--color-warning-line)] bg-[var(--color-warning-soft)]'
                        : 'border-[var(--color-border)] bg-[var(--color-surface-soft)]'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="mono text-[14px] font-semibold text-[var(--color-text)]">
                        {p.addr}:{p.port} <span className="text-[11px] font-normal text-[var(--color-text-2)]">({p.proto})</span>
                      </span>
                      {danger && (
                        <span className="rounded-[4px] bg-[var(--color-warning-deep)] px-1.5 py-0.5 text-[10px] font-bold text-white uppercase tracking-wider">
                          ⚠ 外部暴露风险
                        </span>
                      )}
                    </div>
                    {(p.process || p.pid) && (
                      <div className={`mt-1 flex items-center gap-2 text-[12px] ${danger ? 'text-[var(--color-warning-deep)]' : 'text-[var(--color-text-2)]'}`}>
                        <span className="font-medium">进程:</span>
                        <span className="mono">{p.process || '—'}</span>
                        <span className="text-[var(--color-text-muted)]">|</span>
                        <span className="font-medium">PID:</span>
                        <span className="mono">{p.pid || '—'}</span>
                      </div>
                    )}
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* 服务卡片 */}
        <div className="rounded-[var(--radius-card)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm transition-all hover:shadow-[0_4px_16px_rgba(16,24,40,0.04)]">
          <div className="mb-4 flex flex-col gap-3 border-b border-[var(--color-divider)] pb-3 md:flex-row md:items-center md:justify-between">
            <h2 className="m-0 text-[15px] font-semibold text-[var(--color-text)]">
              ⚙ 运行中系统服务 ({filteredServices.length}/{data?.services?.length || 0})
            </h2>
            <input
              type="text"
              placeholder="搜索服务名称 / 描述..."
              value={serviceSearch}
              onChange={(e) => setServiceSearch(e.target.value)}
              className="w-full max-w-[240px] rounded-[var(--radius-input)] border border-[var(--color-border)] px-2.5 py-1.5 text-[12px] outline-none transition-colors focus:border-[var(--color-primary)]"
            />
          </div>

          <div className="flex flex-col gap-2 max-h-[300px] overflow-y-auto pr-1">
            {filteredServices.length === 0 ? (
              <div className="py-8 text-center text-[12px] text-[var(--color-text-muted)]">
                没有找到匹配的运行中服务
              </div>
            ) : (
              filteredServices.map((s, idx) => (
                <div
                  key={idx}
                  className="flex items-start justify-between rounded-[6px] border border-[var(--color-border)] bg-[var(--color-surface-soft)] p-2.5 hover:bg-[var(--color-surface)]"
                >
                  <div className="mr-3 truncate">
                    <div className="truncate font-semibold text-[var(--color-text)] text-[13px]" title={s.name}>
                      {s.name}
                    </div>
                    <div className="mt-0.5 truncate text-[12px] text-[var(--color-text-2)]" title={s.description}>
                      {s.description || '—'}
                    </div>
                  </div>
                  <span className="rounded-[var(--radius-pill)] bg-[var(--color-teal-soft)] px-2 py-0.5 text-[11px] font-medium text-[var(--color-teal-deep)] border border-[var(--color-teal-line)] whitespace-nowrap">
                    {s.active}
                  </span>
                </div>
              ))
            )}
          </div>
        </div>

      </div>

      {/* 右侧：软件包清单（占5列） */}
      <div className="lg:col-span-5">
        <div className="flex flex-col rounded-[var(--radius-card)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm transition-all hover:shadow-[0_4px_16px_rgba(16,24,40,0.04)] h-full">
          <div className="mb-4 flex flex-col gap-3 border-b border-[var(--color-divider)] pb-3">
            <h2 className="m-0 text-[15px] font-semibold text-[var(--color-text)]">
              📦 已安装软件包 ({filteredPackages.length}/{data?.packages?.length || 0})
            </h2>
            <input
              type="text"
              placeholder="搜索软件包名称..."
              value={packageSearch}
              onChange={(e) => setPackageSearch(e.target.value)}
              className="w-full rounded-[var(--radius-input)] border border-[var(--color-border)] px-2.5 py-1.5 text-[12px] outline-none transition-colors focus:border-[var(--color-primary)]"
            />
          </div>

          <div className="flex-1 rounded-[6px] border border-[var(--color-border)] overflow-hidden bg-[var(--color-surface-soft)]">
            {filteredPackages.length === 0 ? (
              <div className="py-16 text-center text-[12px] text-[var(--color-text-muted)]">
                没有找到匹配的已装包
              </div>
            ) : (
              <List
                height={480}
                itemCount={filteredPackages.length}
                itemSize={38}
                width="100%"
              >
                {Row}
              </List>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
