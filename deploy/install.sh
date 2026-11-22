#!/usr/bin/env bash
# 在 Ubuntu/Debian VPS 上以 root 執行：sudo bash deploy/install.sh
set -euo pipefail

APP_DIR=/opt/discord-agent
SRC_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [[ $EUID -ne 0 ]]; then
  echo "請用 root 或 sudo 執行。" >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64)  BIN=discord-agent-linux-amd64 ;;
  aarch64) BIN=discord-agent-linux-arm64 ;;
  *) echo "不支援的 CPU 架構，請自行交叉編譯。" >&2; exit 1 ;;
esac

if [[ ! -f "$SRC_DIR/dist/$BIN" ]]; then
  echo "找不到 dist/$BIN，請從 GitHub Actions 下載該執行檔放到 dist/。" >&2
  exit 1
fi

id -u agent >/dev/null 2>&1 || useradd --system --shell /usr/sbin/nologin --home-dir "$APP_DIR" agent
mkdir -p "$APP_DIR/data"

install -m 0755 "$SRC_DIR/dist/$BIN" "$APP_DIR/discord-agent"

if [[ ! -f "$APP_DIR/config.json" ]]; then
  install -m 0600 -o agent -g agent "$SRC_DIR/config.example.json" "$APP_DIR/config.json"
  echo "已建立 $APP_DIR/config.json"
fi

if [[ ! -f "$APP_DIR/.env" ]]; then
  install -m 0600 -o agent -g agent "$SRC_DIR/.env.example" "$APP_DIR/.env"
  echo "已建立 $APP_DIR/.env，請先填入 Token 與 API 金鑰再啟動。"
fi

chown -R agent:agent "$APP_DIR"
chmod 0600 "$APP_DIR/.env" "$APP_DIR/config.json"
install -m 0644 "$SRC_DIR/deploy/discord-agent.service" /etc/systemd/system/discord-agent.service

systemctl daemon-reload
systemctl enable discord-agent
echo "安裝完成。請確認 .env 後執行：systemctl start discord-agent"
echo "查看日誌：journalctl -u discord-agent -f"
