INSERT INTO hardening_items (key, category, title, plain_explanation, risk_level, default_enabled) VALUES
('ssh_no_password', 'ssh', '禁用密码登录', '禁止通过密码进行 SSH 登录，强制只允许使用 SSH 密钥验证。开启此项需要经过 5 分钟试运行，超时未确认会自动回滚。', 'high', FALSE),
('ssh_port', 'ssh', '修改 SSH 端口', '更改 SSH 服务的默认端口（22）。修改后请用新端口登录验证，若失联超时未确认，会自动回滚到 22 端口。', 'high', FALSE),
('ssh_no_root', 'ssh', '禁止 root 直接登录', '禁止 root 用户直接通过 SSH 远程登录，仅允许普通用户登录后用 sudo 提权。开启此项需要试运行，超时自动回滚。', 'high', FALSE),
('ufw', 'firewall', '启用防火墙 (UFW)', '开启基础防火墙，默认阻断所有未放行的外部入站连接。', 'med', FALSE),
('ufw_ports', 'firewall', '放行常用端口', '默认放行 SSH 端口，并开放 HTTP(80) 与 HTTPS(443) 以供 Web服务正常对外，其他未列出的端口一律禁止外部访问。', 'low', FALSE),
('fail2ban', 'bruteforce', '启用防爆破 (Fail2ban)', '监控登录日志，自动封禁在短时间内连续多次登录失败的可疑源 IP。如果系统中未安装 Fail2ban，Agent 会自动下载并配置基础防护规则。', 'med', FALSE),
('login_limit', 'bruteforce', '登录失败尝试限制', '在 SSH 级别配置最大密码重试限制。多次尝试失败后连接会被自动挂断。', 'low', FALSE),
('auto_update', 'bruteforce', '自动安全补丁更新', '启用系统后台的自动安全更新机制（如 unattended-upgrades），定期自动获取并安装操作系统和基础软件的安全补丁。', 'low', FALSE)
ON CONFLICT (key) DO UPDATE SET
    category = EXCLUDED.category,
    title = EXCLUDED.title,
    plain_explanation = EXCLUDED.plain_explanation,
    risk_level = EXCLUDED.risk_level,
    default_enabled = EXCLUDED.default_enabled;
