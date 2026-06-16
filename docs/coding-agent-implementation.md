# Guardian（服务器安全管家）· Coding Agent 实施指令

> 本文件可直接交给 AI coding agent。所有产品决策、技术选型均已拍板，**实施方不需要、也不应当再做任何产品层面的决策**。如遇文档未覆盖的细节，选择"最小实现 + 不破坏既定原则"的方案，并在该处留 `// TODO(confirm):` 注释，不要自行扩大范围。

---

## 0. 不可动摇的原则（违背即视为实现错误）

1. **自部署 · 单用户。** 本产品由使用者自己用 Docker 部署在自己的一台机器上，**没有多用户、没有注册、没有 `users` 表**。所有数据归"部署者本人"。
2. **不做账号体系，只做一道"访问口令"。** 控制台是网页，部署在公网上需要防陌生人闯入。实现一个**单一访问令牌（ACCESS_TOKEN）**：部署时写入环境变量，前端首次输入后存 localStorage，之后所有请求头携带校验。这是"开门密码"，不是登录系统。
3. **Agent-only，控制台永不持有用户的 SSH 密钥。** Agent 始终**主动向外**连控制台（出站 WSS）。控制台不对用户服务器开任何入站端口，不存任何 SSH 凭据。接入方式 = 用户在自己的 SSH 会话里粘贴一行安装命令。
4. **"绝不把你锁在门外"是产品的最高承诺，落地为"死人开关（dead-man switch）"。** 任何高风险加固（改 SSH、激进防火墙）必须：改动前快照 → 预检 → 试运行 + 倒计时 → 用户主动确认才落定，**超时或 agent 失联则自动回滚**。这条不允许为了省事而简化。
5. **为"以后可能开源"留低成本余地：** 所有可变项（访问令牌、通知配置、密钥）一律走环境变量/配置文件，**不得把任何个人信息硬编码进代码**；`install.sh` 参数化。除此之外不为开源做任何额外投入（不写文档站、不做 i18n、不搞 issue 模板）。

---

## 1. 技术选型（已拍板，禁止替换）

| 层 | 选型 |
|---|---|
| Agent（装在被管服务器上） | **Go** + `gorilla/websocket`，编译为单文件静态二进制（amd64 + arm64） |
| 控制台后端 | **Go + Gin** |
| 前端 | **React + TypeScript + Vite + Tailwind + shadcn/ui**，图表用 **Recharts** |
| 主数据库 | **PostgreSQL**（指标也存这里，24h 保留 + 定时清理，**不引入时序库**） |
| 缓存/状态 | **Redis**（agent 在线状态、命令 pub/sub、限流） |
| "人话"解释 | **静态目录预置文案为主**；动态部分用 **Anthropic Claude API** 兜底并缓存 |
| 部署 | **Docker Compose + Caddy（自动 TLS）**，单台 VPS；**不上 k8s** |
| CI | **GitHub Actions**：编译多架构 agent 二进制 + 构建镜像 |
| 认证 | 控制台：**单一 ACCESS_TOKEN**（环境变量）；Agent：一次性 enrollment token 换长期 agent token（服务端只存哈希） |

---

## 2. 系统架构

```
┌─────────────┐   出站 WSS    ┌──────────────────────────────┐
│  Agent (Go) │ ────────────► │     控制台后端 (Go/Gin)        │
│ ·指标采集    │ ◄──────────── │ ·Unlock(口令校验)             │
│ ·命令执行    │   命令下发     │ ·AgentHub(连接管理)           │
│ ·加固模块    │               │ ·Servers ·Metrics            │
│ ·快照/回滚   │               │ ·Hardening ·Alerts           │
│ ·死人开关    │               │ ·Explain(人话) ·Notify       │
└─────────────┘               └───────┬──────────────┬────────┘
                                  Postgres          Redis
                                                        ▲
                                  ┌──────────────────┐  │ REST
                                  │ Web 控制台 (React)│ ─┘
                                  └──────────────────┘
```

模块职责：**AgentHub** 维护所有 agent 的 WSS 长连接与在线状态、路由命令；**Hardening** 编排加固任务、管理死人开关倒计时与回滚；**Metrics** 接收/入库/清理指标；**Alerts** 接收 fail2ban 封禁事件并触发通知；**Explain** 给每条加固项/告警挂人话（静态优先，LLM 兜底+缓存）。

---

## 3. 数据模型（PostgreSQL，**无 users 表**）

| 表 | 关键字段 |
|---|---|
| `servers` | id, name, hostname, os, distro, arch, agent_token_hash, status(online/offline), last_seen_at, current_admin_ip, agent_version, created_at |
| `metrics` | server_id(FK), ts, cpu_pct, mem_used, mem_total, disk_used, disk_total, net_rx, net_tx, load1, uptime_sec ｜ 24h 保留 |
| `hardening_items` | key(PK), category, title, plain_explanation, risk_level(low/med/high), default_enabled ｜ 全局加固项目录（含人话） |
| `hardening_jobs` | id, server_id(FK), item_key(FK), status(pending/trial/applied/rolledback/failed), snapshot_id(FK), confirm_deadline, confirmed_at, created_at |
| `config_snapshots` | id, server_id(FK), job_id(FK), files(改动前配置归档，JSONB 或 blob), created_at |
| `security_events` | id, server_id(FK), type(bruteforce_blocked/…), source_ip, detail, plain_explanation, severity, status(new/seen/resolved), created_at |
| `notification_settings` | id(单行), channels(JSONB：email/telegram/serverchan + 各自 target), enabled ｜ 全局唯一，归部署者本人 |

关系：`servers` 1—N `metrics` / `hardening_jobs` / `security_events`；`hardening_jobs` 1—1 `config_snapshots`。

---

## 4. 接口设计

### 控制台 REST（浏览器调用，请求头带 `X-Access-Token`）

| 方法 / 路径 | 入参 | 出参 |
|---|---|---|
| `POST /api/unlock` | accessToken | 200/401（校验环境变量里的 ACCESS_TOKEN） |
| `POST /api/servers` | name | server + enrollmentToken + 一行安装命令 |
| `GET /api/servers` | — | 服务器列表（含 status + 最新指标摘要 + 安全摘要） |
| `GET /api/servers/:id` | — | 服务器详情 + 最新指标 + 安全摘要 |
| `GET /api/servers/:id/metrics?range=24h` | — | 时序点数组 |
| `GET /api/servers/:id/hardening` | — | 加固项目录 + 本机当前状态 + 人话 |
| `POST /api/servers/:id/hardening/:itemKey/apply` | — | hardening_job（高风险则进入 trial，返回 confirm_deadline） |
| `POST /api/servers/:id/hardening/jobs/:jobId/confirm` | — | job（"我还能登录"，取消死人开关回滚） |
| `POST /api/servers/:id/hardening/jobs/:jobId/rollback` | — | job（手动一键回滚） |
| `GET /api/servers/:id/alerts?status=` | status? | 告警列表（含人话） |
| `GET/PUT /api/settings/notifications` | channels | 通知配置（单行） |

### Agent 协议

| 通道 | 说明 |
|---|---|
| `POST /api/agent/enroll` | 入：enrollment_token + 主机信息(os/distro/arch)；出：长期 agent_token |
| `WSS /api/agent/ws`（auth: agent_token） | agent→server：`{type: metrics｜event｜job_result｜heartbeat}`；server→agent：`{type: command, cmd: run_hardening｜rollback｜collect_now}` |

---

## 5. 目录结构（monorepo）

```
server-guardian/
├─ agent/                  # Go
│  ├─ cmd/agent/main.go
│  └─ internal/{collector,conn,executor,hardening,snapshot,deadman}/
├─ backend/                # Go/Gin
│  ├─ cmd/server/main.go
│  └─ internal/{unlock,agenthub,servers,metrics,hardening,alerts,explain,notify,db,api}/
├─ web/                    # React+Vite+TS
│  └─ src/{pages,components/ui,api,hooks}/
├─ deploy/{docker-compose.yml,Caddyfile,Dockerfile.backend,Dockerfile.web}
├─ install.sh             # 参数化的一行安装命令模板
└─ .github/workflows/
```

---

## 6. 任务清单（按依赖顺序；标注优先级与验收标准）

> 优先级定义：**P0 = MVP 必须，缺了产品不成立；P1 = MVP 应有，体验关键；P2 = 二期。** 每个任务粒度约等于一次提交。请按编号顺序实施，编号即依赖顺序。

### M0 · 地基（P0）

**T1 — 初始化 monorepo + 基础设施**
- 内容：monorepo 结构；`deploy/docker-compose.yml` 起 postgres + redis + caddy + backend + web；后端 Gin 骨架 + `GET /api/health`。
- 依赖：无
- 验收：`docker compose up` 后，访问 `https://<域名>/api/health` 返回 200；Caddy 自动签发证书成功。

**T2 — 数据库迁移**
- 内容：用迁移工具（golang-migrate）建第 3 节全部 6 张表（无 users）。
- 依赖：T1
- 验收：迁移可正向执行与回滚；表结构与第 3 节字段完全一致。

**T3 — 前端骨架 + 解锁页 + API client**
- 内容：Vite+TS+Tailwind+shadcn 初始化；解锁页（单输入框）；统一 API client 自动附加 `X-Access-Token`；401 时跳回解锁页。
- 依赖：T1
- 验收：输入正确口令进入主框架，错误口令报错并停留；刷新后凭 localStorage 保持已解锁。

### M1 · 接入与"看见"（P0）

**T4 — 访问口令校验中间件**
- 内容：`POST /api/unlock` 校验环境变量 ACCESS_TOKEN；Gin 中间件对 `/api/*`（除 health/unlock/agent）校验请求头。
- 依赖：T1
- 验收：带正确头通过，缺失/错误返回 401；agent 路由不被此中间件拦截。

**T5 — 添加服务器：生成 enrollment token + 安装命令**
- 内容：`POST /api/servers` 创建记录并生成一次性 enrollment token，返回参数化 `install.sh` 一行命令；服务器列表页 `GET /api/servers`。
- 依赖：T2, T3, T4
- 验收：新建服务器后页面出现一张"待接入"卡片，并给出可复制的安装命令。

**T6 — Agent 骨架：enroll + 出站 WSS + 心跳**
- 内容：`agent enroll` 用 enrollment token 换 agent token 并落本地；建立出站 WSS 长连接；定时 heartbeat；`install.sh` 下载二进制并以 systemd 服务常驻。
- 依赖：T5
- 验收：在一台测试 Linux 上跑安装命令后，该服务器卡片 30s 内变为"在线"。

**T7 — AgentHub：连接登记与在线状态**
- 内容：维护 agent WSS 连接表；在线状态写 Redis（含 last_seen）；断线检测与 status 翻转。
- 依赖：T6
- 验收：拔网/停 agent 后，控制台 60s 内显示"离线"；恢复后自动"在线"。

### M2 · 控制面板（MVP 门面，P0）

**T8 — Agent 采集器**
- 内容：采集 CPU/内存/磁盘/网络 rx-tx/load1/uptime/os-distro；按固定间隔上报。
- 依赖：T6
- 验收：上报帧字段齐全且数值合理（与 `htop`/`df` 抽样比对误差可接受）。

**T9 — Metrics 入库 + 24h 清理 + 查询接口**
- 内容：接收入 `metrics`；定时任务清理 24h 前数据；`GET /metrics?range=24h`。
- 依赖：T8, T2
- 验收：连续运行 25h 后，表内最早数据不早于 24h；接口返回有序时序点。

**T10 — 仪表盘首页 + 详情页·概览 Tab**
- 内容：总览页服务器卡片网格（在线徽章 + 安全徽章 + CPU/内存/磁盘迷你条）；详情页概览 Tab（4 指标大卡 + 系统信息卡 + CPU/内存 24h MiniChart，Recharts）；含空态/加载骨架/离线态。
- 依赖：T9
- 验收：能看到真实实时指标并随时间刷新；无服务器时显示空态；离线服务器整页置灰 + 顶部黄条。

### M3 · 安全加固 + 死人开关（核心价值，P0）

**T11 — 加固项目录种子数据（含人话）**
- 内容：填充 `hardening_items`：SSH 加固（密钥登录、改端口、禁 root 密码、禁密码登录）、防火墙（ufw/nftables 基础规则）、防爆破（fail2ban）、自动安全更新（unattended-upgrades）；每项写 `plain_explanation` 与 `risk_level`。
- 依赖：T2
- 验收：`GET /hardening` 返回完整目录，高风险项标记正确。

**T12 — Agent 加固执行器 + 改动前快照**
- 内容：每个加固动作实现幂等执行；**执行前**把将被改动的文件/规则归档进 `config_snapshots`。
- 依赖：T6, T11
- 验收：执行任一加固后，对应 snapshot 存在且内容可用于还原；重复执行不产生副作用。

**T13 — 死人开关（最高优先级机制）**
- 内容：高风险 job 走 `pending → trial`，启动 `confirm_deadline` 倒计时；agent 健康检查/失联监测；超时或失联自动触发 `rollback`；`confirm` / `rollback` 接口；前端"请重新登录确认"弹窗 + 倒计时 + 自动回滚提示。
- 依赖：T12
- 验收：① 开启"禁密码登录"后进入 trial 与倒计时；② 点确认 → 落定为 applied；③ 不操作至超时 → 自动 rolledback 且配置确实还原；④ trial 期间 kill agent → 触发自动回滚。**四条全过方可视为完成。**

**T14 — 加固前预检 + 当前 IP 自动加白**
- 内容：禁密码登录前实测密钥登录可用；自动把请求来源/当前管理 IP 加入防火墙白名单并写 `servers.current_admin_ip`。
- 依赖：T12
- 验收：密钥不可用时拒绝执行并给出人话提示；加固后当前管理通道始终可达。

**T15 — 加固中心页面**
- 内容：详情页·安全加固 Tab：安全总览条 + 分组加固项列表；每行含人话、风险标识、状态（未开启/已开启/进行中/试运行待确认+倒计时/失败）。
- 依赖：T13
- 验收：各状态均能在 UI 正确呈现；高风险项开启走完整死人开关流程。

### M4 · 告警 + 人话层（P1）

**T16 — fail2ban 事件采集与告警入库**
- 内容：agent 监听 fail2ban 封禁事件 → 上报 → 写 `security_events`；详情页·告警 Tab 列表（含空态）。
- 依赖：T7, T11
- 验收：制造一次爆破（外部 IP 多次错误登录）后，能看到对应告警与来源 IP。

**T17 — Explain 服务（人话）**
- 内容：加固项/已知告警类型用静态文案；未知动态情形调 Claude API 生成并缓存。
- 依赖：T11, T16
- 验收：每条加固建议与告警都带一句通俗解释；LLM 调用有缓存命中，不重复请求同类内容。

**T18 — 通知推送**
- 内容：`notification_settings` 支持 email / Telegram / Server酱；有攻击或服务器离线时主动推送；设置页可配置与测试。
- 依赖：T16
- 验收：触发告警后，配置的通道实际收到消息；"发送测试通知"可用。

### M5 · 收尾（P1）

**T19 — 多架构构建 CI + 部署文档**
- 内容：GitHub Actions 编译 amd64/arm64 agent 二进制并发布；构建推送镜像；编写 `README` 部署步骤（含环境变量清单：ACCESS_TOKEN、数据库、Claude API Key、通知配置、域名）。
- 依赖：全部
- 验收：CI 产出两种架构二进制；按 README 在干净 VPS 上可一次性部署成功。

**T20 — 错误处理与日志基线（P1）**
- 内容：统一 API 错误结构；agent 与后端关键路径日志；加固/回滚操作审计日志。
- 依赖：全部
- 验收：常见失败（agent 离线时下发命令、口令错误、加固失败）均有清晰返回与日志，不出现裸 500/崩溃。

### 二期（P2，本次不实现，列出以免实现方误扩范围）

- 完整监控告警（历史曲线、自定义规则、长期存储/时序库）
- 运维自动化（部署 / 更新 / 备份）
- 更深的安全处理引导与一键修复
- 暗色主题（design tokens 已预留命名）
- 多用户 / SaaS 化 / 开源配套（文档站、i18n 等）

---

## 7. 待确认事项（实现方如遇以下情况，先按括号内默认做，并标注）

1. 操作系统范围：默认**优先支持主流 Debian/Ubuntu 系**，CentOS/RHEL 系尽量兼容但非阻塞。
2. 指标上报间隔：默认 **10s**（可配置）。
3. 死人开关倒计时默认时长：默认 **5 分钟**（可配置）。
4. fail2ban：若目标机未装，加固时**由 agent 负责安装并配置**。
