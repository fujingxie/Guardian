#!/usr/bin/env bash
# Guardian Agent 一键安装脚本
# 用法（由控制台「添加服务器」按钮生成实际值）：
#   curl -fsSL http://<domain>/install.sh | sudo bash -s -- \
#     --token <enrollment-token> \
#     --console http://<domain> \
#     [--insecure]

set -euo pipefail

TOKEN=""
CONSOLE=""
INSECURE="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --token)      TOKEN="$2"; shift 2 ;;
        --console)    CONSOLE="$2"; shift 2 ;;
        --insecure)   INSECURE="true"; shift 1 ;;
        *) echo "未知参数：$1" >&2; exit 64 ;;
    esac
done

if [[ -z "$TOKEN" || -z "$CONSOLE" ]]; then
    echo "用法：install.sh --token <enrollment-token> --console <http://domain> [--insecure]" >&2
    exit 64
fi

# 1. 检测系统架构
ARCH=$(uname -m)
AGENT_ARCH=""
case "$ARCH" in
    x86_64|amd64) AGENT_ARCH="amd64" ;;
    aarch64|arm64) AGENT_ARCH="arm64" ;;
    *) echo "[guardian] 不支持的 CPU 架构：$ARCH" >&2; exit 1 ;;
esac

echo "[guardian] 检测到系统架构为：$AGENT_ARCH"

# 2. 拼接下载地址并下载 Agent 二进制
DOWNLOAD_URL="${CONSOLE}/api/agent/download?arch=${AGENT_ARCH}"
CURL_OPTS="-fsSL"
if [ "$INSECURE" = "true" ]; then
    CURL_OPTS="-fsSLk"
fi

echo "[guardian] 正在从控制台下载 Agent 二进制文件..."
mkdir -p /usr/local/bin
curl $CURL_OPTS -o /usr/local/bin/guardian-agent "$DOWNLOAD_URL"
chmod +x /usr/local/bin/guardian-agent

# 3. 创建 Systemd 单元文件
echo "[guardian] 正在配置 Systemd 服务..."
INSECURE_FLAG=""
if [ "$INSECURE" = "true" ]; then
    INSECURE_FLAG="--insecure"
fi

# 创建状态配置目录（提前创建，以便 Service 启动时能正常读写）
mkdir -p /var/lib/guardian

cat <<EOF > /etc/systemd/system/guardian-agent.service
[Unit]
Description=Guardian Agent
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/guardian
Environment=GUARDIAN_STATE_DIR=/var/lib/guardian
ExecStart=/usr/local/bin/guardian-agent --token ${TOKEN} --console ${CONSOLE} ${INSECURE_FLAG}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

# 4. 重新加载并启动守护进程
echo "[guardian] 正在加载并启动守护进程..."
systemctl daemon-reload
systemctl enable guardian-agent
systemctl start guardian-agent

echo "[guardian] Agent 安装配置成功，并已作为 Systemd 守护进程在后台运行。"
echo "[guardian] 您可以运行 'systemctl status guardian-agent' 查看服务状态。"
