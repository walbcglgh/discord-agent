package panel

import "html/template"

var pages = template.Must(template.New("pages").Parse(`
{{define "head"}}<!doctype html>
<html lang="zh-Hant"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Selana Agent · 管理面板</title>
<link rel="stylesheet" href="/assets/tailwind.css">
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

<dialog id="m-models" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">模型</h2><div class="flex items-center gap-2"><button type="button" data-modal="provider-add" class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs hover:bg-slate-50">新增提供商</button><button type="button" data-model-provider="" class="rounded-lg bg-indigo-600 px-3 py-1.5 text-xs text-white hover:bg-indigo-700">新增模型</button><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div></div>
<div class="space-y-4 px-6 py-5">
{{if .Providers}}
{{range $p := .Providers}}
<div class="rounded-xl border border-slate-200 bg-white">
<div class="flex flex-wrap items-center gap-2 border-b border-slate-100 px-4 py-3"><div class="min-w-0 flex-1"><div class="truncate text-sm font-semibold text-slate-900">{{$p.Name}}</div><div class="truncate text-xs text-slate-400">{{$p.ID}} · {{$p.Protocol}} · {{index $.ProviderCounts $p.ID}} 個模型</div></div>
<button type="button" data-model-provider="{{$p.ID}}" class="rounded-lg bg-indigo-50 px-2.5 py-1.5 text-xs font-medium text-indigo-700 hover:bg-indigo-100">新增模型</button>
<details class="relative"><summary class="cursor-pointer rounded-lg px-2 py-1.5 text-xs text-slate-500 hover:bg-slate-100">編輯</summary><form method="post" action="/providers/update" class="absolute right-0 top-8 z-20 w-72 space-y-2 rounded-xl border border-slate-200 bg-white p-3 shadow-lg"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{$p.ID}}">
<label class="block text-xs font-medium text-slate-600">顯示名稱<input name="name" value="{{$p.Name}}" required class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">Base URL<input name="base_url" value="{{$p.BaseURL}}" required class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm"></label>
<label class="block text-xs font-medium text-slate-600">協議<select name="protocol" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm"><option value="openai-chat" {{if eq $p.Protocol "openai-chat"}}selected{{end}}>OpenAI Chat Completions</option><option value="openai-responses" {{if eq $p.Protocol "openai-responses"}}selected{{end}}>OpenAI Responses</option><option value="anthropic" {{if eq $p.Protocol "anthropic"}}selected{{end}}>Anthropic Messages</option><option value="google" {{if eq $p.Protocol "google"}}selected{{end}}>Google API</option></select></label>
<label class="block text-xs font-medium text-slate-600">API 金鑰（留空沿用）<input name="api_key" type="password" autocomplete="off" class="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm"></label>
<button class="rounded-lg bg-indigo-600 px-3 py-1.5 text-xs text-white">儲存</button></form></details>
<form method="post" action="/providers/delete" class="m-0" onsubmit="return confirm('刪除這個提供商與旗下所有模型？')"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{$p.ID}}"><button class="rounded-lg px-2 py-1.5 text-xs text-rose-600 hover:bg-rose-50">刪除</button></form>
</div>
<div class="divide-y divide-slate-100 px-4">
{{range $.Models}}{{if eq .ProviderID $p.ID}}<div class="flex items-center gap-3 py-2.5 text-sm"><span class="flex-1 truncate font-medium text-slate-800">{{.Name}}</span><code class="hidden flex-1 truncate text-xs text-slate-500 sm:block">{{.ModelID}}</code><span class="text-xs text-slate-400">{{if .Enabled}}啟用{{else}}停用{{end}}</span><form method="post" action="/models/delete" class="m-0"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="rounded-lg px-2 py-1 text-xs text-rose-600 hover:bg-rose-50">刪除</button></form></div>{{end}}{{end}}
{{if eq (index $.ProviderCounts $p.ID) 0}}<div class="py-3 text-xs text-slate-500">尚未新增模型。</div>{{end}}
</div></div>
{{end}}
{{else}}<p class="text-xs text-slate-500">尚未設定提供商。先按「新增提供商」，再選 providers 新增模型。</p>{{end}}
</div></dialog>

<dialog id="m-provider-add" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">新增提供商</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div>
<form method="post" action="/providers/add" class="space-y-4 px-6 py-5"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label class="block text-sm font-medium text-slate-700">提供商 ID <span class="text-rose-500">*</span><input name="id" placeholder="例如 openai, google, anthropic" pattern="[a-z0-9_-]+" required class="mt-1.5 w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm"><span class="mt-1 block text-xs font-normal text-slate-500">小寫字母、數字、連字號、底線；建立後不可更改。</span></label>
<label class="block text-sm font-medium text-slate-700">顯示名稱 <span class="text-rose-500">*</span><input name="name" placeholder="例如 OpenAI, Google Gemini" required class="mt-1.5 w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm"></label>
<label class="block text-sm font-medium text-slate-700">預設 Base URL <span class="text-rose-500">*</span><input name="base_url" placeholder="https://api.example.com" required class="mt-1.5 w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm"></label>
<label class="block text-sm font-medium text-slate-700">API 金鑰<input name="api_key" type="password" autocomplete="off" placeholder="選填，直接貼在後台" class="mt-1.5 w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm"></label>
<label class="block text-sm font-medium text-slate-700">協議 <span class="text-rose-500">*</span><select name="protocol" class="mt-1.5 w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm"><option value="openai-chat">OpenAI 相容（Chat Completions）</option><option value="openai-responses">OpenAI Responses</option><option value="anthropic">Anthropic Messages</option><option value="google">Google API</option></select></label>
<div class="flex justify-end gap-2 border-t border-slate-100 pt-4"><button type="button" data-close class="rounded-lg border border-slate-300 bg-white px-4 py-2.5 text-sm">取消</button><button class="rounded-lg bg-amber-500 px-4 py-2.5 text-sm font-medium text-white hover:bg-amber-600">儲存</button></div>
</form></dialog>

<dialog id="m-model-add" class="ui-dialog"><div class="flex items-center justify-between border-b border-slate-100 px-6 py-4"><h2 class="text-lg font-semibold text-slate-900">新增模型</h2><button type="button" data-close class="rounded-lg px-2 py-1 text-slate-400 hover:bg-slate-100">✕</button></div>
<form method="post" action="/models/add" class="space-y-4 px-6 py-5"><input type="hidden" name="csrf" value="{{.CSRF}}">
<label class="block text-sm font-medium text-slate-700">提供商<select id="model_provider" name="provider_id" class="mt-1.5 w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm">{{range .Providers}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select></label>
<label class="relative block text-sm font-medium text-slate-700">模型 ID <span class="text-rose-500">*</span>
<input id="model_id_input" name="model_id" placeholder="例如 gpt-4o, gemini-2.0-flash" autocomplete="off" required class="mt-1.5 w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm">
<div id="model_suggestions" class="absolute left-0 right-0 top-full z-20 mt-1 hidden max-h-60 overflow-auto rounded-xl border border-slate-200 bg-white shadow-lg"></div>
<span class="mt-1 block text-xs font-normal text-slate-500">點一下欄位會自動抓這個提供商的模型清單；抓不到時可直接手動輸入。</span></label>
<label class="block text-sm font-medium text-slate-700">模型映射 <span class="text-rose-500">*</span><input name="name" placeholder="例如 GPT-4o, Gemini 2.0 Flash" required class="mt-1.5 w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm"><span class="mt-1 block text-xs font-normal text-slate-500">只會出現在「本輪由 … 生成回覆」。</span></label>
<label class="block text-sm font-medium text-slate-700">模型類型<select name="model_type" class="mt-1.5 w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm"><option>聊天</option></select></label>
<div class="flex justify-end gap-2 border-t border-slate-100 pt-4"><button type="button" data-close class="rounded-lg border border-slate-300 bg-white px-4 py-2.5 text-sm">取消</button><button class="rounded-lg bg-amber-500 px-4 py-2.5 text-sm font-medium text-white hover:bg-amber-600">確定</button></div>
</form></dialog>

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
function esc(s) {
  return String(s).replace(/[&<>"']/g, function (c) {
    return {'&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;'}[c];
  });
}
var modelCache = {};
function openModelDialog(providerID) {
  var sel = document.getElementById('model_provider');
  var input = document.getElementById('model_id_input');
  var box = document.getElementById('model_suggestions');
  if (sel && providerID) sel.value = providerID;
  if (input) input.value = '';
  if (box) { box.innerHTML = ''; box.classList.add('hidden'); }
  openModal('model-add');
  if (input) setTimeout(function () { input.focus(); }, 50);
}
function renderSuggestions(filter) {
  var sel = document.getElementById('model_provider');
  var box = document.getElementById('model_suggestions');
  if (!sel || !box) return;
  var list = modelCache[sel.value] || [];
  var q = String(filter || '').toLowerCase();
  var items = list.filter(function (x) { return x.toLowerCase().indexOf(q) >= 0; }).slice(0, 100);
  if (items.length === 0) { box.innerHTML = ''; box.classList.add('hidden'); return; }
  box.innerHTML = items.map(function (x) {
    return '<button type="button" data-pick-model="' + esc(x) + '" class="block w-full px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-50">' + esc(x) + '</button>';
  }).join('');
  box.classList.remove('hidden');
}
async function loadModels() {
  var sel = document.getElementById('model_provider');
  if (!sel || !sel.value) return;
  var id = sel.value;
  try {
    if (!modelCache[id]) {
      var res = await fetch('/models/list?provider_id=' + encodeURIComponent(id), {credentials: 'same-origin'});
      if (!res.ok) throw new Error('fetch failed');
      var data = await res.json();
      modelCache[id] = Array.isArray(data.models) ? data.models : [];
    }
    renderSuggestions(document.getElementById('model_id_input').value);
  } catch (e) {
    var box = document.getElementById('model_suggestions');
    if (box) { box.innerHTML = ''; box.classList.add('hidden'); }
  }
}
var modelInput = document.getElementById('model_id_input');
if (modelInput) {
  modelInput.addEventListener('focus', loadModels);
  modelInput.addEventListener('input', function () { renderSuggestions(modelInput.value); });
}
var providerSelect = document.getElementById('model_provider');
if (providerSelect) providerSelect.addEventListener('change', function () {
  modelCache = {};
  var box = document.getElementById('model_suggestions');
  if (box) { box.innerHTML = ''; box.classList.add('hidden'); }
});
document.addEventListener('click', function (e) {
  var pick = e.target.closest('[data-pick-model]');
  if (pick) {
    document.getElementById('model_id_input').value = pick.getAttribute('data-pick-model');
    document.getElementById('model_suggestions').classList.add('hidden');
    return;
  }
  var mp = e.target.closest('[data-model-provider]');
  if (mp) { openModelDialog(mp.getAttribute('data-model-provider')); return; }
  var t = e.target.closest('[data-modal]');
  if (t) { openModal(t.getAttribute('data-modal')); return; }
  var c = e.target.closest('[data-close]');
  if (c) { var d = c.closest('dialog'); if (d) d.close(); }
  if (!e.target.closest('#model_id_input') && !e.target.closest('#model_suggestions')) {
    var box = document.getElementById('model_suggestions');
    if (box) box.classList.add('hidden');
  }
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
{{if .Active}}document.addEventListener('DOMContentLoaded', function () { openModal('{{.Active}}'); });{{end}}
var toast = document.getElementById('toast');
if (toast) setTimeout(function () { toast.remove(); }, 8000);
</script>
</body></html>
{{end}}
`))
