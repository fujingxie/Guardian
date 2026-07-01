import type {
  Alert,
  AlertStats,
  HardeningItem,
  MetricPoint,
  Server,
  Settings,
  SummaryStats,
  TimelinePoint,
} from '@/api/types'

// 一组演示数据，对齐设计稿中的 6 台机器
export const MOCK_ACCESS_TOKEN = 'guardian-demo-2026'

export const servers: Server[] = [
  {
    id: 'web-prod-01',
    name: 'web-prod-01',
    ip: '45.32.18.207',
    status: 'online',
    security: 'protected',
    metrics: { cpu: 23, mem: 61, disk: 48, netUp: 1.2, netDown: 4.6 },
    attacksBlockedToday: 23,
    system: {
      distro: 'Ubuntu 22.04 LTS',
      kernel: '5.15.0-91-generic',
      uptime: '38 天 4 小时',
      agent: 'v0.1.0',
    },
  },
  {
    id: 'db-master',
    name: 'db-master',
    ip: '45.32.18.91',
    status: 'online',
    security: 'pending',
    pendingCount: 3,
    metrics: { cpu: 67, mem: 82, disk: 55, netUp: 0.4, netDown: 0.9 },
    attacksBlockedToday: 8,
    system: {
      distro: 'Debian 12',
      kernel: '6.1.0-18-amd64',
      uptime: '12 天 9 小时',
      agent: 'v0.1.0',
    },
  },
  {
    id: 'api-staging',
    name: 'api-staging',
    ip: '139.84.220.16',
    status: 'online',
    security: 'protected',
    metrics: { cpu: 12, mem: 34, disk: 71, netUp: 0.2, netDown: 1.1 },
    attacksBlockedToday: 0,
    system: {
      distro: 'Ubuntu 22.04 LTS',
      kernel: '5.15.0-91-generic',
      uptime: '5 天 17 小时',
      agent: 'v0.1.0',
    },
  },
  {
    id: 'mail-relay',
    name: 'mail-relay',
    ip: '207.148.83.5',
    status: 'offline',
    security: 'danger',
    metrics: { cpu: 0, mem: 0, disk: 0, netUp: 0, netDown: 0 },
    attacksBlockedToday: 0,
    lastSeen: new Date(Date.now() - 12 * 60 * 1000).toISOString(),
    system: {
      distro: 'Ubuntu 20.04 LTS',
      kernel: '5.4.0-150-generic',
      uptime: '—',
      agent: 'v0.1.0',
    },
  },
  {
    id: 'cache-redis-02',
    name: 'cache-redis-02',
    ip: '45.77.130.44',
    status: 'online',
    security: 'protected',
    metrics: { cpu: 8, mem: 40, disk: 22, netUp: 0.8, netDown: 1.4 },
    attacksBlockedToday: 2,
    system: {
      distro: 'Debian 12',
      kernel: '6.1.0-18-amd64',
      uptime: '22 天 1 小时',
      agent: 'v0.1.0',
    },
  },
  {
    id: 'worker-eu-1',
    name: 'worker-eu-1',
    ip: '95.179.142.8',
    status: 'online',
    security: 'pending',
    pendingCount: 1,
    metrics: { cpu: 88, mem: 70, disk: 60, netUp: 2.1, netDown: 5.3 },
    attacksBlockedToday: 5,
    system: {
      distro: 'Ubuntu 22.04 LTS',
      kernel: '5.15.0-91-generic',
      uptime: '9 天 3 小时',
      agent: 'v0.1.0',
    },
  },
]

export const summary: SummaryStats = {
  total: 6,
  online: 5,
  offline: 1,
  protected: 4,
  pending: 4,
  pendingServers: 2,
  todayBlocked: 39,
  yesterdayDelta: 12,
}

// 24 小时指标曲线 —— 每个服务器一份
export function buildMetricSeries(serverId: string): MetricPoint[] {
  const seed = serverId
    .split('')
    .reduce((acc, c) => acc + c.charCodeAt(0), 0)
  const rand = (i: number) => {
    const x = Math.sin(seed * 9.1 + i * 1.7) * 10000
    return x - Math.floor(x)
  }
  const points: MetricPoint[] = []
  const now = Date.now()
  for (let i = 144; i >= 0; i--) {
    const ts = new Date(now - i * 10 * 60 * 1000).toISOString()
    const wave = (Math.sin(i / 6) + 1) / 2
    const cpu = Math.round(15 + wave * 45 + rand(i) * 18)
    const mem = Math.round(40 + Math.sin(i / 9) * 12 + rand(i + 1) * 14)
    points.push({
      ts,
      cpu: Math.min(99, Math.max(2, cpu)),
      mem: Math.min(99, Math.max(10, mem)),
      disk: 50 + Math.round(rand(i + 2) * 3),
      netUp: +(rand(i + 3) * 4).toFixed(2),
      netDown: +(rand(i + 4) * 8).toFixed(2),
    })
  }
  return points
}

export function buildHardening(serverId: string): HardeningItem[] {
  const sv = servers.find((s) => s.id === serverId)
  const pending = sv?.security === 'pending'
  return [
    {
      key: 'ssh_no_password',
      group: 'ssh',
      title: '禁用 SSH 密码登录',
      plainExplanation: '只允许密钥登录，让扫描密码的机器人扑空。',
      enabled: !pending && serverId !== 'db-master',
      highRisk: true,
    },
    {
      key: 'ssh_port',
      group: 'ssh',
      title: '更换 SSH 端口',
      plainExplanation: '把默认 22 换成不常见端口，避开 99% 的自动化扫描。',
      enabled: serverId === 'web-prod-01' || serverId === 'cache-redis-02',
      value: serverId === 'web-prod-01' ? ':49222' : ':22',
    },
    {
      key: 'ssh_no_root',
      group: 'ssh',
      title: '禁止 root 用户登录',
      plainExplanation: '即使密码泄露，攻击者也拿不到最高权限。',
      enabled: true,
    },
    {
      key: 'ufw',
      group: 'firewall',
      title: '启用基础防火墙（UFW）',
      plainExplanation: '默认拒绝所有入站连接，只放行明确允许的端口。',
      enabled: serverId !== 'db-master',
    },
    {
      key: 'ufw_ports',
      group: 'firewall',
      title: '配置放行端口',
      plainExplanation: '只放行你实际在用的服务端口。',
      enabled: true,
      value: '22 · 80 · 443',
    },
    {
      key: 'fail2ban',
      group: 'bruteforce',
      title: '启用 fail2ban 反爆破',
      plainExplanation: '同一 IP 短时间多次失败，自动封禁。',
      enabled: serverId !== 'worker-eu-1',
      value: serverId === 'web-prod-01' ? '今日封禁 7 IP' : undefined,
    },
    {
      key: 'login_limit',
      group: 'bruteforce',
      title: '限制并发登录尝试',
      plainExplanation: '同时只允许少量登录会话，挡住分布式爆破。',
      enabled: serverId === 'web-prod-01',
    },
    {
      key: 'auto_update',
      group: 'bruteforce',
      title: '自动安全更新',
      plainExplanation: '系统打补丁不掉队，新漏洞冷启动就被堵上。',
      enabled: serverId !== 'db-master' && serverId !== 'worker-eu-1',
    },
  ]
}

export function buildAlerts(serverId: string): Alert[] {
  const sv = servers.find((s) => s.id === serverId)
  if (!sv) return []
  if (sv.attacksBlockedToday === 0) return []
  const now = Date.now()
  const samples: Omit<Alert, 'id' | 'ts'>[] = [
    {
      kind: 'bruteforce',
      sourceIp: '193.42.118.55',
      severity: 'high',
      read: false,
      resolved: false,
      title: 'SSH 爆破已被 fail2ban 阻止',
      message:
        '同一 IP 在 3 分钟内尝试登录 18 次后被自动封禁 1 小时，未造成入侵。',
    },
    {
      kind: 'port_scan',
      sourceIp: '45.155.205.211',
      severity: 'medium',
      read: false,
      resolved: false,
      title: '检测到端口扫描',
      message: '对外暴露端口被陌生 IP 顺序探测，已记录但未拦截。',
    },
    {
      kind: 'new_login',
      sourceIp: '123.118.91.4',
      severity: 'info',
      read: true,
      resolved: false,
      title: '新设备 SSH 登录成功',
      message: '首次出现的 IP 通过密钥登录成功 —— 如果不是你本人，请尽快排查。',
    },
    {
      kind: 'bruteforce',
      sourceIp: '88.214.25.51',
      severity: 'high',
      read: true,
      resolved: true,
      title: '另一次 SSH 爆破已被阻止',
      message: '攻击者尝试常见用户名 root/admin/test，全部被拒并封禁。',
    },
  ]
  return samples.slice(0, sv.attacksBlockedToday > 2 ? 4 : 2).map((a, i) => ({
    id: `${serverId}-alert-${i}`,
    ts: new Date(now - (i + 1) * 47 * 60 * 1000).toISOString(),
    ...a,
  }))
}

export function buildAlertsTimeline(serverId: string): TimelinePoint[] {
  const seed = serverId
    .split('')
    .reduce((acc, c) => acc + c.charCodeAt(0), 0)
  const now = Date.now()
  return Array.from({ length: 7 }, (_, idx) => {
    const dayOffset = 6 - idx
    const d = new Date(now - dayOffset * 24 * 60 * 60 * 1000)
    const base = Math.max(0, Math.round(Math.sin(seed + idx) * 12 + idx * 8))
    return {
      date: d.toISOString().slice(0, 10),
      high: idx % 3 === 0 ? Math.round(base * 0.25) : 0,
      medium: Math.round(base * 0.7),
      info: Math.round(base * 0.35),
    }
  })
}

export function buildAlertStats(): AlertStats {
  return {
    topIPs: [
      { ip: '2.57.121.25', country: '英国', count: 86819 },
      { ip: '91.92.40.240', country: '保加利亚', count: 48050 },
      { ip: '92.118.39.77', country: '美国', count: 33638 },
      { ip: '45.227.254.170', country: '立陶宛', count: 21528 },
      { ip: '91.92.40.204', country: '未知', count: 21111 },
      { ip: '2.57.121.112', country: '英国', count: 19675 },
      { ip: '185.191.126.213', country: '荷兰', count: 16820 },
      { ip: '45.146.130.188', country: '新加坡', count: 10422 },
      { ip: '89.248.163.35', country: '罗马尼亚', count: 8205 },
      { ip: '43.134.10.8', country: '中国', count: 7219 },
    ],
    countries: [
      { country: '未知', count: 120000 },
      { country: '英国', count: 88000 },
      { country: '保加利亚', count: 52000 },
      { country: '荷兰', count: 43000 },
      { country: '美国', count: 36000 },
      { country: '中国', count: 24000 },
      { country: '新加坡', count: 18000 },
      { country: '香港', count: 15000 },
      { country: '印度', count: 9000 },
      { country: '德国', count: 7200 },
      { country: '立陶宛', count: 6800 },
      { country: '法国', count: 5200 },
      { country: '加拿大', count: 3600 },
      { country: '巴西', count: 2600 },
      { country: '日本', count: 2100 },
    ],
  }
}

export const settings: Settings = {
  notify: {
    email: 'me@example.com',
    telegram: '',
    serverChan: '',
    alertTypes: {
      bruteforce: true,
      port_scan: true,
      new_login: true,
      metric_threshold: true,
      offline: true,
      unknown: true,
    },
    enabled: { email: true, telegram: false, serverChan: false },
  },
  version: { console: '0.1.0', agent: '0.1.0', agentsOnline: '5 / 6' },
}
