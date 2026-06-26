import { http, HttpResponse, delay } from 'msw'
import {
  MOCK_ACCESS_TOKEN,
  buildAlerts,
  buildHardening,
  buildMetricSeries,
  servers,
  settings as settingsData,
  summary,
} from './data'
import type { HardeningItem, Settings } from '@/api/types'

const ACCESS_TOKEN_HEADER = 'x-access-token'

function authed(req: Request) {
  return req.headers.get(ACCESS_TOKEN_HEADER) === MOCK_ACCESS_TOKEN
}

const itemsByServer = new Map<string, HardeningItem[]>()
function getItems(serverId: string) {
  if (!itemsByServer.has(serverId)) {
    itemsByServer.set(serverId, buildHardening(serverId))
  }
  return itemsByServer.get(serverId)!
}

const settingsState: { current: Settings } = {
  current: structuredClone(settingsData),
}

const TRIAL_MS = 5 * 60 * 1000

export const handlers = [
  http.post('/api/unlock', async ({ request }) => {
    await delay(200)
    const body = (await request.json()) as { accessToken: string }
    if (body.accessToken === MOCK_ACCESS_TOKEN) {
      return HttpResponse.json({ ok: true, token: MOCK_ACCESS_TOKEN })
    }
    return HttpResponse.json(
      { ok: false, message: '访问口令不正确' },
      { status: 401 },
    )
  }),

  http.get('/api/servers', async ({ request }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    await delay(180)
    return HttpResponse.json({ servers, summary })
  }),

  http.get('/api/servers/:id', async ({ request, params }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    await delay(150)
    const sv = servers.find((s) => s.id === params.id)
    if (!sv) return new HttpResponse(null, { status: 404 })
    return HttpResponse.json({ server: sv })
  }),

  http.get('/api/servers/:id/metrics', async ({ request, params }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    await delay(200)
    return HttpResponse.json({ points: buildMetricSeries(params.id as string) })
  }),

  http.get('/api/servers/:id/hardening', async ({ request, params }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    await delay(150)
    return HttpResponse.json({ items: getItems(params.id as string) })
  }),

  http.post(
    '/api/servers/:id/hardening/:itemKey/apply',
    async ({ request, params }) => {
      if (!authed(request)) return new HttpResponse(null, { status: 401 })
      await delay(220)
      const items = getItems(params.id as string)
      const item = items.find((i) => i.key === params.itemKey)
      if (!item) return new HttpResponse(null, { status: 404 })
      if (item.highRisk) {
        item.enabled = true
        item.status = 'trial'
        item.trial = { rollbackAt: new Date(Date.now() + TRIAL_MS).toISOString() }
      } else {
        item.enabled = !item.enabled
        item.status = 'idle'
      }
      return HttpResponse.json({ item })
    },
  ),

  http.post(
    '/api/servers/:id/hardening/:itemKey/confirm',
    async ({ request, params }) => {
      if (!authed(request)) return new HttpResponse(null, { status: 401 })
      await delay(200)
      const items = getItems(params.id as string)
      const item = items.find((i) => i.key === params.itemKey)
      if (!item) return new HttpResponse(null, { status: 404 })
      item.status = 'idle'
      item.trial = undefined
      return HttpResponse.json({ item })
    },
  ),

  http.post(
    '/api/servers/:id/hardening/:itemKey/rollback',
    async ({ request, params }) => {
      if (!authed(request)) return new HttpResponse(null, { status: 401 })
      await delay(200)
      const items = getItems(params.id as string)
      const item = items.find((i) => i.key === params.itemKey)
      if (!item) return new HttpResponse(null, { status: 404 })
      item.enabled = false
      item.status = 'idle'
      item.trial = undefined
      return HttpResponse.json({ item })
    },
  ),

  http.get('/api/servers/:id/alerts', async ({ request, params }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    await delay(180)
    return HttpResponse.json({ alerts: buildAlerts(params.id as string) })
  }),

  http.get('/api/settings/notifications', async ({ request }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    await delay(120)
    return HttpResponse.json(settingsState.current)
  }),

  http.put('/api/settings/notifications', async ({ request }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    const body = (await request.json()) as Settings
    settingsState.current = body
    await delay(200)
    return HttpResponse.json(settingsState.current)
  }),

  http.post('/api/settings/notifications/test', async ({ request }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    await delay(600)
    return HttpResponse.json({ ok: true, message: '测试通知已发送' })
  }),

  http.post('/api/servers', async ({ request }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    const body = (await request.json()) as { name?: string }
    await delay(220)
    const id = `server-${Math.random().toString(36).slice(2, 8)}`
    const enrollmentToken = `enroll_${Math.random().toString(36).slice(2, 14)}`
    return HttpResponse.json({
      server: {
        id,
        name: body.name || id,
        ip: '—',
        status: 'offline',
        security: 'danger',
        metrics: { cpu: 0, mem: 0, disk: 0, netUp: 0, netDown: 0 },
        attacksBlockedToday: 0,
      },
      enrollmentToken,
      installCommand: `curl -fsSL https://guardian.example.com/install.sh | sudo bash -s -- --token ${enrollmentToken} --console https://guardian.example.com`,
    })
  }),

  http.put('/api/servers/:id', async ({ request, params }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    const body = (await request.json()) as { name: string }
    const sv = servers.find((s) => s.id === params.id)
    if (!sv) return new HttpResponse(null, { status: 404 })
    sv.name = body.name
    return HttpResponse.json({ ok: true })
  }),

  http.delete('/api/servers/:id', async ({ request, params }) => {
    if (!authed(request)) return new HttpResponse(null, { status: 401 })
    const idx = servers.findIndex((s) => s.id === params.id)
    if (idx !== -1) {
      servers.splice(idx, 1)
    }
    return HttpResponse.json({ ok: true })
  }),
]
