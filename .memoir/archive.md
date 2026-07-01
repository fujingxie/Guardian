## 项目定位

Guardian 是一个服务器安全监控与加固管理平台。它解决的核心问题是：在多台 Linux 服务器的运维场景中，集中监控服务器安全状态（如失败登录、攻击告警）、执行安全加固脚本、管理探针 Agent 的生命周期。项目采用 C/S 架构，后端提供 REST API，前端为 Web 管理面板，Agent 部署在被监控的服务器上。

## 技术栈与设计

- **后端**：Go + Gin，PostgreSQL（数据持久化），Redis（在线状态/缓存/限流），Caddy（反向代理/TLS 终止）
- **前端**：React + TypeScript + Vite + Tailwind，React Router 路由，手写 UI 组件，MSW（Mock Service Worker 用于开发联调）
- **Agent**：Go 单二进制，通过出站 WebSocket 主动连接后端，执行指标采集、Fail2Ban 日志采集、系统画像上报、Deadman 检测、安全加固脚本（基于 shell 命令下发）
- **部署**：Docker Compose（后端 + Caddy + PostgreSQL + Redis），Agent 独立二进制，install.sh 为自动化安装脚本

1. **注册制 Agent 管理**：Agent 首次运行通过 enroll 接口注册，后端生成唯一 token，后续通信靠 token 认证。
2. **Agent 出站长连接**：Agent 通过 WSS 主动连接控制台，后端经 AgentHub 下发命令，Agent 上报心跳、指标、告警、系统画像与任务结果。
3. **安全加固策略集中下发**：后端定义加固项（hardening_items 表），Agent 执行后上报结果，前端可见进度。
4. **通知与解释分离**：notify service 负责发送告警（邮件/Webhook），explain service 提供安全事件的解释文本（供前端展示）。通知配置的 `channels.alertTypes` 控制哪些告警类型主动推送；关闭后事件仍写入告警列表。
5. **前端路由与状态**：使用 React Router 管理页面路由，页面内用 React state 管理视图状态。
6. **迁移文件为纯 SQL**：使用 golang-migrate 或自定义 migrate 工具执行 up/down。

## 运行部署运维

```bash
cd deploy
cp .env.example .env  # 编辑配置
docker compose up -d
```
- docker-compose.yml 启动 backend、caddy、postgres、redis
- Caddy 自动处理 TLS（需配置域名）或作为纯反向代理
- Agent 不在容器内运行，需在目标服务器执行 `install.sh`

- 监控 agent 心跳/在线状态（通过 agent heartbeat 接口）
- 定期检查 hardening 任务执行状态（DB 中的任务表）
- 前端 mock 数据在生产环境应关闭（设置 VITE_USE_MOCKS=false）

## 待办与已知问题

- [ ] Agent 自动更新机制（当前需手动替换 agent_bin）
- [ ] 前端用户认证/登录功能（目前可能无鉴权或使用简单 token）
- [ ] 通知渠道配置界面（当前可能只在后端配置）
- [ ] 安全加固项的编排/回滚能力
- [ ] 单元测试/集成测试覆盖

- Agent 状态上报可能存在并发冲突（state.json 文件锁未实现）
- 前端 mock 数据与真实 API 返回格式不一致，切换时需注意
- 部署目录 deploy/ 下的 Caddyfile 与 Caddyfile.web 可能存在冲突，需要确认使用哪个
- 没有显式的日志采集策略（Agent 直接读取 fail2ban 日志，路径硬编码）
