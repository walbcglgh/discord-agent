# Selana Discord Agent 部署指南（Ubuntu / Debian）

以下假設你已經把 `discord-agent-linux-amd64`（以及可選的 `.sha256`）放到 VPS 上，例如 `~/discord-agent-linux-amd64`。全程用 root 或 `sudo` 執行。

---

## 1. 建立目錄與安裝執行檔

```bash
sudo mkdir -p /opt/discord-agent/data/tmp
sudo install -m 0755 ~/discord-agent-linux-amd64 /opt/discord-agent/discord-agent
```

如果有一起下載 `.sha256`，才驗證；沒有就跳過：

```bash
cd ~
sha256sum -c discord-agent-linux-amd64.sha256
```

---

## 2. 建立執行帳號與設定檔

VPS 上通常只有執行檔，**不需要** `config.example.json`；直接用 `-init` 讓程式產生預設設定：

```bash
sudo id -u agent >/dev/null 2>&1 || \
  sudo useradd --system --shell /usr/sbin/nologin --home-dir /opt/discord-agent agent

sudo -u agent /opt/discord-agent/discord-agent \
  -init \
  -config /opt/discord-agent/config.json \
  -env /opt/discord-agent/.env

sudo touch /opt/discord-agent/.env
sudo chmod 0600 /opt/discord-agent/config.json /opt/discord-agent/.env
sudo chown -R agent:agent /opt/discord-agent
```

> 新設定檔裡 `llm.models` 是空的，這是正常的；下一步用面板加模型。

---

## 3. 安裝 edge-tts（要語音輸出才需要）

Debian 12 / Ubuntu 24.04 建議用 venv，避免 PEP 668 限制：

```bash
sudo apt update
sudo apt install -y python3 python3-venv
sudo python3 -m venv /opt/discord-agent/venv
sudo /opt/discord-agent/venv/bin/pip install edge-tts
sudo chown -R agent:agent /opt/discord-agent/venv
```

程式裡呼叫的是 `python3 -m edge_tts`，所以 systemd 服務要加上 venv 的 PATH（見第 5 步）。

不需要 TTS 的話可以整段略過；模型呼叫 `speak` 時會收到「伺服器沒有 python3 / edge-tts」。

---

## 4. 用一次性面板設定 Token 與模型

面板**不是服務**，用完就關：

```bash
cd /opt/discord-agent
sudo -u agent ./discord-agent \
  -panel \
  -config /opt/discord-agent/config.json \
  -env /opt/discord-agent/.env
```

> 一定要先 `cd /opt/discord-agent`：資料庫路徑是相對路徑 `data/agent.db`，從別的目錄啟動會出現 `mkdir data: permission denied`。

首次啟動會在日誌印出初始面板密碼。開啟瀏覽器：

```
http://127.0.0.1:8080
```

> 只能本機連；如果要從外面連，請自己加 SSH tunnel：`ssh -L 8080:127.0.0.1:8080 user@vps`。

在面板裡至少完成：

1. **Discord** 頁：貼上 Bot Token（或填 `DISCORD_BOT_TOKEN` 到 `.env`）。
2. **模型** 頁：新增至少一個模型：
   - Name：例如 `DeepSeek v4.1 Flash`（會出現在頁尾）
   - 協議：`openai-chat` / `openai-responses` / `anthropic` / `google`
   - 上游 Model ID、Base URL、API Key 或 API Key 環境變數名稱
   - 勾選啟用
3. 可選：**VPS**、**多媒體**、**MCP** 頁依需求填。

按完儲存後 `Ctrl+C` 結束面板。密碼若忘記：

```bash
cd /opt/discord-agent
sudo -u agent env PANEL_PASSWORD='你的新密碼' ./discord-agent \
  -reset-panel-password -config /opt/discord-agent/config.json -env /opt/discord-agent/.env
```

---

## 4.5 用 Cloudflare Tunnel 對外（建議，不用每次開 SSH tunnel）

面板本身仍只聽 `127.0.0.1:8080`，讓 cloudflared 從本機轉出去即可。程式會認 `X-Forwarded-Proto`，所以 OAuth 回呼會自動變成 `https://你的網域/mcp/oauth/callback`，Cookie 也會加 `Secure`。

### 安裝 cloudflared

```bash
curl -L --output cloudflared \
  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64
sudo install -m 0755 cloudflared /usr/local/bin/cloudflared
```

### 方案 A：Quick Tunnel（最快，網址每次重開會變）

```bash
sudo install -m 0644 deploy/cloudflared-quick.service /etc/systemd/system/cloudflared.service
sudo systemctl daemon-reload
sudo systemctl enable --now cloudflared
sudo journalctl -u cloudflared -f
```

日誌裡會出現：

```
https://xxxx-xxxx.trycloudflare.com
```

直接用瀏覽器開那個網址，帳號 `admin`，密碼同面板密碼。

> Quick Tunnel 的網址重啟後會變；若要固定網址用方案 B。

### 方案 B：Named Tunnel（固定網域）

```bash
cloudflared tunnel login
cloudflared tunnel create discord-agent-panel
cloudflared tunnel route dns discord-agent-panel panel.example.com
```

把 repo 的 `deploy/cloudflared-config.example.yml` 複製到 `/etc/cloudflared/config.yml`，替換 `<TUNNEL_UUID>` 與 hostname，然後：

```bash
sudo install -m 0644 deploy/cloudflared-named.service /etc/systemd/system/cloudflared.service
sudo systemctl daemon-reload
sudo systemctl enable --now cloudflared
```

### 安全措施（重要）

- 面板「完整設定」裡的 `panel.allowed_ips` 保持空白；Tunnel 進來的來源都是 `127.0.0.1`，填 home IP 反而會把自己鎖在外面。
- 強烈建議在 Cloudflare Zero Trust 再加一層 Access（Email OTP），即使面板密碼外洩也多一道。
- OAuth 提供者的 Redirect URI 要填實際對外網址，例如 `https://panel.example.com/mcp/oauth/callback`；MCP 頁會自動顯示目前網址。

---

## 5. 建立 systemd 服務

```bash
sudo tee /etc/systemd/system/discord-agent.service >/dev/null <<'EOF'
[Unit]
Description=Selana Discord AI Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=agent
Group=agent
WorkingDirectory=/opt/discord-agent
EnvironmentFile=/opt/discord-agent/.env
# 若安裝了 edge-tts venv，取消下一行註解：
# Environment=PATH=/opt/discord-agent/venv/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ExecStart=/opt/discord-agent/discord-agent -config /opt/discord-agent/config.json -env /opt/discord-agent/.env
Restart=on-failure
RestartSec=5

# 2C1G 保護；若部署在更大的機器可自行調高
MemoryMax=640M
CPUQuota=150%

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/opt/discord-agent/data /opt/discord-agent/config.json
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable discord-agent
sudo systemctl start discord-agent
```

看日誌：

```bash
journalctl -u discord-agent -f
```

看到類似下面這行就成功了：

```
已連線，可用模型數量：1
```

如果數量是 `0`，代表還沒在面板加模型，或模型沒啟用 / 沒金鑰。

> 若不小心執行了 `sudo -i` 變成 root shell，輸入 `exit` 就回到原本使用者；上面那些 `sudo ...` 指令在 root shell 裡也照樣能貼。

---

## 6. Discord 邀請 Bot

OAuth2 URL 用這個（權限已含踢出、封鎖、管理成員、檢視稽核日誌）：

```
https://discord.com/oauth2/authorize?client_id=<BOT_CLIENT_ID>&permissions=2201170807942&scope=bot%20applications.commands
```

面板「總覽」頁也會自動產生這顆連結。

---

## 7. 熱更新設定

Bot 本體每 3 秒檢查 `config.json`。之後要改模型、MCP、VPS、Token 等：

```bash
cd /opt/discord-agent
sudo -u agent ./discord-agent -panel -config /opt/discord-agent/config.json -env /opt/discord-agent/.env
```

改完按儲存，`Ctrl+C` 關掉面板即可，**不用重啟 Bot**。MCP 工具也會跟著重新註冊。

---

## 8. 常見問題

| 現象 | 處理 |
|---|---|
| `沒有已啟用且具備金鑰的模型` | 到面板「模型」頁新增並啟用模型，確認 API Key 或環境變數有值 |
| `尚未設定 ... 的 API key` | 填寫 API Key，或在 `.env` 設對應環境變數後重啟 |
| TTS 沒聲音 | 確認 venv 已裝 `edge-tts`，且 systemd 的 `Environment=PATH=...` 已取消註解 |
| 語音輸入沒反應 | 到面板「多媒體」頁填 Groq API Key |
| `mkdir data: permission denied` | 啟動前先 `cd /opt/discord-agent`，資料庫路徑是相對路徑 |
| `install: cannot stat '/root/discord-agent-linux-amd64'` | `sudo -i` 後 `~` 是 `/root`，不是 `admin` 家目錄；用 `find /home /tmp /root -name 'discord-agent-linux-amd64'` 找實際位置 |
| MCP OAuth 回呼失敗 | 授權服務商的 Redirect URI 要填 `http://127.0.0.1:8080/mcp/oauth/callback`（面板頁會自動顯示目前網址） |
| 面板連不上 | 面板只監聽 `127.0.0.1:8080`，請用 SSH tunnel |
| 私訊沒回覆 | 刻意設計：私訊一律不處理 |
| 多人同時 @ | 同一頻道會排隊，一個一個回 |

---

## 9. 升級

1. 下載新版本與 `.sha256`，驗證 checksum。
2. `sudo systemctl stop discord-agent`
3. 覆蓋 `/opt/discord-agent/discord-agent`，`chmod 0755`，`chown agent:agent`
4. `sudo systemctl start discord-agent`
5. `journalctl -u discord-agent -f` 確認運作

設定檔 `config.json` 不会被覆蓋。
