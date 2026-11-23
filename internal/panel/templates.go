package panel

import "html/template"

var pages = template.Must(template.New("pages").Parse(`
{{define "head"}}<!doctype html>
<html lang="zh-Hant"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Selana Agent · 管理面板</title>
<script src="https://cdn.tailwindcss.com"></script>
<script>
if (window.tailwind) {
  tailwind.config = { theme: { extend: { fontFamily: { sans: ['ui-sans-serif','system-ui','Microsoft JhengHei','sans-serif'] } } } };
}
</script>
<style>
dialog::backdrop{background:rgba(15,23,42,.6)}
.ui-dialog{width:100%;max-width:min(94vw,720px);border:0;padding:0;border-radius:16px;box-shadow:0 24px 60px rgba(15,23,42,.28)}
.ui-dialog:not([open]){display:none}
pre{white-space:pre-wrap;word-break:break-word}
</style>
</head><body class="min-h-screen bg-slate-100 text-slate-800 antialiased">
{{end}}

{{define "login"}}
{{template "head"}}
<main class="flex min-h-screen items-center justify-center px-5">
<form method="post" action="/login" class="w-full max-w-sm rounded-2xl border border-slate-200 bg-white p-7 shadow-sm">
<h1 class="text-xl font-semibold text-slate-900">管理面板登入</h1>
{{if .Error}}<div class="mt-4 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{{.Error}}</div>{{end}}
<input type="hidden" name="csrf" value="{{.CSRF}}">
<label class="mt-5 block text-sm font-medium text-slate-700">帳號
<input name="username" autocomplete="username" required class="mt-1.5 w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"></label>
<label class="mt-4 block text-sm font-medium text-slate-700">密碼
<input name="password" type="password" autocomplete="current-password" required class="mt-1.5 w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"></label>
<button class="mt-6 w-full rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">登入</button>
<p class="mt-4 text-xs leading-relaxed text-slate-500">若透過 Cloudflare Tunnel 對外，建議再加一層 Access 保護。</p>
</form></main></body></html>
{{end}}

{{define "console"}}
{{template "head"}}
<header class="border-b border-slate-200 bg-white">
<div class="mx-auto flex max-w-5xl items-center justify-between px-5 py-4">
<div class="flex items-center gap-2 font-semibold text-slate-900"><span class="text-xl">🌙</span> Selana Agent</div>
<div class="flex items-center gap-3">
<span class="hidden text-xs text-slate-500 sm:inline">{{if .Status.bot_id}}Bot 已設定{{else}}尚未設定 Bot{{end}}</span>
<form method="post" action="/logout" class="m-0"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="rounded-lg border border-slate-200 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-50">登出</button></form>
</div></div>
</header>

<main class="mx-auto max-w-5xl px-5 py-10">
<div class="mb-8">
<h1 class="text-2xl font-semibold tracking-tight text-slate-900">控制台</h1>
<p class="mt-1 text-sm text-slate-500">點選項目開啟設定，完成後再啟動 Bot。</p>
</div>

<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
<button type="button" data-modal="discord" class="group rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-indigo-300 hover:shadow-md">
<span class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-50 text-lg">🤖</span>
<span class="mt-3 block font-medium text-slate-900">Discord Bot</span>
<span class="mt-1 block text-xs text-slate-500">Token、管理員與回覆模式</span></button>
<button type="button" data-modal="models" class="group rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-indigo-300 hover:shadow-md">
<span class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-violet-50 text-lg">🧠</span>
<span class="mt-3 block font-medium text-slate-900">模型</span>
<span class="mt-1 block text-xs text-slate-500">目前 {{len .Models}} 個上游</span></button>
<button type="button" data-modal="mcp" class="group rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-indigo-300 hover:shadow-md">
<span class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-sky-50 text-lg">🔌</span>
<span class="mt-3 block font-medium text-slate-900">MCP</span>
<span class="mt-1 block text-xs text-slate-500">外部工具與 OAuth 授權</span></button>
<button type="button" data-modal="vps" class="group rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-indigo-300 hover:shadow-md">
<span class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-50 text-lg">🖥️</span>
<span class="mt-3 block font-medium text-slate-900">VPS</span>
<span class="mt-1 block text-xs text-slate-500">指令工具連線</span></button>
<button type="button" data-modal="media" class="group rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-indigo-300 hover:shadow-md">
<span class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-amber-50 text-lg">🎙️</span>
<span class="mt-3 block font-medium text-slate-900">語音</span>
<span class="mt-1 block text-xs text-slate-500">轉錄與語音輸出</span></button>
<button type="button" data-modal="settings" class="group rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-indigo-300 hover:shadow-md">
<span class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-slate-100 text-lg">⚙️</span>
<span class="mt-3 block font-medium text-slate-900">助理設定</span>
<span class="mt-1 block text-xs text-slate-500">名稱、顯示與進階 JSON</span></button>
<button type="button" data-modal="logs" class="group rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-indigo-300 hover:shadow-md">
<span class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-slate-100 text-lg">📄</span>
<span class="mt-3 block font-medium text-slate-900">日誌</span>
<span class="mt-1 block text-xs text-slate-500">唯讀，最近 200 行</span></button>
<button type="button" data-modal="errors" class="group rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-indigo-300 hover:shadow-md">
<span class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-slate-100 text-lg">⚠️</span>
<span class="mt-3 block font-medium text-slate-900">錯誤紀錄</span>
<span class="mt-1 block text-xs text-slate-500">唯讀，最近 50 則</span></button>
</div>

{{if .InviteURL}}<div class="mt-8 rounded-2xl border border-slate-200 bg-white p-5 text-sm">
<div class="font-medium text-slate-900">邀請 Bot 加入伺服器</div>
<p class="mt-1 text-xs text-slate-500">使用最小權限，完成設定後再點下面的連結。</p>
<a href="{{.InviteURL}}" class="mt-3 inline-block rounded-lg border border-indigo-200 bg-indigo-50 px-3 py-2 text-xs font-medium text-indigo-700 hover:bg-indigo-100">開啟 OAuth2 邀請連結</a>
</div>{{end}}

<div class="mt-8 rounded-2xl border border-indigo-100 bg-indigo-50 p-5">
<div class="text-sm font-medium text-slate-900">完成設定</div>
<p class="mt-1 text-xs text-slate-600">面板會關閉，Bot 在同一個程序裡啟動。</p>
<form method="post" action="/finish" onsubmit="return confirm('確定完成設定並啟動 Bot？面板將會關閉。')" class="m-0"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="mt-4 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">完成並啟動 Bot</button></form>
</div>
</main>

<dialog id="m-discord" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">Discord Bot</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div>
<form method="post" action="/discord/save" class="space-y-4 px-6 py-5"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label class="block text-sm font-medium text-slate-700">Bot Token
<input name="token" type="password" autocomplete="off" placeholder="留空表示沿用" class="mt-1.5 w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"></label>
<p class="text-xs text-slate-500">目前環境變數名稱：{{.Discord.TokenEnv}}</p>
<label class="block text-sm font-medium text-slate-700">回覆模式
<select name="reply_mode" class="mt-1.5 w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm"><option value="hybrid" {{if eq .Discord.ReplyMode "hybrid"}}selected{{end}}>混合（白名單頻道全部 + 其他頻道 @提及）</option><option value="mention" {{if eq .Discord.ReplyMode "mention"}}selected{{end}}>僅 @提及</option><option value="channel" {{if eq .Discord.ReplyMode "channel"}}selected{{end}}>白名單頻道全部</option></select></label>
<label class="block text-sm font-medium text-slate-700">管理員 User ID
<input name="admin_user_ids" value="{{range $i,$v := .Discord.AdminUserIDs}}{{if $i}}, {{end}}{{$v}}{{end}}" placeholder="逗號分隔" class="mt-1.5 w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm"></label>
<details class="rounded-xl border border-slate-200 bg-slate-50 p-3"><summary class="cursor-pointer text-sm font-medium text-slate-700">進階</summary><div class="mt-3 space-y-3">
<label class="block text-xs font-medium text-slate-600">白名單頻道 ID（逗號分隔）
<input name="allowed_channel_ids" value="{{range $i,$v := .Discord.AllowedChannelIDs}}{{if $i}}, {{end}}{{$v}}{{end}}" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">限制伺服器 ID（空白=全部）
<input name="allowed_guild_ids" value="{{range $i,$v := .Discord.AllowedGuildIDs}}{{if $i}}, {{end}}{{$v}}{{end}}" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<div class="grid grid-cols-2 gap-3"><label class="block text-xs font-medium text-slate-600">限速間隔（秒）<input name="rate_limit_seconds" value="{{.Discord.RateLimitSeconds}}" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">回覆上限<input name="max_reply_chars" value="{{.Discord.MaxReplyChars}}" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label></div>
<label class="block text-xs font-medium text-slate-600">Token 環境變數名稱<input name="token_env" value="{{.Discord.TokenEnv}}" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="typing_indicator" {{if .Discord.TypingIndicator}}checked{{end}} class="h-4 w-4 rounded border-slate-300"> 顯示正在輸入</label>
<p class="text-xs text-slate-500">私訊一律不處理。</p>
</div></details>
<button class="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">儲存</button></form></dialog>

<dialog id="m-models" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">模型</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div>
<div class="space-y-5 px-6 py-5">
<div><h3 class="text-sm font-medium text-slate-900">已設定（{{len .Models}}）</h3>
{{if .Models}}<div class="mt-2 divide-y divide-slate-100 rounded-xl border border-slate-200">{{range .Models}}<div class="flex items-center gap-3 px-3 py-2.5 text-sm"><span class="flex-1 truncate font-medium text-slate-800">{{.Name}}</span><span class="hidden truncate text-xs text-slate-500 sm:block">{{.ModelID}}</span><span class="text-xs text-slate-400">{{if .Enabled}}啟用{{else}}停用{{end}}</span><form method="post" action="/models/delete" class="m-0"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="rounded-lg px-2 py-1 text-xs text-rose-600 hover:bg-rose-50">刪除</button></form></div>{{end}}</div>{{else}}<p class="mt-2 text-xs text-slate-500">尚未設定模型，Bot 目前無法回覆。</p>{{end}}</div>

<form method="post" action="/models/add" class="space-y-3 border-t border-slate-100 pt-5"><input type="hidden" name="csrf" value="{{.CSRF}}">
<h3 class="text-sm font-medium text-slate-900">新增模型</h3>
<div class="flex gap-2"><select id="model_preset" class="flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm"><option value="">自訂</option><option value="deepseek">DeepSeek</option><option value="openai">OpenAI</option><option value="anthropic">Anthropic</option><option value="gemini">Gemini</option><option value="groq">Groq</option></select><button type="button" onclick="applyPreset()" class="rounded-lg border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50">帶入</button></div>
<div class="grid gap-3 sm:grid-cols-2"><label class="block text-xs font-medium text-slate-600">Name<input name="name" placeholder="DeepSeek V4.1 Flash" required class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">Model ID<input name="model_id" placeholder="deepseek-chat" required class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label></div>
<label class="block text-xs font-medium text-slate-600">Base URL<input name="base_url" placeholder="https://api.deepseek.com/v1" class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">API Key 環境變數<input name="api_key_env" placeholder="DEEPSEEK_API_KEY" class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<details class="rounded-xl border border-slate-200 bg-slate-50 p-3"><summary class="cursor-pointer text-sm font-medium text-slate-700">進階</summary><div class="mt-3 space-y-3">
<label class="block text-xs font-medium text-slate-600">協議<select name="protocol" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"><option value="openai-chat">OpenAI Chat Completions</option><option value="openai-responses">OpenAI Responses</option><option value="anthropic">Anthropic Messages API</option><option value="google">Google API</option></select></label>
<label class="block text-xs font-medium text-slate-600">直接貼 API Key（不建議）<input name="api_key" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<div class="grid grid-cols-2 gap-3"><label class="block text-xs font-medium text-slate-600">Max Tokens<input name="max_tokens" value="1024" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label><label class="block text-xs font-medium text-slate-600">Temperature<input name="temperature" value="0.7" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label></div>
<label class="flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="enabled" checked class="h-4 w-4 rounded border-slate-300"> 啟用</label></div></details>
<button class="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">新增模型</button></form>

<details class="rounded-xl border border-slate-200 bg-slate-50 p-3"><summary class="cursor-pointer text-sm font-medium text-slate-700">從提供商自動抓取模型清單</summary><div class="mt-3">
<form method="post" action="/models/discover" class="space-y-3"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label class="block text-xs font-medium text-slate-600">協議<select name="disc_protocol" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"><option value="openai-chat">OpenAI Chat Completions</option><option value="openai-responses">OpenAI Responses</option></select></label>
<label class="block text-xs font-medium text-slate-600">Base URL<input name="disc_base_url" placeholder="https://api.example.com/v1" required class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">API Key 環境變數<input name="disc_key_env" placeholder="EXAMPLE_API_KEY" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<button class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm hover:bg-slate-50">查詢</button></form>
{{if .Discovered}}<form method="post" action="/models/bulk" class="mt-4 space-y-2"><input type="hidden" name="csrf" value="{{.CSRF}}"><input type="hidden" name="count" value="{{len .Discovered}}"><input type="hidden" name="protocol" value="{{.DiscProtocol}}"><input type="hidden" name="base_url" value="{{.DiscBaseURL}}"><input type="hidden" name="key_env" value="{{.DiscKeyEnv}}"><input type="hidden" name="api_key" value="{{.DiscKey}}">
<div class="text-xs font-medium text-slate-700">勾選並命名</div>
{{range $i, $id := .Discovered}}<div class="flex items-center gap-2"><input type="hidden" name="id_{{$i}}" value="{{$id}}"><input type="checkbox" name="use_{{$i}}" checked class="h-4 w-4"><code class="flex-1 truncate text-xs text-slate-600">{{$id}}</code><input name="name_{{$i}}" value="{{$id}}" class="w-40 rounded-lg border border-slate-300 bg-white px-2 py-1 text-xs"></div>{{end}}
<button class="mt-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm text-white">批次新增</button></form>{{end}}
</div></details>
</div></dialog>

<dialog id="m-mcp" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">MCP</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div>
<div class="space-y-5 px-6 py-5">
<div><h3 class="text-sm font-medium text-slate-900">已設定（{{len .Servers}}）</h3>
{{if .Servers}}<div class="mt-2 divide-y divide-slate-100 rounded-xl border border-slate-200">{{range .Servers}}<div class="flex flex-wrap items-center gap-2 px-3 py-2.5 text-sm"><span class="flex-1 truncate font-medium text-slate-800">{{.Name}}</span><span class="text-xs text-slate-400">{{.AuthType}}</span><span class="text-xs text-slate-400">{{if .Enabled}}啟用{{else}}停用{{end}}</span>
{{if eq .AuthType "oauth"}}<form method="post" action="/mcp/oauth/start" class="m-0"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="rounded-lg bg-indigo-50 px-2 py-1 text-xs text-indigo-700 hover:bg-indigo-100">OAuth 授權</button></form>{{end}}
<form method="post" action="/mcp/toggle" class="m-0"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="rounded-lg px-2 py-1 text-xs text-slate-500 hover:bg-slate-100">切換</button></form>
<form method="post" action="/mcp/delete" class="m-0"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="rounded-lg px-2 py-1 text-xs text-rose-600 hover:bg-rose-50">刪除</button></form></div>{{end}}</div>{{else}}<p class="mt-2 text-xs text-slate-500">尚未設定 MCP 伺服器。</p>{{end}}</div>

<form method="post" action="/mcp/add" class="space-y-3 border-t border-slate-100 pt-5"><input type="hidden" name="csrf" value="{{.CSRF}}">
<h3 class="text-sm font-medium text-slate-900">新增伺服器</h3>
<label class="block text-xs font-medium text-slate-600">名稱<input name="name" required class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">URL（Streamable HTTP endpoint）<input name="url" placeholder="https://mcp.example.com/mcp" required class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">驗證方式<select name="auth_type" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"><option value="none">無</option><option value="api_key">API Key</option><option value="oauth">OAuth（授權碼流程）</option></select></label>
<details class="rounded-xl border border-slate-200 bg-slate-50 p-3"><summary class="cursor-pointer text-sm font-medium text-slate-700">API Key / OAuth 進階設定</summary><div class="mt-3 space-y-3">
<p class="text-xs text-slate-500">OAuth 回呼網址：<code id="oauth_redirect" class="rounded bg-white px-1 py-0.5"></code> <button type="button" onclick="copyRedirect()" class="text-indigo-600 hover:underline">複製</button></p>
<label class="block text-xs font-medium text-slate-600">API Key 環境變數<input name="api_key_env" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">或直接貼 API Key<input name="api_key" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">Authorization URL<input name="oauth_auth_url" placeholder="https://provider.com/authorize" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">Token URL<input name="oauth_token_url" placeholder="https://provider.com/token" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<div class="grid gap-3 sm:grid-cols-2"><label class="block text-xs font-medium text-slate-600">Client ID<input name="oauth_client_id" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label><label class="block text-xs font-medium text-slate-600">Client secret<input name="oauth_client_secret" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label></div>
<label class="block text-xs font-medium text-slate-600">Scopes（逗號分隔）<input name="oauth_scopes" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="oauth_use_pkce" checked class="h-4 w-4 rounded border-slate-300"> 使用 PKCE</label>
<details class="rounded-lg border border-slate-200 bg-white p-2"><summary class="cursor-pointer text-xs font-medium text-slate-600">直接貼 token（不建議）</summary><div class="mt-2 space-y-2"><input name="oauth_token_env" placeholder="Token 環境變數" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"><input name="oauth_access_token" placeholder="Access token" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"><input name="oauth_refresh_token" placeholder="Refresh token" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></div></details>
</div></details>
<button class="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">新增伺服器</button></form>
</div></dialog>

<dialog id="m-vps" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">VPS 指令工具</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div>
<form method="post" action="/vps/save" class="space-y-4 px-6 py-5"><input type="hidden" name="csrf" value="{{.CSRF}}">
<p class="text-xs leading-relaxed text-slate-500">模型只會看到「一台可以運行指令的 VPS」，看不到連線資訊。</p>
<label class="flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="enabled" {{if .VPS.Enabled}}checked{{end}} class="h-4 w-4 rounded border-slate-300"> 啟用</label>
<div class="grid gap-3 sm:grid-cols-3"><label class="block text-xs font-medium text-slate-600">Host<input name="host" value="{{.VPS.Host}}" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label><label class="block text-xs font-medium text-slate-600">Port<input name="port" value="{{.VPS.Port}}" class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label><label class="block text-xs font-medium text-slate-600">User<input name="user" value="{{.VPS.User}}" class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label></div>
<label class="block text-xs font-medium text-slate-600">驗證方式<select name="auth_type" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"><option value="key" {{if eq .VPS.AuthType "key"}}selected{{end}}>SSH 私鑰</option><option value="password" {{if eq .VPS.AuthType "password"}}selected{{end}}>密碼</option></select></label>
<details class="rounded-xl border border-slate-200 bg-slate-50 p-3"><summary class="cursor-pointer text-sm font-medium text-slate-700">憑證（進階）</summary><div class="mt-3 space-y-3">
<label class="block text-xs font-medium text-slate-600">私鑰環境變數<input name="key_env" value="{{.VPS.KeyEnv}}" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">或直接貼私鑰（留空沿用）<textarea name="key" class="mt-1 min-h-20 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-xs" autocomplete="off"></textarea></label>
<label class="block text-xs font-medium text-slate-600">passphrase（留空沿用）<input name="passphrase" type="password" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">密碼環境變數<input name="password_env" value="{{.VPS.PasswordEnv}}" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">或直接貼密碼（留空沿用）<input name="password" type="password" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
</div></details>
<label class="block text-xs font-medium text-slate-600">喚醒 URL（選用）<input name="wake_url" value="{{.VPS.WakeURL}}" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<button class="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">儲存</button></form></dialog>

<dialog id="m-media" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">語音</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div>
<form method="post" action="/media/save" class="space-y-4 px-6 py-5"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label class="block text-xs font-medium text-slate-600">Groq API Key 環境變數<input name="groq_api_key_env" value="{{.Media.GroqAPIKeyEnv}}" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<details class="rounded-xl border border-slate-200 bg-slate-50 p-3"><summary class="cursor-pointer text-sm font-medium text-slate-700">進階</summary><div class="mt-3 space-y-3">
<label class="block text-xs font-medium text-slate-600">直接貼 Key（留空沿用）<input name="groq_api_key" type="password" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
<div class="grid grid-cols-2 gap-3"><label class="block text-xs font-medium text-slate-600">音訊上限（秒）<input name="max_audio_seconds" value="{{.Media.MaxAudioSeconds}}" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label><label class="block text-xs font-medium text-slate-600">文字上限（字元）<input name="max_tts_chars" value="{{.Media.MaxTTSChars}}" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label></div>
<label class="block text-xs font-medium text-slate-600">語音<input name="tts_voice" value="{{.Media.TTSVoice}}" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"></label>
</div></details>
<p class="text-xs text-slate-500">語音輸出用 edge-tts；VPS 需安裝 <code>python3 -m pip install edge-tts</code>。</p>
<button class="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">儲存</button></form></dialog>

<dialog id="m-settings" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">助理設定</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div>
<div class="space-y-5 px-6 py-5">
<form method="post" action="/appearance" class="space-y-3"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label class="block text-xs font-medium text-slate-600">助理名稱<input name="assistant_name" value="{{.AssistantName}}" required class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">無法回覆時的訊息<input name="unavailable_message" value="{{.UnavailableMessage}}" required class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"></label>
<details class="rounded-xl border border-slate-200 bg-slate-50 p-3"><summary class="cursor-pointer text-sm font-medium text-slate-700">除錯顯示</summary><div class="mt-3 space-y-2"><label class="flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="display_thinking" {{if .Display.Thinking}}checked{{end}} class="h-4 w-4 rounded border-slate-300"> 顯示思考過程</label><label class="flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="display_tool_calls" {{if .Display.ToolCalls}}checked{{end}} class="h-4 w-4 rounded border-slate-300"> 顯示工具調用</label><label class="flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="display_raw_output" {{if .Display.RawOutput}}checked{{end}} class="h-4 w-4 rounded border-slate-300"> 顯示模型原始輸出</label></div></details>
<button class="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">儲存助理設定</button></form>
<div class="border-t border-slate-100 pt-4"><form method="post" action="/commands"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm hover:bg-slate-50">重新註冊 Slash 指令</button></form></div>
<details class="rounded-xl border border-slate-200 bg-slate-50 p-3"><summary class="cursor-pointer text-sm font-medium text-slate-700">完整設定（JSON，進階）</summary>
<form method="post" action="/settings" class="mt-3 space-y-3"><input type="hidden" name="csrf" value="{{.CSRF}}"><textarea name="config" spellcheck="false" class="max-h-96 min-h-52 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-xs leading-relaxed">{{.JSON}}</textarea><button class="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-indigo-700">儲存並套用</button></form>
</details>
</div></dialog>

<dialog id="m-logs" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">日誌</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div><div class="px-6 py-5"><pre class="max-h-[65vh] overflow-auto rounded-xl bg-slate-900 p-4 text-xs leading-relaxed text-slate-100">{{range .Lines}}{{.}}{{"\n"}}{{end}}</pre></div></dialog>

<dialog id="m-errors" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">錯誤紀錄</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div><div class="px-6 py-5"><p class="mb-3 text-xs text-slate-500">只保存最近 50 條已去除金鑰的錯誤摘要。</p>
{{if .Entries}}<div class="divide-y divide-slate-100 rounded-xl border border-slate-200">{{range .Entries}}<div class="px-3 py-2 text-xs"><span class="mr-2 text-slate-400">{{.Time.Format "2006-01-02 15:04"}}</span><span class="text-slate-700">{{.Message}}</span></div>{{end}}</div>{{else}}<p class="text-xs text-slate-500">尚無錯誤紀錄。</p>{{end}}
</div></dialog>

{{if or .Message .Error}}<div id="toast" class="fixed bottom-6 right-6 z-50 max-w-sm rounded-xl border px-4 py-3 text-sm shadow-lg {{if .Error}}border-rose-200 bg-rose-50 text-rose-700{{else}}border-emerald-200 bg-emerald-50 text-emerald-700{{end}}">{{if .Error}}{{.Error}}{{else}}{{.Message}}{{end}}</div>{{end}}

<script>
function openModal(id) {
  var el = document.getElementById('m-' + id);
  if (el && typeof el.showModal === 'function') el.showModal();
}
document.addEventListener('click', function (e) {
  var t = e.target.closest('[data-modal]');
  if (t) { openModal(t.getAttribute('data-modal')); return; }
  var c = e.target.closest('[data-close]');
  if (c) { var d = c.closest('dialog'); if (d) d.close(); }
});
document.querySelectorAll('dialog').forEach(function (d) {
  d.addEventListener('click', function (e) { if (e.target === d) d.close(); });
});
function copyRedirect() {
  var u = location.origin + '/mcp/oauth/callback';
  if (navigator.clipboard) navigator.clipboard.writeText(u);
}
var redirect = document.getElementById('oauth_redirect');
if (redirect) redirect.textContent = location.origin + '/mcp/oauth/callback';
function applyPreset() {
  var p = document.getElementById('model_preset').value;
  var presets = {
    deepseek: {name:'DeepSeek V4.1 Flash', protocol:'openai-chat', base_url:'https://api.deepseek.com/v1', model_id:'deepseek-chat', key:'DEEPSEEK_API_KEY'},
    openai: {name:'OpenAI GPT', protocol:'openai-chat', base_url:'https://api.openai.com/v1', model_id:'gpt-4o-mini', key:'OPENAI_API_KEY'},
    anthropic: {name:'Anthropic Claude', protocol:'anthropic', base_url:'https://api.anthropic.com/v1', model_id:'claude-3-5-haiku-latest', key:'ANTHROPIC_API_KEY'},
    gemini: {name:'Google Gemini', protocol:'google', base_url:'https://generativelanguage.googleapis.com/v1beta', model_id:'gemini-2.0-flash', key:'GEMINI_API_KEY'},
    groq: {name:'Groq', protocol:'openai-chat', base_url:'https://api.groq.com/openai/v1', model_id:'llama-3.3-70b-versatile', key:'GROQ_API_KEY'}
  };
  var v = presets[p]; if (!v) return;
  var f = document.querySelector('form[action="/models/add"]');
  f.elements['name'].value = v.name;
  f.elements['protocol'].value = v.protocol;
  f.elements['base_url'].value = v.base_url;
  f.elements['model_id'].value = v.model_id;
  f.elements['api_key_env'].value = v.key;
}
{{if .Active}}document.addEventListener('DOMContentLoaded', function () { openModal('{{.Active}}'); });{{end}}
var toast = document.getElementById('toast');
if (toast) setTimeout(function () { toast.remove(); }, 8000);
</script>
</body></html>
{{end}}
`))
