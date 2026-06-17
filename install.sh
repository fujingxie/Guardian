#!/usr/bin/env bash
# Guardian Agent 一行安装命令模板
# 用法（由控制台「添加服务器」按钮生成实际值）：
#   curl -fsSL https://<domain>/install.sh | sudo bash -s -- \
#     --token <enrollment-token> \
#     --console https://<domain>
#
# T6 落地后正式实现。当前为占位 stub，仅打印参数以便回路联调。

set -euo pipefail

TOKEN=""
CONSOLE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --token)   TOKEN="$2"; shift 2 ;;
        --console) CONSOLE="$2"; shift 2 ;;
        *) echo "未知参数：$1" >&2; exit 64 ;;
    esac
done

if [[ -z "$TOKEN" || -z "$CONSOLE" ]]; then
    echo "用法：install.sh --token <enrollment-token> --console <https://domain>" >&2
    exit 64
fi

echo "[guardian] T6 阶段会在此下载 agent 二进制并落 systemd 服务"
echo "[guardian] token=$TOKEN console=$CONSOLE"
echo "[guardian] 这是个 stub，不要在生产机上执行。"
