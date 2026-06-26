import type {
  Alert,
  AlertStats,
  HardeningItem,
  MetricPoint,
  Server,
  Settings,
  SummaryStats,
  TimelinePoint,
  InventoryData,
} from './types'

const TOKEN_KEY = 'guardian.accessToken'

export function getAccessToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setAccessToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearAccessToken() {
  localStorage.removeItem(TOKEN_KEY)
}

class HttpError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers)
  const token = getAccessToken()
  if (token) headers.set('x-access-token', token)
  if (init.body && !headers.has('content-type')) {
    headers.set('content-type', 'application/json')
  }
  const res = await fetch(path, { ...init, headers })
  if (res.status === 401) {
    clearAccessToken()
    // 让 UI 层捕获并跳回 /unlock
    const err = new HttpError(401, 'unauthorized')
    throw err
  }
  if (!res.ok) {
    let msg = res.statusText
    try {
      const data = (await res.json()) as { message?: string }
      if (data?.message) msg = data.message
    } catch {
      /* ignore */
    }
    throw new HttpError(res.status, msg)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  unlock: (accessToken: string) =>
    request<{ ok: true; token: string }>('/api/unlock', {
      method: 'POST',
      body: JSON.stringify({ accessToken }),
    }),

  listServers: () =>
    request<{ servers: Server[]; summary: SummaryStats }>('/api/servers'),

  getServer: (id: string) =>
    request<{ server: Server }>(`/api/servers/${id}`),

  getMetrics: (id: string, range = '24h') =>
    request<{ points: MetricPoint[] }>(
      `/api/servers/${id}/metrics?range=${range}`,
    ),

  getHardening: (id: string) =>
    request<{ items: HardeningItem[] }>(`/api/servers/${id}/hardening`),

  applyHardening: (id: string, key: string) =>
    request<{ item: HardeningItem }>(
      `/api/servers/${id}/hardening/${key}/apply`,
      { method: 'POST' },
    ),

  confirmHardening: (id: string, key: string) =>
    request<{ item: HardeningItem }>(
      `/api/servers/${id}/hardening/${key}/confirm`,
      { method: 'POST' },
    ),

  rollbackHardening: (id: string, key: string) =>
    request<{ item: HardeningItem }>(
      `/api/servers/${id}/hardening/${key}/rollback`,
      { method: 'POST' },
    ),

  getAlerts: (id: string) =>
    request<{ alerts: Alert[] }>(`/api/servers/${id}/alerts`),

  getAlertsTimeline: (id: string) =>
    request<{ timeline: TimelinePoint[] }>(`/api/servers/${id}/alerts/timeline`),

  getAlertsStats: (id: string) =>
    request<AlertStats>(`/api/servers/${id}/alerts/stats`),

  getSettings: () => request<Settings>('/api/settings/notifications'),

  updateSettings: (s: Settings) =>
    request<Settings>('/api/settings/notifications', {
      method: 'PUT',
      body: JSON.stringify(s),
    }),

  testNotification: (channel: 'email' | 'telegram' | 'serverChan') =>
    request<{ ok: true; message: string }>(
      '/api/settings/notifications/test',
      { method: 'POST', body: JSON.stringify({ channel }) },
    ),

  addServer: (name?: string) =>
    request<{
      server: Server
      enrollmentToken: string
      installCommand: string
    }>('/api/servers', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  updateServer: (id: string, name: string) =>
    request<{ ok: boolean }>(`/api/servers/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ name }),
    }),

  deleteServer: (id: string) =>
    request<{ ok: boolean }>(`/api/servers/${id}`, {
      method: 'DELETE',
    }),

  getInventory: (id: string) =>
    request<InventoryData>(`/api/servers/${id}/inventory`),
}

export { HttpError }
