# Guardian 服务器安全管家 — 实现规格（Agent Build Spec）

> 面向开发 agent 的落地规格。配套高保真界面见 `Guardian 完整界面.dc.html`（解锁 / 详情三 Tab / 设置 / 弹窗）与 `Guardian 服务器总览.dc.html`（总览首页 A/B 两套 + 移动端 + 空态）。
> 产品定位：**自部署 · 单用户 · 无账号**。数据全部留在用户自己的服务器；登录被一个「访问口令」替代。

---

## 1. 设计令牌（Design Tokens）

直接以 CSS 变量落地，全站只用这一套。

```css
:root {
  /* 表面 */
  --bg-app:        #F7F8FA;  /* 应用底色 */
  --bg-canvas:     #E9EBEF;  /* 框架外灰底（仅展示用） */
  --surface:       #FFFFFF;  /* 卡片 / 侧栏 / 顶栏 */
  --surface-soft:  #FAFBFC;  /* 表头 / 禁用输入底 */
  --border:        #E5E8EC;  /* 主边框 */
  --divider:       #EDF0F3;  /* 卡内分隔线 */

  /* 文字 */
  --text:          #1A1D23;  /* 主文字 */
  --text-2:        #5B6472;  /* 次级文字 / 说明 */
  --text-muted:    #9AA2AF;  /* 占位 / 弱化 */
  --text-faint:    #C9D2DC;  /* 箭头 / 空值 — */

  /* 主色 */
  --primary:       #2F6FED;
  --primary-hover: #1D4ED8;
  --primary-soft:  #EAF1FE;  /* 选中态底 / 次按钮底 */
  --primary-line:  #C5D8FB;  /* 次按钮描边 */

  /* 安心青绿（加固/保护语义） */
  --teal:          #14B8A6;
  --teal-deep:     #0F9488;
  --teal-soft:     #F0FAF8;
  --teal-line:     #BFE9E2;

  /* 状态 */
  --success:       #16A34A;  --success-soft: #E9F7EE;
  --warning:       #D97706;  --warning-soft: #FEF3E2;  --warning-line: #FBD9B5;  --warning-deep: #B45309;
  --danger:        #DC2626;  --danger-soft:  #FDECEC;

  /* 圆角 */
  --r-card:   10px;
  --r-btn:    6px;
  --r-input:  6px;
  --r-pill:   9999px;
  --r-frame:  14px;  /* 大容器 */

  /* 阴影（克制，仅两级） */
  --shadow-sm: 0 1px 2px rgba(16,24,40,.05);
  --shadow-lg: 0 12px 32px rgba(16,24,40,.12);
  --shadow-modal: 0 24px 48px rgba(16,24,40,.28);
}
```

### 间距
4px 基准网格。常用：`卡片内距 24`、`卡片间距 16`、`区块间距 32`、`输入/按钮纵向 10–13`。

### 字体
- 正文 / UI：**Inter**，中文回退 `'PingFang SC','Microsoft YaHei'`。字重仅 400 / 500 / 600 / 700。
- 等宽：**JetBrains Mono** — **只用于** IP、端口、版本号、运行时长、时间戳、延迟等技术读数。
- 字号尺度：标题 24–34 / 区块标题 16–21 / 正文 14–15 / 说明 13 / 标签 11–12 / 大数字 30–32（`font-variant-numeric: tabular-nums`）。

```html
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
```

---

## 2. 组件清单（Components）

| 组件 | 关键规格 |
|---|---|
| **App Shell** | 侧栏 240px（白底，右 1px 边框，含 Logo + 导航「总览/设置」+ 底部版本徽标）；顶栏 64px（白底，下 1px 边框）。 |
| **Logo** | 34×34 圆角 9px，底 `--primary`，盾牌+对勾图标白色，阴影 `0 2px 6px rgba(47,111,237,.35)`。 |
| **导航项** | 默认 `--text-2`；选中 底 `--primary-soft` + 文字 `--primary` + 600；hover 底 `#F2F4F7`。 |
| **主按钮** | 底 `--primary`，文字白，圆角 6，padding 11×16，hover `--primary-hover`。 |
| **次按钮** | 白底，描边 `--border`（或蓝向 `--primary-line`），文字 `--text-2`/`--primary`，hover 底 `#F7F8FA`/`--primary-soft`。 |
| **危险/警告按钮** | 高风险确认用 `--warning`（hover `--warning-deep`）；不用纯红做主操作，红色留给「已发生的危险」。 |
| **状态点** | 7×7 圆点：在线 `--success`、离线/危险 `--danger`/`--text-muted`。 |
| **安全徽章（Pill）** | 受保护 `--success`/`--success-soft`+盾牌；待处理 `--warning-deep`/`--warning-soft`+三角；失联 `--danger`/`--danger-soft`+点。圆角 pill，11–12px，600。 |
| **指标条（Meter）** | 轨道 `--divider` 高 5–6px 圆角 pill；填充默认 `--primary`，**≥80% 切 `--warning`**（数值同色加粗）。 |
| **开关（Toggle）** | 42×24，开 `--primary` 滑块靠右，关 `--border` 滑块靠左；滑块 20×20 白圆 + 微阴影。 |
| **输入框** | 白底，描边 `--border`，圆角 6，padding 10×14；聚焦 `--primary` + 3px 蓝光环；错误 1.5px `--danger` + 红光环 + 行内错误文案。 |
| **卡片** | 白底，1px `--border`，圆角 10，`--shadow-sm`；hover（可点）上移 2px + 边框加深 + `0 4px 12px`。 |
| **Tab** | 底部 1px 边框轨道；选中项 2px `--primary` 下划线 + 文字 `--primary` 600。 |
| **弹窗** | 遮罩 `rgba(16,24,40,.45)`；面板白底圆角 12 + `--shadow-modal`，宽约 460；危险/警告类顶部加 4px 语义色条。 |
| **空态** | 居中：96/88 圆角图标块（语义浅底）+ 标题 + 双行说明，文案安抚（"一切平静"）。 |

---

## 3. 页面与状态（Screens & States）

### 3.1 解锁页 `/unlock`
- 居中卡片：Logo + 「Guardian」+ 副标「替你盯着服务器，绝不把你锁在门外」。
- 单个**访问口令**输入（password，眼睛切换明文）+ 「进入」主按钮全宽。
- 底部徽标：绿点 +「自部署 · 单用户 · 无账号」。
- **状态**：默认 / 口令错误（输入红描边 + 行内「访问口令不正确，请重试」）。

### 3.2 服务器总览 `/`
- 顶栏：服务器筛选下拉（"全部服务器 · 6 台"）、在线计数、告警铃铛（有未读时红点）。
- 页头：标题「我的服务器」+ 副行「N 台 · M 在线 · 今日拦截 X 次」+「添加服务器」主按钮。
- **布局 A（默认推荐）**：服务器卡片网格 `repeat(auto-fill, minmax(300px,1fr))`，gap 16。每卡含：名称 + IP(mono) + 在线点；安全徽章；CPU/内存/磁盘 三条 Meter；页脚「今日挡下 N 次攻击」。
- **布局 B（可选/高密度）**：顶部 4 个汇总指标卡（总数 / 受保护 / 待处理 / 今日拦截）+ 表格式列表（列：服务器、状态、CPU、内存、磁盘、安全、今日拦截、`>`）。
- **状态**：正常 / 待处理（橙徽章 + Meter ≥80% 转橙）/ 离线危险（灰名 + 红徽章「失联 · 危险」+ 指标置灰为 —）/ **空态**（无服务器引导）。
- **响应式**：<768px 单列卡片；侧栏收为顶栏汉堡 → 左抽屉（遮罩 `rgba(16,24,40,.45)`）。

### 3.3 服务器详情 `/server/:id`
顶部：返回 + 服务器名 + IP(mono) + 在线状态；下方 3 Tab。

**Tab 1 · 概览**
- 4 指标大卡（CPU/内存/磁盘大数字 + 正常徽章 + Meter；网络 ↑↓ MB/s 实时）。
- 系统信息条（发行版 / 内核 / 运行时长 / Agent 版本，全 mono）。
- 两张 24h 趋势面积图（CPU 蓝、内存青绿；标注峰值/均值）。

**Tab 2 · 安全加固（核心）**
- 顶部加固进度条：`已完成 N/总数` + 青绿进度 + 「今日已挡下 X 次攻击」。
- 分组开关列表：**SSH 安全 / 防火墙 / 防爆破 & 更新**。每项 = 标题 + 大白话说明 + 右侧开关（或值徽章如 `:49222`、`22·80·443`、`今日封禁 7 IP`）。
- **高风险项**特殊态（如「禁用密码登录」）：卡片橙左边框 + 「高风险」徽章；启用走**试运行**：显示**自动回滚倒计时**（如 04:32）+「去验证」按钮。
- 开关三态：已开启（蓝）/ 未开启（灰）/ 不可用（置灰）。

**Tab 3 · 告警**
- 过滤切片：全部 / 未处理 · N。
- 时间线列表项：时间(mono) + 标题 + 来源 IP(mono) + 大白话说明 + 危险度徽章（高危红 / 中橙 / 提示灰）。
- 未读：左 3px 语义色边框 + 标题旁蓝点；已读：降透明度、左边框透明；已解决：绿「已解决」徽章 + 进一步降透明度。
- **空态**：青绿盾牌 +「一切平静 / 过去 24 小时没有任何告警」。

### 3.4 设置 `/settings`
仅三块（自部署单用户，无团队/角色/计费）：
1. **通知配置** — Email / Telegram / Server酱 行：标签 + 输入 + 「测试」次按钮 + 开关。未配置项整行弱化、测试按钮禁用。
2. **访问口令** — 当前 / 新 / 确认 三个 password 输入 + 「更新口令」。
3. **关于** — 控制台版本 / Agent 版本（含在线数）/ 开源仓库链接（外链图标）/ 隐私声明行（青绿盾牌 +「数据全部留在你的服务器」）。

### 3.5 弹窗（Modals）
1. **添加服务器** — 名称(可选) + 一行安装命令（深色代码块 + 复制按钮）+ 「等待服务器连接…」轮询态 + 取消/完成。
2. **高风险确认（死人开关）** — 顶部橙条；标题「确认禁用密码登录？」；说明**试运行 5 分钟**机制；青绿信息框强调「不会把你锁在外面，超时自动回滚」；勾选「我已确认能用 SSH 密钥登录」后启用主操作「试运行并启动倒计时」（橙）。
3. **验证超时 / 已自动回滚** — 顶部红条；标题「没收到你的确认，已自动回滚」；展示回滚结果（密码登录已恢复 / SSH 连通性正常）；动作「知道了 / 重试」。

---

## 4. 交互与文案准则

- **死人开关（核心安全保证）**：任何可能切断访问的加固（禁用密码登录、改 SSH 端口、防火墙规则）必须走 `应用 → 倒计时验证窗口(默认 5min) → 未确认则自动回滚`。倒计时与「去验证 / 确认安全」入口需在加固 Tab 与全局可见。
- **颜色语义严格**：绿/青绿=安全已就绪；橙=需注意/待处理/高风险待确认；红=已发生的危险或失联。指标 Meter ≥80% 自动转橙。
- **文案双层**：技术标签（如 fail2ban、UFW、root）保留，但每项配一句**大白话后果说明**（"挡住绝大多数攻击"、"让扫描机器人扑空"）。
- **数字用 mono + tabular-nums**，避免跳动。
- **可访问性**：命中区 ≥44px（移动端）；状态不靠颜色单一传达（点 + 文字 + 图标并用）；输入错误同时给红框 + 文案。

---

## 5. 建议数据模型（前端契约）

```ts
type ServerStatus = 'online' | 'offline';
type SecurityState = 'protected' | 'pending' | 'danger';

interface Server {
  id: string;
  name: string;            // web-prod-01
  ip: string;              // 45.32.18.207
  status: ServerStatus;
  security: SecurityState;
  pendingCount?: number;   // 待处理项数
  metrics: { cpu: number; mem: number; disk: number; netUp: number; netDown: number }; // 0–100 / MB/s
  attacksBlockedToday: number;
  lastSeen?: string;       // 离线时：'12 分钟前'
  system?: { distro: string; kernel: string; uptime: string; agent: string };
}

interface Hardening {
  key: 'ssh_no_password'|'ssh_port'|'ssh_no_root'|'ufw'|'ufw_ports'|'fail2ban'|'login_limit'|'auto_update';
  group: 'ssh'|'firewall'|'bruteforce';
  enabled: boolean;
  highRisk?: boolean;
  value?: string;          // 端口 / 放行列表 等
  trial?: { rollbackAt: string };  // 试运行回滚时刻
}

interface Alert {
  id: string;
  ts: string;              // ISO
  kind: 'bruteforce'|'port_scan'|'new_login'|'...';
  sourceIp?: string;
  severity: 'high'|'medium'|'info';
  read: boolean;
  resolved: boolean;
  message: string;         // 大白话说明
}

interface Settings {
  notify: { email?: string; telegram?: string; serverChan?: string;
            enabled: { email: boolean; telegram: boolean; serverChan: boolean } };
  version: { console: string; agent: string; agentsOnline: string };
}
```

---

## 6. 交付物清单
- `Guardian 服务器总览.dc.html` — 总览首页（布局 A 卡片网格 / 布局 B 摘要+列表）+ 移动端（默认 + 汉堡抽屉）+ 空态。
- `Guardian 完整界面.dc.html` — 解锁（默认/错误）、详情（概览/安全加固/告警 + 告警空态）、设置、三弹窗。
- `SPEC.md` — 本文档。

> 实现优先级：解锁 → 总览（布局 A）→ 详情·安全加固（含死人开关）→ 告警 → 设置 → 弹窗 → 布局 B / 响应式细节。
