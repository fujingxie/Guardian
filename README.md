# Guardian (服务器安全管家)

Guardian 是一款专门为极客、中小网站站长及独立开发者设计的**轻量级自部署服务器安全管理工具**。

通过与服务器上极简、低开销的 Agent 建立长连接，Guardian 能够为您提供实时的系统资源指标监测、Fail2ban 暴力破解拦截上报，并为您提供内置双重防锁死回滚（死人开关）机制的 SSH 安全加固、UFW 防火墙防护，以及 AI 驱动的安全事件“大白话”人话翻译与多通道通知（微信、邮件、Telegram）推送。

## 🌟 核心亮点

1. **自部署 · 单用户**：没有复杂的团队注册与多租户隔离，整套系统完全部署在您自己掌控的容器中，所有数据归您个人所有。
2. **极简安全认证**：控制台不设立账号体系，仅通过单一的访问令牌 (`ACCESS_TOKEN`) 进行登录认证；Agent-only 模式下，控制台**绝不持有且不要求**服务器的 SSH 私钥，Agent 始终主动向外建立出站 WebSocket 连接。
3. **“绝不把你锁在门外”的最高承诺 (死人开关)**：任何涉及高风险的安全加固（如修改 SSH 端口、关闭密码登录等）会经过 5 分钟的试运行。如果配置有误导致您和 Agent 失联，Agent 会在本地**自动执行强行回滚**，恢复原始配置并重启服务，彻底防止配置手抖导致服务器失联。
4. **AI 人话翻译与 IP 归属地定位**：自动将晦涩的爆破、扫描系统日志翻译为通俗的大白话（集成 DeepSeek / Claude API），并能够自动显示攻击源 IP 的全球物理定位（如 `美国`、`中国北京` 等）。
5. **多通道秒级推送**：一旦发生拦截或服务器失联掉线，支持将通知秒级推送到您的发信邮箱、微信（方糖 Server酱）或 Telegram 机器人。

---

## 🚀 极速部署指南

### 第一步：创建配置文件
在控制台所在的服务器上创建部署目录，并建立 `deploy/.env` 配置文件（可参考 `deploy/.env.example`）：
```env
# 控制台登录开门令牌（建议修改为复杂的随机字符串）
ACCESS_TOKEN=guardian-demo-2026

# 控制台对外暴露的域名或裸 IP (例如 38.76.166.42)
GUARDIAN_DOMAIN=38.76.166.42

# 拼装一键加机脚本的外网控制台基地址 (重要：需加上 Caddy 对外的映射端口)
CONSOLE_BASE_URL=http://38.76.166.42:18081

# 数据库凭据 (使用默认即可，Postgres 容器会自动创建对应的库)
POSTGRES_USER=guardian
POSTGRES_PASSWORD=guardian
POSTGRES_DB=guardian

# 死人开关倒计时（分钟）
TRIAL_MINUTES=5

# AI 人话翻译服务 API 秘钥（支持 DeepSeek 或 Claude）
DEEPSEEK_API_KEY=您的DeepSeekAPIKey
# ANTHROPIC_API_KEY=如果您使用Claude

# 发信邮箱 SMTP 配置（开启邮件报警通知时填写，支持 QQ/网易/Gmail）
SMTP_HOST=smtp.qq.com
SMTP_PORT=587
SMTP_USER=您的发信账号@qq.com
SMTP_PASS=您的邮箱发信授权码
SMTP_FROM=您的发信账号@qq.com
```

### 第二步：使用 Docker Compose 一键拉起控制台
在 `deploy` 目录下运行：
```bash
docker compose up -d
```
服务启动后，在浏览器中打开：`http://<您的IP>:18081`，输入您在 `.env` 中配的 `ACCESS_TOKEN` 解锁口令，即可进入控制台大盘！

---

## 🖥️ 新服务器一键加机（Agent 部署）

1. 进入控制台网页，点击右上角 **“添加服务器”**。
2. 输入服务器别名（如 `ubuntu-node`），点击确认。
3. 拷贝网页上自动生成的一键安装命令，例如：
   ```bash
   curl -fsSL http://<ip>:18081/install.sh | sudo bash -s -- --token enroll_xxx --console http://<ip>:18081 --insecure
   ```
4. 登录到您的待加固服务器终端，粘贴该命令并按回车。
5. 安装程序会自动探测 CPU 架构（amd64 / arm64），自动下载适配的静态 Agent，并自动将其注册为 Systemd 系统守护进程并在后台稳定运行。
6. 约 5 秒钟后，刷新网页大盘，该服务器即呈现在线，并开始上报监控与拦截日志！

---

## 🔧 常见故障排查 (Q&A)

### Q: 为什么一键加机命令在新服务器执行报错返回 HTML 代码？
**A**: 这是因为您在控制台修改了 `deploy/Caddyfile` 的配置，但是 Docker 并没有重构/重启 Caddy。请在控制台服务器上运行 `docker compose restart caddy` 强制重启网关服务即可解决。

### Q: 为什么我修改了 SSH 密码登录，但还是可以输入密码登录？
**A**: 请确保您刚才的加固任务在网页上点击了 **“确认加固”**。如果未在 5 分钟内点击确认，系统为了防止您失联，已经全自动将配置回滚了。

### Q: 如何查看 Agent 本地的日志？
**A**: 登录加固机器，运行以下命令即可查阅 Agent 的本地输出与自动回滚运行记录：
```bash
journalctl -u guardian-agent -f
```
