-- Guardian 初始化迁移 —— 严格按 docs/coding-agent-implementation.md §3
-- 自部署单用户：没有 users 表。

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 服务器
CREATE TABLE servers (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    hostname          TEXT,
    os                TEXT,
    distro            TEXT,
    arch              TEXT,
    agent_token_hash  TEXT,
    status            TEXT NOT NULL DEFAULT 'offline'
                      CHECK (status IN ('online', 'offline')),
    last_seen_at      TIMESTAMPTZ,
    current_admin_ip  TEXT,
    agent_version     TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX servers_status_idx ON servers (status);

-- 指标（24h 保留 + 定时清理）
CREATE TABLE metrics (
    server_id   TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    ts          TIMESTAMPTZ NOT NULL,
    cpu_pct     REAL NOT NULL,
    mem_used    BIGINT NOT NULL,
    mem_total   BIGINT NOT NULL,
    disk_used   BIGINT NOT NULL,
    disk_total  BIGINT NOT NULL,
    net_rx      BIGINT NOT NULL,
    net_tx      BIGINT NOT NULL,
    load1       REAL NOT NULL,
    uptime_sec  BIGINT NOT NULL
);
CREATE INDEX metrics_server_ts_idx ON metrics (server_id, ts DESC);

-- 加固项目录（全局，含人话）
CREATE TABLE hardening_items (
    key                TEXT PRIMARY KEY,
    category           TEXT NOT NULL,
    title              TEXT NOT NULL,
    plain_explanation  TEXT NOT NULL,
    risk_level         TEXT NOT NULL
                       CHECK (risk_level IN ('low', 'med', 'high')),
    default_enabled    BOOLEAN NOT NULL DEFAULT FALSE
);

-- 加固任务实例
CREATE TABLE config_snapshots (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id   TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    job_id      UUID, -- 反向引用 hardening_jobs.id，循环关系用软连接
    files       JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX config_snapshots_server_idx ON config_snapshots (server_id);

CREATE TABLE hardening_jobs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id         TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    item_key          TEXT NOT NULL REFERENCES hardening_items(key),
    status            TEXT NOT NULL DEFAULT 'pending'
                      CHECK (status IN (
                          'pending', 'trial', 'applied', 'rolledback', 'failed'
                      )),
    snapshot_id       UUID REFERENCES config_snapshots(id) ON DELETE SET NULL,
    confirm_deadline  TIMESTAMPTZ,
    confirmed_at      TIMESTAMPTZ,
    error             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX hardening_jobs_server_idx ON hardening_jobs (server_id);
CREATE INDEX hardening_jobs_status_idx ON hardening_jobs (status);

-- 安全事件
CREATE TABLE security_events (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id          TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    type               TEXT NOT NULL,
    source_ip          TEXT,
    detail             JSONB,
    plain_explanation  TEXT,
    severity           TEXT NOT NULL DEFAULT 'info'
                       CHECK (severity IN ('high', 'medium', 'info')),
    status             TEXT NOT NULL DEFAULT 'new'
                       CHECK (status IN ('new', 'seen', 'resolved')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX security_events_server_idx ON security_events (server_id, created_at DESC);
CREATE INDEX security_events_status_idx ON security_events (status);

-- 通知配置（全局单行，归部署者本人）
CREATE TABLE notification_settings (
    id        SMALLINT PRIMARY KEY DEFAULT 1
              CHECK (id = 1),
    channels  JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled   BOOLEAN NOT NULL DEFAULT TRUE
);
INSERT INTO notification_settings (id, channels, enabled) VALUES (1, '{}'::jsonb, TRUE)
ON CONFLICT (id) DO NOTHING;
