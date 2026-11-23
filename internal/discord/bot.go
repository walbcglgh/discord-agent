package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/llm"
	"discord-agent/internal/memory"
	"discord-agent/internal/tools"
	"discord-agent/internal/util"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session        *discordgo.Session
	store          *config.Store
	mem            *memory.Store
	tools          *tools.Registry
	log            *log.Logger
	botToken       string
	started        time.Time
	botID          string
	rateMu         sync.Mutex
	lastMsg        map[string]time.Time
	modelMu        sync.RWMutex
	lastModel      string
	pendingMu      sync.Mutex
	pendingActions map[string]PendingAction
	uiMu           sync.Mutex
	uiPayloads     map[string]uiPayload
	queueMu        sync.Mutex
	channelQueues  map[string]*sync.Mutex
}

type ToolTrace struct {
	Name   string
	Args   string
	Result string
}

// Generation keeps the intermediate stages separate from the final reply.
type Generation struct {
	Reply     string
	Thinking  string
	RawOutput string
	Tools     []ToolTrace
	ModelName string
}

func New(token string, store *config.Store, mem *memory.Store, toolReg *tools.Registry, logger *log.Logger) (*Bot, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	s.StateEnabled = false
	s.Identify.Intents = discordgo.IntentGuildMessages | discordgo.IntentDirectMessages | discordgo.IntentMessageContent
	b := &Bot{
		session: s, store: store, mem: mem, tools: toolReg, log: logger,
		botToken: token, started: time.Now(), lastMsg: make(map[string]time.Time),
		pendingActions: make(map[string]PendingAction),
		uiPayloads:     make(map[string]uiPayload),
		channelQueues:  make(map[string]*sync.Mutex),
	}
	s.AddHandler(b.onReady)
	s.AddHandler(b.onMessageCreate)
	s.AddHandler(b.onInteraction)
	return b, nil
}

func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return err
	}
	return b.registerCommands()
}

// ReregisterCommands syncs slash commands after config changes from the panel.
func (b *Bot) ReregisterCommands() error { return b.registerCommands() }

func (b *Bot) Stop() error { return b.session.Close() }

func (b *Bot) Session() *discordgo.Session { return b.session }

func (b *Bot) onReady(_ *discordgo.Session, r *discordgo.Ready) {
	if r == nil || r.User == nil {
		return
	}
	b.botID = r.User.ID
	b.log.Printf("Discord 已連線：%s#%s", r.User.Username, r.User.Discriminator)
}

func (b *Bot) registerCommands() error {
	app, err := b.session.Application("@me")
	if err != nil {
		return fmt.Errorf("取得 Application ID 失敗: %w", err)
	}
	appID := app.ID
	if b.botID == "" {
		b.botID = appID
	}
	cmds := b.commandDefinitions()
	cfg := b.store.Get()
	guilds := cfg.Discord.AllowedGuildIDs
	if len(guilds) == 0 {
		_, err := b.session.ApplicationCommandBulkOverwrite(appID, "", cmds)
		return err
	}
	for _, g := range guilds {
		if _, err := b.session.ApplicationCommandBulkOverwrite(appID, g, cmds); err != nil {
			return fmt.Errorf("註冊 %s 的指令失敗: %w", g, err)
		}
	}
	return nil
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || m.Content == "" {
		return
	}
	cfg := b.store.Get()
	if !b.shouldReply(cfg, s, m) {
		return
	}
	turn := b.channelLock(m.ChannelID)
	turn.Lock()
	defer turn.Unlock()
	mentioned := strings.Contains(m.Content, "<@"+b.botID+">") ||
		strings.Contains(m.Content, "<@!"+b.botID+">")
	// @ 提及一律排隊，不因限速丟棄；一般頻道訊息才做限速。
	if !mentioned && !b.allowMessage(m.Author.ID+":"+m.ChannelID, cfg.Discord.RateLimitSeconds) {
		return
	}
	userText := b.cleanContent(m.Content)
	if userText == "" {
		b.replyNotice(s, m.ChannelID, "在的，請問有什麼可以幫你？", cfg)
		return
	}
	userText = fmt.Sprintf("[%s] %s", m.Author.Username, userText)
	scope := "channel:" + m.ChannelID
	if m.GuildID == "" {
		scope = "dm:" + m.Author.ID
	}
	stopTyping := make(chan struct{})
	if cfg.Discord.TypingIndicator {
		_ = s.ChannelTyping(m.ChannelID)
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopTyping:
					return
				case <-ticker.C:
					_ = s.ChannelTyping(m.ChannelID)
				}
			}
		}()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	ctx = withMessageRef(ctx, MessageRef{
		ChannelID: m.ChannelID, MessageID: m.ID, GuildID: m.GuildID,
		UserID: m.Author.ID, Username: m.Author.Username, Scope: scope,
		IsAdmin: contains(cfg.Discord.AdminUserIDs, m.Author.ID),
	})
	audio := b.audioText(ctx, m, cfg)
	if audio.TooLong {
		close(stopTyping)
		b.replyNotice(s, m.ChannelID, "超過 2 分鐘的音頻我收不到。", cfg)
		return
	}
	if audio.Text != "" {
		userText += "\n[語音轉文字] " + audio.Text
	}
	images := b.imageAttachments(m)
	outputs := &tools.Outputs{}
	ctx = tools.WithOutputs(ctx, outputs)
	pending := &PendingCollector{}
	ctx = withPending(ctx, pending)
	gen, err := b.generate(ctx, cfg, scope, userText, images, false)
	close(stopTyping)
	if err != nil {
		b.replyNotice(s, m.ChannelID, b.recordError(cfg, err), cfg)
		return
	}
	b.replyGenerated(s, m.ChannelID, gen, cfg)
	b.sendFiles(s, m.ChannelID, outputs.Files)
	b.sendPendingButtons(s, m.ChannelID, pending.Items)
	if cfg.Memory.Enabled {
		_ = b.mem.AddExchange(scope, userText, gen.Reply)
		_ = b.mem.IncrStat("messages")
		b.maybeSummarize(cfg, scope)
	}
}

func (b *Bot) shouldReply(cfg *config.Config, s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if m.GuildID == "" {
		return false // 私訊一律不處理
	}
	if len(cfg.Discord.AllowedGuildIDs) > 0 && !contains(cfg.Discord.AllowedGuildIDs, m.GuildID) {
		return false
	}
	mentioned := strings.Contains(m.Content, "<@"+b.botID+">") ||
		strings.Contains(m.Content, "<@!"+b.botID+">")
	// @here / @everyone are never a valid trigger by themselves.
	if !mentioned && (strings.Contains(m.Content, "@everyone") || strings.Contains(m.Content, "@here")) {
		return false
	}
	channelAllowed := contains(cfg.Discord.AllowedChannelIDs, m.ChannelID)
	switch cfg.Discord.ReplyMode {
	case "mention":
		return mentioned
	case "channel":
		return channelAllowed
	default:
		return channelAllowed || mentioned
	}
}

func (b *Bot) cleanContent(content string) string {
	if b.botID == "" {
		return strings.TrimSpace(content)
	}
	content = strings.ReplaceAll(content, "<@!"+b.botID+">", "")
	content = strings.ReplaceAll(content, "<@"+b.botID+">", "")
	return strings.TrimSpace(content)
}

// channelLock serializes replies within one channel: later mentions wait
// their turn instead of interleaving with an in-flight reply.
func (b *Bot) channelLock(channelID string) *sync.Mutex {
	b.queueMu.Lock()
	defer b.queueMu.Unlock()
	if m, ok := b.channelQueues[channelID]; ok {
		return m
	}
	m := &sync.Mutex{}
	b.channelQueues[channelID] = m
	return m
}

func (b *Bot) allowMessage(key string, seconds int) bool {
	b.rateMu.Lock()
	defer b.rateMu.Unlock()
	if seconds < 1 {
		seconds = 1
	}
	if last, ok := b.lastMsg[key]; ok && time.Since(last) < time.Duration(seconds)*time.Second {
		return false
	}
	b.lastMsg[key] = time.Now()
	if len(b.lastMsg) > 2000 {
		for k, t := range b.lastMsg {
			if time.Since(t) > time.Hour {
				delete(b.lastMsg, k)
			}
		}
	}
	return true
}

// ---------- 輸出裝飾 ----------

func assistantName(cfg *config.Config) string {
	if name := strings.TrimSpace(cfg.AssistantName); name != "" {
		return name
	}
	return "助理"
}

func withFooter(name, text string) string {
	model := strings.TrimSpace(name)
	if model == "" {
		model = "系統"
	}
	return strings.TrimRight(text, "\n") + "\n\n本輪訊息由 " + model + " 生成回覆"
}

func (b *Bot) setCurrentModel(name string) {
	b.modelMu.Lock()
	b.lastModel = name
	b.modelMu.Unlock()
}

func (b *Bot) currentModelName(cfg *config.Config) string {
	b.modelMu.RLock()
	name := b.lastModel
	b.modelMu.RUnlock()
	if strings.TrimSpace(name) == "" {
		return assistantName(cfg)
	}
	return name
}

func fence(label, content string) string {
	content = strings.ReplaceAll(content, "```", "'''")
	r := []rune(strings.TrimSpace(content))
	if len(r) > 1200 {
		content = string(r[:1200]) + "…（已截斷）"
	} else {
		content = string(r)
	}
	return "```" + label + "\n" + content + "\n```"
}

func toolText(traces []ToolTrace) string {
	var sb strings.Builder
	for i, t := range traces {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "工具：%s\n參數：%s\n結果：%s", clip(t.Name, 100), clip(t.Args, 600), clip(t.Result, 600))
	}
	return sb.String()
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func formatGeneration(cfg *config.Config, gen Generation) string {
	var parts []string
	if cfg.Display.Thinking && strings.TrimSpace(gen.Thinking) != "" {
		parts = append(parts, fence("思考過程", gen.Thinking))
	}
	if cfg.Display.ToolCalls && len(gen.Tools) > 0 {
		parts = append(parts, fence("工具調用", toolText(gen.Tools)))
	}
	if cfg.Display.RawOutput && strings.TrimSpace(gen.RawOutput) != "" {
		parts = append(parts, fence("模型原始輸出", gen.RawOutput))
	}
	parts = append(parts, gen.Reply)
	return strings.Join(parts, "\n\n")
}

func unavailableMessage(cfg *config.Config) string {
	msg := strings.TrimSpace(cfg.UnavailableMessage)
	if msg == "" {
		msg = "{assistant}暫時無法回覆"
	}
	return strings.ReplaceAll(msg, "{assistant}", assistantName(cfg))
}

func (b *Bot) secrets() []string {
	cfg := b.store.Get()
	list := []string{b.botToken, os.Getenv("PANEL_PASSWORD"), os.Getenv(cfg.Discord.TokenEnv), cfg.Discord.Token}
	for _, p := range cfg.LLM.Providers {
		list = append(list, p.APIKey, os.Getenv(p.APIKeyEnv))
	}
	for _, m := range cfg.LLM.Models {
		list = append(list, m.APIKey, os.Getenv(m.APIKeyEnv))
	}
	for _, s := range cfg.MCP.Servers {
		list = append(list, s.APIKey, os.Getenv(s.APIKeyEnv), s.OAuthAccessToken,
			os.Getenv(s.OAuthTokenEnv), s.OAuthRefreshToken, s.OAuthClientSecret)
	}
	list = append(list, cfg.VPS.Host, cfg.VPS.User, strconv.Itoa(cfg.VPS.Port),
		config.Secret(cfg.VPS.PasswordEnv, cfg.VPS.Password),
		config.Secret(cfg.VPS.KeyEnv, cfg.VPS.Key), cfg.VPS.Passphrase, cfg.VPS.WakeURL,
		config.Secret(cfg.Media.GroqAPIKeyEnv, cfg.Media.GroqAPIKey))
	list = append(list, cfg.Tools.WebSearch.APIKey, os.Getenv(cfg.Tools.WebSearch.APIKeyEnv))
	return list
}

// recordError stores only a sanitized copy; raw provider errors never reach
// Discord, the panel log, or the error review page.
func (b *Bot) recordError(cfg *config.Config, err error) string {
	safe := util.Redact(err.Error(), b.secrets())
	b.log.Printf("生成回覆失敗: %s", safe)
	_ = b.mem.AddError(safe)
	return unavailableMessage(cfg)
}

func (b *Bot) replyGenerated(s *discordgo.Session, channelID string, gen Generation, cfg *config.Config) {
	b.sendRaw(s, channelID, formatGeneration(cfg, gen), cfg, gen.ModelName)
}

func (b *Bot) replyNotice(s *discordgo.Session, channelID, text string, cfg *config.Config) {
	b.sendRaw(s, channelID, text, cfg, b.currentModelName(cfg))
}

func (b *Bot) sendRaw(s *discordgo.Session, channelID, text string, cfg *config.Config, modelName string) {
	limit := cfg.Discord.MaxReplyChars
	if limit < 500 || limit > 2000 {
		limit = 1900
	}
	footer := "\n\n本輪訊息由 " + strings.TrimSpace(modelName) + " 生成回覆"
	for _, chunk := range splitMessage(text, limit-len([]rune(footer))) {
		chunk += footer
		if _, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content:         chunk,
			AllowedMentions: allowedUserMentions(),
		}); err != nil {
			b.log.Printf("傳送訊息失敗: %s", util.Redact(err.Error(), b.secrets()))
		}
	}
}

func splitMessage(text string, limit int) []string {
	if strings.TrimSpace(text) == "" {
		return []string{"（沒有內容）"}
	}
	runes := []rune(text)
	var out []string
	for len(runes) > limit {
		cut := limit
		if i := strings.LastIndex(string(runes[:limit]), "\n"); i > limit/2 {
			cut = i + 1
		}
		out = append(out, strings.TrimSpace(string(runes[:cut])))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		out = append(out, strings.TrimSpace(string(runes)))
	}
	return out
}

// ---------- 兩階段生成 ----------

func (b *Bot) buildMessages(cfg *config.Config, scope, userText, stage string, images []llm.Image) []llm.Message {
	var sb strings.Builder
	sb.WriteString(cfg.Chat.SystemPrompt)
	fmt.Fprintf(&sb, "\n你是「%s」，服務對象是 Discord 伺服器成員。", assistantName(cfg))
	if cfg.Chat.ReplyLanguage != "" {
		sb.WriteString("\n請使用 " + cfg.Chat.ReplyLanguage + " 回覆。")
	}
	if cfg.Memory.Enabled {
		if summary, _ := b.mem.Summary(scope); summary != "" {
			sb.WriteString("\n\n[先前對話摘要]\n" + summary)
		}
		if facts, _ := b.mem.Facts(scope, 12); len(facts) > 0 {
			sb.WriteString("\n\n[已知資訊]")
			for _, f := range facts {
				fmt.Fprintf(&sb, "\n- %s: %s", f.Key, f.Value)
			}
		}
	}
	if stage == "plan" {
		sb.WriteString("\n\n第一階段：先思考並呼叫需要的工具。需要勘查上下文時，建議一次只查 10 則，最多 100 則，避免耗光 tokens。不要輸出要發給伺服器的最終回覆。")
	} else {
		sb.WriteString("\n\n第二階段：根據工具結果與當下情境決定最終回覆。只輸出要發給伺服器的內容，不提及思考過程或工具。")
	}
	msgs := []llm.Message{{Role: "system", Content: sb.String()}}
	if recent, err := b.mem.Recent(scope, cfg.Chat.ContextMessages); err == nil {
		msgs = append(msgs, recent...)
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: userText, Images: images})
	return msgs
}

func firstModel(cfg *config.Config) (config.ModelConfig, bool) {
	for _, m := range cfg.LLM.Models {
		if m.Enabled {
			return m, true
		}
	}
	return config.ModelConfig{}, false
}

// routeCandidates picks one enabled model at random, then all other enabled
// models sharing the same Name as fallbacks, also randomly ordered.
func routeCandidates(cfg *config.Config) ([]config.ModelConfig, error) {
	var enabled []config.ModelConfig
	for _, m := range cfg.LLM.Models {
		if !m.Enabled {
			continue
		}
		resolved, ok := cfg.ResolveModel(m)
		if ok && config.IsValidProtocol(resolved.Protocol) {
			enabled = append(enabled, m)
		}
	}
	if len(enabled) == 0 {
		return nil, fmt.Errorf("沒有已啟用的模型")
	}
	primary := enabled[rand.Intn(len(enabled))]
	out := []config.ModelConfig{primary}
	for _, m := range enabled {
		if m.ID != primary.ID && m.Name == primary.Name {
			out = append(out, m)
		}
	}
	rand.Shuffle(len(out)-1, func(i, j int) { out[i+1], out[j+1] = out[j+1], out[i+1] })
	return out, nil
}

func (b *Bot) generate(ctx context.Context, cfg *config.Config, scope, userText string, images []llm.Image, oneShot bool) (Generation, error) {
	candidates, err := routeCandidates(cfg)
	if err != nil {
		return Generation{}, err
	}
	var lastErr error
	lastName := ""
	for _, mc := range candidates {
		gen, gerr := b.generateWithModel(ctx, cfg, mc, scope, userText, images)
		if gerr == nil {
			gen.ModelName = mc.Name
			b.setCurrentModel(mc.Name)
			return gen, nil
		}
		lastName = mc.Name
		lastErr = fmt.Errorf("提供商 %s / 模型 %s (%s): %w",
			cfg.ProviderNameOf(mc), mc.Name, mc.ModelID, gerr)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("模型暫時無法回覆")
	}
	return Generation{ModelName: lastName}, lastErr
}

func (b *Bot) generateWithModel(ctx context.Context, cfg *config.Config, mc config.ModelConfig, scope, userText string, images []llm.Image) (Generation, error) {
	resolved, ok := cfg.ResolveModel(mc)
	if !ok {
		return Generation{}, fmt.Errorf("模型 %q 找不到提供商設定", mc.Name)
	}
	provider, err := llm.New(resolved)
	if err != nil {
		return Generation{}, err
	}
	maxTokens := mc.MaxTokens
	if maxTokens < 1 {
		maxTokens = cfg.Chat.MaxOutputTokens
	}
	temp := mc.Temperature
	if temp <= 0 {
		temp = cfg.Chat.Temperature
	}
	gen := Generation{}
	var defs []llm.ToolDef
	if cfg.Chat.EnableTools && b.tools != nil {
		defs = b.tools.Definitions()
	}

	msgs := b.buildMessages(cfg, scope, userText, "plan", images)
	var toolResults strings.Builder
	for round := 0; round <= cfg.Chat.MaxToolRounds; round++ {
		resp, err := provider.Chat(ctx, llm.Request{
			Model: mc.ModelID, Messages: msgs, Tools: defs,
			MaxTokens: maxTokens, Temperature: temp,
		})
		if err != nil {
			return gen, err
		}
		if strings.TrimSpace(resp.Content) != "" {
			gen.Thinking += strings.TrimSpace(resp.Content) + "\n"
		}
		if len(resp.ToolCalls) == 0 {
			break
		}
		msgs = append(msgs, llm.Message{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls})
		for i, tc := range resp.ToolCalls {
			id := tc.ID
			if id == "" {
				id = fmt.Sprintf("call_%d_%d", round, i)
			}
			var toolImages []llm.Image
			toolCtx := tools.WithImages(tools.WithScope(ctx, scope), &toolImages)
			result, terr := b.tools.Execute(toolCtx, tc.Name, json.RawMessage(tc.Arguments))
			if terr != nil {
				result = "工具執行失敗：" + terr.Error()
			}
			result = clip(result, 4000)
			gen.Tools = append(gen.Tools, ToolTrace{Name: tc.Name, Args: clip(tc.Arguments, 600), Result: result})
			fmt.Fprintf(&toolResults, "\n工具 %s 結果：%s", tc.Name, result)
			msgs = append(msgs, llm.Message{Role: "tool", ToolCallID: id, Name: tc.Name, Content: result})
			if len(toolImages) > 0 {
				msgs = append(msgs, llm.Message{
					Role: "user", Content: "以上工具提供的圖片：", Images: toolImages,
				})
			}
		}
	}

	finalMsgs := b.buildMessages(cfg, scope, userText, "final", images)
	if toolResults.Len() > 0 {
		finalMsgs = append(finalMsgs, llm.Message{
			Role: "user", Content: "可用資訊：" + toolResults.String() + "\n請依上述資訊決定最終回覆。",
		})
	}
	resp, err := provider.Chat(ctx, llm.Request{
		Model: mc.ModelID, Messages: finalMsgs,
		MaxTokens: maxTokens, Temperature: temp,
	})
	if err != nil {
		return gen, err
	}
	if strings.TrimSpace(resp.Content) == "" {
		return gen, fmt.Errorf("模型回傳空白內容")
	}
	gen.RawOutput = strings.TrimSpace(resp.Content)
	gen.Reply = gen.RawOutput
	return gen, nil
}

func (b *Bot) maybeSummarize(cfg *config.Config, scope string) {
	if !cfg.Memory.Enabled || cfg.Chat.SummaryEvery < 2 {
		return
	}
	count, err := b.mem.MessageCount(scope)
	if err != nil || count < cfg.Chat.SummaryEvery || count%cfg.Chat.SummaryEvery != 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		recent, err := b.mem.Recent(scope, cfg.Chat.SummaryEvery)
		if err != nil {
			return
		}
		var sb strings.Builder
		for _, m := range recent {
			fmt.Fprintf(&sb, "%s: %s\n", m.Role, m.Content)
		}
		mc, ok := firstModel(cfg)
		if !ok {
			return
		}
		resolved, ok := cfg.ResolveModel(mc)
		if !ok {
			return
		}
		provider, err := llm.New(resolved)
		if err != nil {
			return
		}
		resp, err := provider.Chat(ctx, llm.Request{
			Model: mc.ModelID,
			Messages: []llm.Message{
				{Role: "system", Content: "請將以下對話濃縮成 200 字以內的繁體中文摘要，保留重要事實與使用者偏好。"},
				{Role: "user", Content: sb.String()},
			},
			MaxTokens: 300, Temperature: 0.2,
		})
		if err == nil && strings.TrimSpace(resp.Content) != "" {
			_ = b.mem.SetSummary(scope, resp.Content)
		}
	}()
}

func (b *Bot) Status() map[string]any {
	cfg := b.store.Get()
	mc, ok := firstModel(cfg)
	stats, _ := b.mem.Stats()
	modelName := b.currentModelName(cfg)
	modelID := "未設定"
	providerName := "未設定"
	if ok {
		modelID = mc.ModelID
		providerName = cfg.ProviderNameOf(mc)
	}
	return map[string]any{
		"uptime":     time.Since(b.started).Round(time.Second).String(),
		"bot_id":     b.botID,
		"provider":   providerName,
		"model":      modelID,
		"mapping":    modelName,
		"reply_mode": cfg.Discord.ReplyMode,
		"assistant":  assistantName(cfg),
		"messages":   stats["messages"],
		"started_at": b.started.Format(time.RFC3339),
	}
}

func allowedUserMentions() *discordgo.MessageAllowedMentions {
	return &discordgo.MessageAllowedMentions{
		Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
	}
}

func allowedSpecificUser(id string) *discordgo.MessageAllowedMentions {
	return &discordgo.MessageAllowedMentions{
		Parse: []discordgo.AllowedMentionType{}, Users: []string{id},
	}
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
