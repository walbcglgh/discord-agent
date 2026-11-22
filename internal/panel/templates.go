package panel

import "html/template"

var pages = template.Must(template.New("pages").Parse(`
{{define "head"}}
<!doctype html>
<html lang="zh-Hant"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Selana Agent · 管理面板</title>
<style>
:root{--bg:#f5f7fa;--card:#fff;--ink:#1f2937;--muted:#6b7280;--line:#e5e7eb;--accent:#4f46e5;--ok:#0f766e;--err:#b91c1c}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--ink);font-family:ui-sans-serif,system-ui,"Microsoft JhengHei",sans-serif;font-size:15px}
header{background:#fff;border-bottom:1px solid var(--line);padding:14px 28px;display:flex;align-items:center;gap:22px}
header b{font-size:17px}nav a{color:var(--muted);text-decoration:none;margin-right:16px}nav a:hover{color:var(--accent)}
main{max-width:1000px;margin:26px auto;padding:0 20px}
.card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:20px;margin-bottom:18px;box-shadow:0 1px 2px rgba(0,0,0,.03)}
h1{font-size:22px;margin:0 0 18px}h2{font-size:16px;margin:0 0 12px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(210px,1fr));gap:12px}
.kv{background:#f9fafb;border:1px solid var(--line);border-radius:8px;padding:12px}
.kv small{display:block;color:var(--muted);margin-bottom:5px}
.kv strong{font-size:15px;word-break:break-all}
label{display:block;margin:10px 0 5px;font-weight:600}
input,select,textarea{width:100%;padding:9px 11px;border:1px solid var(--line);border-radius:8px;background:#fff;color:var(--ink);font-family:inherit}
textarea{min-height:520px;font-family:ui-monospace,Consolas,monospace;font-size:12.5px;line-height:1.5}
button{margin-top:14px;padding:9px 18px;border:0;border-radius:8px;background:var(--accent);color:#fff;font-weight:600;cursor:pointer}
button:hover{opacity:.9}button.ghost{background:#eef2ff;color:var(--accent)}
.msg{background:#ecfdf5;color:var(--ok);border:1px solid #a7f3d0;padding:10px 12px;border-radius:8px;margin-bottom:14px}
.err{background:#fef2f2;color:var(--err);border:1px solid #fecaca;padding:10px 12px;border-radius:8px;margin-bottom:14px}
.hint{color:var(--muted);font-size:13px;line-height:1.7}
pre{background:#111827;color:#e5e7eb;border-radius:8px;padding:14px;overflow:auto;max-height:620px;font-size:12.5px;line-height:1.5}
a.btn{display:inline-block;margin-top:12px;color:var(--accent)}
table{width:100%;border-collapse:collapse}td,th{text-align:left;padding:8px;border-bottom:1px solid var(--line)}
</style></head><body>
{{end}}

{{define "nav"}}
<header><b>🌙 Selana Agent</b><nav>
<a href="/">總覽</a><a href="/discord">Discord</a><a href="/models">模型</a><a href="/mcp">MCP</a><a href="/vps">VPS</a><a href="/media">多媒體</a><a href="/settings">設定</a><a href="/logs">日誌</a><a href="/errors">錯誤紀錄</a>
</nav><form method="post" action="/logout" style="margin:0 0 0 auto"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="ghost" style="margin:0;padding:6px 12px">登出</button></form></header>
{{end}}

{{define "login"}}
{{template "head"}}
<body style="display:flex;align-items:center;justify-content:center;min-height:100vh">
<form method="post" action="/login" class="card" style="width:360px;margin:0">
<h1>管理面板登入</h1>
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<input type="hidden" name="csrf" value="{{.CSRF}}">
<label>帳號</label><input name="username" autocomplete="username" required>
<label>密碼</label><input type="password" name="password" autocomplete="current-password" required>
<button>登入</button>
<p class="hint">面板只監聽本機，請透過 SSH 通道連入。</p>
</form></body></html>
{{end}}

{{define "overview"}}
{{template "head"}}{{template "nav" .}}
<main>
{{if .Message}}<div class="msg">{{.Message}}</div>{{end}}
<h1>總覽</h1>
<div class="card"><h2>系統狀態</h2><div class="grid">
<div class="kv"><small>上線時間</small><strong>{{.Status.uptime}}</strong></div>
<div class="kv"><small>目前模型名稱</small><strong>{{.Status.provider}}</strong></div>
<div class="kv"><small>模型</small><strong>{{.Status.model}}</strong></div>
<div class="kv"><small>助理名稱</small><strong>{{.Status.assistant}}</strong></div>
<div class="kv"><small>回覆模式</small><strong>{{.Status.reply_mode}}</strong></div>
<div class="kv"><small>已處理訊息</small><strong>{{.Status.messages}}</strong></div>
<div class="kv"><small>Bot ID</small><strong>{{.Status.bot_id}}</strong></div>
</div>
{{if .InviteURL}}<a class="btn" href="{{.InviteURL}}">複製／開啟 OAuth2 邀請連結（最小權限）</a>{{end}}
</div>
<div class="card"><h2>安全檢查</h2><table>
<tr><th>面板監聽</th><td>{{.Config.Panel.Listen}}</td></tr>
<tr><th>允許的 IP</th><td>{{if .Config.Panel.AllowedIPs}}{{range .Config.Panel.AllowedIPs}}{{.}} {{end}}{{else}}未限制{{end}}</td></tr>
<tr><th>管理員 Discord ID</th><td>{{if .Config.Discord.AdminUserIDs}}{{range .Config.Discord.AdminUserIDs}}{{.}} {{end}}{{else}}<b style="color:var(--err)">尚未設定</b>{{end}}</td></tr>
<tr><th>白名單頻道</th><td>{{if .Config.Discord.AllowedChannelIDs}}{{range .Config.Discord.AllowedChannelIDs}}{{.}} {{end}}{{else}}未設定{{end}}</td></tr>
</table><p class="hint">API 金鑰狀態（僅顯示是否已設定，不顯示內容）：{{range $k,$v := .Env}}<br>{{$k}}：{{if $v}}<span style="color:var(--ok)">已設定</span>{{else}}<span style="color:var(--muted)">未設定</span>{{end}}{{end}}</p>
</div></main></body></html>
{{end}}

{{define "settings"}}
{{template "head"}}{{template "nav" .}}
<main>
{{if .Message}}<div class="msg">{{.Message}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<h1>設定</h1>
<div class="card"><h2>助理設定</h2>
<form method="post" action="/appearance"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label>助理名稱（出現在錯誤訊息中）</label>
<input name="assistant_name" value="{{.AssistantName}}" required>
<label>無法回覆時的訊息（可用 {assistant} 代入名稱）</label>
<input name="unavailable_message" value="{{.UnavailableMessage}}" required>
<p class="hint">下列 intermediate 內容預設全部隱藏；勾選後會以程式碼區塊附加在回覆中，供除錯用。</p>
<label style="font-weight:400"><input type="checkbox" name="display_thinking" {{if .Display.Thinking}}checked{{end}} style="width:auto"> 顯示思考過程</label>
<label style="font-weight:400"><input type="checkbox" name="display_tool_calls" {{if .Display.ToolCalls}}checked{{end}} style="width:auto"> 顯示工具調用</label>
<label style="font-weight:400"><input type="checkbox" name="display_raw_output" {{if .Display.RawOutput}}checked{{end}} style="width:auto"> 顯示模型原始輸出</label>
<button>儲存助理設定</button></form></div>
<div class="card"><h2>維護</h2>
<p class="hint">模型與 MCP 請到上方「模型」「MCP」頁設定。</p>
<form method="post" action="/commands"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="ghost">重新註冊 Slash 指令</button></form>
</div>
<div class="card"><h2>完整設定（JSON）</h2>
<p class="hint">修改後按「儲存並套用」立即生效。金鑰建議用環境變數（api_key_env），勿貼在公開場合。監聽位址需重啟程式才會生效。</p>
<form method="post" action="/settings"><input type="hidden" name="csrf" value="{{.CSRF}}">
<textarea name="config" spellcheck="false">{{.JSON}}</textarea>
<button>儲存並套用</button></form></div>
</main></body></html>
{{end}}

{{define "models"}}
{{template "head"}}{{template "nav" .}}
<main>
{{if .Message}}<div class="msg">{{.Message}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<h1>模型</h1>
<p class="hint">每個模型都是獨立上游。Name 可含大寫與空格，會直接出現在「本輪訊息由 {Name} 生成回覆」；相同 Name 的多個模型互為隨機回退。沒有預設提供商。</p>
<div class="card"><h2>已設定模型</h2>
{{if .Models}}<table><tr><th>Name</th><th>協議</th><th>上游 Model ID</th><th>Base URL</th><th>啟用</th><th></th></tr>
{{range .Models}}<tr><td>{{.Name}}</td><td>{{.Protocol}}</td><td>{{.ModelID}}</td><td>{{.BaseURL}}</td><td>{{if .Enabled}}是{{else}}否{{end}}</td>
<td><form method="post" action="/models/delete"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="ghost" style="margin:0;padding:4px 8px">刪除</button></form></td></tr>{{end}}
</table>{{else}}<p class="hint">尚未設定任何模型。</p>{{end}}
</div>
<div class="card"><h2>手動新增</h2>
<form method="post" action="/models/add"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label>Name（顯示與分組用）</label><input name="name" placeholder="DeepSeek v4.1 Flash" required>
<label>協議</label><select name="protocol">
<option value="openai-chat">OpenAI Chat Completions</option>
<option value="openai-responses">OpenAI Responses</option>
<option value="anthropic">Anthropic Messages API</option>
<option value="google">Google API</option>
</select>
<label>上游 Model ID</label><input name="model_id" placeholder="deepseek-chat" required>
<label>Base URL</label><input name="base_url" placeholder="https://api.deepseek.com/v1">
<label>API Key 環境變數名稱</label><input name="api_key_env" placeholder="DEEPSEEK_API_KEY">
<label>或直接貼 API Key（不建議）</label><input name="api_key">
<div style="display:flex;gap:16px"><label style="flex:1">Max Tokens<input name="max_tokens" value="1024"></label><label style="flex:1">Temperature<input name="temperature" value="0.7"></label></div>
<label style="font-weight:400"><input type="checkbox" name="enabled" checked style="width:auto"> 啟用</label>
<button>新增模型</button></form></div>
<div class="card"><h2>從提供商批次帶入（/v1/models）</h2>
<p class="hint">這是快捷方式，帶入後可到「完整設定」逐欄修改；不支援探索的提供商請用手動新增。</p>
<form method="post" action="/models/discover"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label>協議</label><select name="disc_protocol"><option value="openai-chat">OpenAI Chat Completions</option><option value="openai-responses">OpenAI Responses</option></select>
<label>Base URL</label><input name="disc_base_url" placeholder="https://api.example.com/v1" required>
<label>API Key 環境變數名稱</label><input name="disc_key_env" placeholder="EXAMPLE_API_KEY">
<label>或直接貼 API Key</label><input name="disc_key">
<button>查詢模型清單</button></form>
{{if .Discovered}}
<form method="post" action="/models/bulk"><input type="hidden" name="csrf" value="{{.CSRF}}">
<input type="hidden" name="count" value="{{len .Discovered}}">
<input type="hidden" name="protocol" value="{{.DiscProtocol}}">
<input type="hidden" name="base_url" value="{{.DiscBaseURL}}">
<input type="hidden" name="key_env" value="{{.DiscKeyEnv}}">
<input type="hidden" name="api_key" value="{{.DiscKey}}">
<h2>勾選並為每個模型取 Name</h2>
{{range $i, $id := .Discovered}}<div style="display:flex;gap:10px;align-items:center;margin:6px 0">
<input type="hidden" name="id_{{$i}}" value="{{$id}}">
<input type="checkbox" name="use_{{$i}}" checked style="width:auto">
<code style="flex:1">{{$id}}</code>
<input name="name_{{$i}}" value="{{$id}}" style="flex:1"></div>{{end}}
<button>批次新增</button></form>{{end}}
</div>
</main></body></html>
{{end}}

{{define "mcp"}}
{{template "head"}}{{template "nav" .}}
<main>
{{if .Message}}<div class="msg">{{.Message}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<h1>MCP 伺服器</h1>
<div class="card"><h2>已設定</h2>
{{if .Servers}}<table><tr><th>名稱</th><th>URL</th><th>驗證</th><th>啟用</th><th></th></tr>
{{range .Servers}}<tr><td>{{.Name}}</td><td>{{.URL}}</td><td>{{.AuthType}}</td><td>{{if .Enabled}}是{{else}}否{{end}}</td>
<td><form method="post" action="/mcp/toggle"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="ghost" style="margin:0;padding:4px 8px">切換</button></form>
<form method="post" action="/mcp/delete"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="ghost" style="margin:0;padding:4px 8px">刪除</button></form></td></tr>{{end}}
</table>{{else}}<p class="hint">尚未設定 MCP 伺服器。</p>{{end}}
</div>
<div class="card"><h2>新增 MCP 伺服器</h2>
<form method="post" action="/mcp/add"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label>名稱</label><input name="name" required>
<label>URL（Streamable HTTP endpoint）</label><input name="url" placeholder="https://mcp.example.com/mcp" required>
<label>驗證方式</label><select name="auth_type"><option value="none">無</option><option value="api_key">API Key</option><option value="oauth">OAuth</option></select>
<p class="hint">API Key 欄位：可用環境變數名稱，或直接填值。OAuth 欄位：access token 可用環境變數或直接填；若提供 refresh token 與 token URL，401 時會自動刷新。</p>
<label>API Key 環境變數</label><input name="api_key_env">
<label>或直接貼 API Key</label><input name="api_key">
<label>OAuth access token 環境變數</label><input name="oauth_token_env">
<label>或直接貼 OAuth access token</label><input name="oauth_access_token">
<label>OAuth refresh token</label><input name="oauth_refresh_token">
<label>OAuth token URL</label><input name="oauth_token_url">
<label>OAuth client ID</label><input name="oauth_client_id">
<label>OAuth client secret</label><input name="oauth_client_secret">
<label>OAuth scopes（逗號分隔）</label><input name="oauth_scopes">
<button>新增伺服器</button></form></div>
</main></body></html>
{{end}}

{{define "discord"}}
{{template "head"}}{{template "nav" .}}
<main>
{{if .Message}}<div class="msg">{{.Message}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<h1>Discord</h1>
<div class="card"><form method="post" action="/discord/save"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label>Bot Token 環境變數名稱</label><input name="token_env" value="{{.Discord.TokenEnv}}" autocomplete="off">
<label>或直接貼 Bot Token（留空表示沿用）</label><input name="token" type="password" autocomplete="off">
<label>回覆模式</label><select name="reply_mode">
<option value="hybrid" {{if eq .Discord.ReplyMode "hybrid"}}selected{{end}}>混合</option>
<option value="mention" {{if eq .Discord.ReplyMode "mention"}}selected{{end}}>僅 @提及</option>
<option value="channel" {{if eq .Discord.ReplyMode "channel"}}selected{{end}}>白名單頻道全部</option>
</select>
<label>管理員 User ID（逗號分隔）</label><input name="admin_user_ids" value="{{range $i,$v := .Discord.AdminUserIDs}}{{if $i}}, {{end}}{{$v}}{{end}}">
<label>白名單頻道 ID（逗號分隔）</label><input name="allowed_channel_ids" value="{{range $i,$v := .Discord.AllowedChannelIDs}}{{if $i}}, {{end}}{{$v}}{{end}}">
<label>限制伺服器 ID（逗號分隔，空白=全部）</label><input name="allowed_guild_ids" value="{{range $i,$v := .Discord.AllowedGuildIDs}}{{if $i}}, {{end}}{{$v}}{{end}}">
<label>每秒限速間隔（秒）</label><input name="rate_limit_seconds" value="{{.Discord.RateLimitSeconds}}">
<label>單則回覆上限（500~2000）</label><input name="max_reply_chars" value="{{.Discord.MaxReplyChars}}">
<p class="hint">私訊一律不處理；此選項已忽略。</p>
<label style="font-weight:400"><input type="checkbox" name="typing_indicator" {{if .Discord.TypingIndicator}}checked{{end}} style="width:auto"> 顯示正在輸入</label>
<button>儲存</button></form></div>
</main></body></html>
{{end}}

{{define "vps"}}
{{template "head"}}{{template "nav" .}}
<main>
{{if .Message}}<div class="msg">{{.Message}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<h1>VPS 指令工具</h1>
<p class="hint">這裡的連線資訊只給後台與 agent 使用，模型只會看到「一台你可以用來運行指令的 VPS」，不會看到 IP、網域名稱、連接埠、SSH key 或任何識別資訊。該 VPS 只有 12 小時，過期請用喚醒 URL 或自行喚醒。</p>
<div class="card"><form method="post" action="/vps/save"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label style="font-weight:400"><input type="checkbox" name="enabled" {{if .VPS.Enabled}}checked{{end}} style="width:auto"> 啟用</label>
<label>Host</label><input name="host" value="{{.VPS.Host}}" autocomplete="off">
<label>Port</label><input name="port" value="{{.VPS.Port}}">
<label>User</label><input name="user" value="{{.VPS.User}}">
<label>驗證方式</label><select name="auth_type"><option value="key" {{if eq .VPS.AuthType "key"}}selected{{end}}>SSH 私鑰</option><option value="password" {{if eq .VPS.AuthType "password"}}selected{{end}}>密碼</option></select>
<label>私鑰環境變數名稱</label><input name="key_env" value="{{.VPS.KeyEnv}}" autocomplete="off">
<label>或直接貼私鑰（留空表示沿用）</label><textarea name="key" style="min-height:90px" autocomplete="off"></textarea>
<label>私鑰 passphrase（留空表示沿用）</label><input name="passphrase" type="password" autocomplete="off">
<label>密碼環境變數名稱</label><input name="password_env" value="{{.VPS.PasswordEnv}}" autocomplete="off">
<label>或直接貼密碼（留空表示沿用）</label><input name="password" type="password" autocomplete="off">
<label>喚醒 URL（選用，連不上時自動呼叫）</label><input name="wake_url" value="{{.VPS.WakeURL}}" autocomplete="off">
<button>儲存</button></form></div>
</main></body></html>
{{end}}

{{define "media"}}
{{template "head"}}{{template "nav" .}}
<main>
{{if .Message}}<div class="msg">{{.Message}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<h1>多媒體</h1>
<div class="card"><h2>語音輸入（Groq whisper-large-v3）</h2>
<form method="post" action="/media/save"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label>Groq API Key 環境變數名稱</label><input name="groq_api_key_env" value="{{.Media.GroqAPIKeyEnv}}" autocomplete="off">
<label>或直接貼 Key（留空表示沿用）</label><input name="groq_api_key" type="password" autocomplete="off">
<label>音訊上限（秒）</label><input name="max_audio_seconds" value="{{.Media.MaxAudioSeconds}}">
<h2>語音輸出（edge-tts）</h2>
<label>語音</label><input name="tts_voice" value="{{.Media.TTSVoice}}">
<label>文字上限（字元，約 1 分鐘）</label><input name="max_tts_chars" value="{{.Media.MaxTTSChars}}">
<p class="hint">VPS 需自行安裝 <code>python3 -m pip install edge-tts</code>。超過上限直接回覆做不到，不會合成。</p>
<button>儲存</button></form></div>
</main></body></html>
{{end}}

{{define "logs"}}
{{template "head"}}{{template "nav" .}}
<main><h1>日誌</h1><div class="card"><pre>{{range .Lines}}{{.}}
{{end}}</pre></div></main></body></html>
{{end}}

{{define "errors"}}
{{template "head"}}{{template "nav" .}}
<main><h1>錯誤紀錄</h1><div class="card">
<p class="hint">只保存最近 50 條已去除金鑰與敏感字串的錯誤摘要，供管理員審查；一般使用者看不到這些內容。</p>
{{if .Entries}}<table><tr><th>時間</th><th>摘要</th></tr>
{{range .Entries}}<tr><td>{{.Time.Format "2006-01-02 15:04"}}</td><td>{{.Message}}</td></tr>{{end}}
</table>{{else}}<p class="hint">尚無錯誤紀錄。</p>{{end}}
</div></main></body></html>
{{end}}
`))
