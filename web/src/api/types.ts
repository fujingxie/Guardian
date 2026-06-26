// 与 docs/SPEC.md §5 严格一致 —— 前后端共用契约
export type ServerStatus = 'online' | 'offline'
export type SecurityState = 'protected' | 'pending' | 'danger'

export interface ServerMetrics {
  cpu: number // 0–100
  mem: number // 0–100
  disk: number // 0–100
  netUp: number // MB/s
  netDown: number // MB/s
}

export interface ServerSystem {
  distro: string
  kernel: string
  uptime: string
  agent: string
}

export interface Server {
  id: string
  name: string
  ip: string
  status: ServerStatus
  security: SecurityState
  pendingCount?: number
  metrics: ServerMetrics
  attacksBlockedToday: number
  lastSeen?: string
  system?: ServerSystem
  hardeningApplied?: number
  hardeningTrial?: number
  hardeningTotal?: number
}

export type HardeningKey =
  | 'ssh_no_password'
  | 'ssh_port'
  | 'ssh_no_root'
  | 'ufw'
  | 'ufw_ports'
  | 'fail2ban'
  | 'login_limit'
  | 'auto_update'

export type HardeningGroup = 'ssh' | 'firewall' | 'bruteforce'

export interface HardeningItem {
  key: HardeningKey
  group: HardeningGroup
  title: string
  plainExplanation: string
  enabled: boolean
  highRisk?: boolean
  value?: string
  trial?: { rollbackAt: string } // ISO 时刻
  status?: 'idle' | 'applying' | 'trial' | 'failed'
}

export type AlertKind = 'bruteforce' | 'port_scan' | 'new_login' | 'metric_threshold' | 'offline' | 'unknown'
export type Severity = 'high' | 'medium' | 'info'

export interface Alert {
  id: string
  ts: string
  kind: AlertKind
  sourceIp?: string
  country?: string
  severity: Severity
  read: boolean
  resolved: boolean
  title: string
  message: string
}

export interface MetricPoint {
  ts: string // ISO
  cpu: number
  mem: number
  disk: number
  netUp: number
  netDown: number
}

export interface NotifySettings {
  email?: string
  telegram?: string
  serverChan?: string
  enabled: {
    email: boolean
    telegram: boolean
    serverChan: boolean
  }
  cpuPctThreshold?: number
  cpuDurationMin?: number
  memPctThreshold?: number
  memDurationMin?: number
  diskPctThreshold?: number
}

export interface Settings {
  notify: NotifySettings
  version: { console: string; agent: string; agentsOnline: string }
}

export interface SummaryStats {
  total: number
  online: number
  offline: number
  protected: number
  pending: number
  pendingServers: number
  todayBlocked: number
  yesterdayDelta: number
}

export interface TimelinePoint {
  date: string
  high: number
  medium: number
  info: number
}

export interface AttackerIP {
  ip: string
  country: string
  count: number
}

export interface CountryStats {
  country: string
  count: number
}

export interface AlertStats {
  topIPs: AttackerIP[]
  countries: CountryStats[]
}

export interface PortItem {
  port: number
  proto: string
  addr: string
  pid?: number
  process?: string
}

export interface ServiceItem {
  name: string
  description: string
  active: string
}

export interface PackageItem {
  name: string
  version: string
}

export interface InventoryData {
  ts: string
  ports: PortItem[]
  services: ServiceItem[]
  packages: PackageItem[]
}

