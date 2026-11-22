package discord

import (
	"context"
	"fmt"
	"strings"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/tools"
	"discord-agent/internal/util"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) commandDefinitions() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name: "ask", Description: "向 AI 提問（帶著目前頻道的上下文）",
			Options: []*discordgo.ApplicationCommandOption{{
				Type: discordgo.ApplicationCommandOptionString, Name: "prompt",
				Description: "想問的問題", Required: true,
			}},
		},
		{Name: "clear", Description: "清除目前頻道的對話記憶（管理員）"},
		{Name: "status", Description: "查看 Bot 與模型狀態（管理員）"},
		{Name: "model", Description: "顯示目前使用的模型（管理員）"},
		{
			Name: "remember", Description: "記住一項資訊（管理員）",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "key", Description: "鍵名", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "value", Description: "內容", Required: true},
			},
		},
		{Name: "help", Description: "顯示使用說明"},
	}
}

func (b *Bot) onInteraction(s *discordgo.Session, ev *discordgo.InteractionCreate) {
	if ev.Type == discordgo.InteractionMessageComponent {
		b.onComponent(s, ev)
		return
	}
	if ev.Type != discordgo.InteractionApplicationCommand || ev.Interaction.GuildID == "" {
		return
	}
	i := ev.Interaction
	data, ok := i.Data.(discordgo.ApplicationCommandInteractionData)
	if !ok {
		return
	}
	userID := ""
	if i.User != nil {
		userID = i.User.ID
	} else if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}
	cfg := b.store.Get()
	switch data.Name {
	case "help":
		b.ephemeral(s, i, "直接 @我 或在白名單頻道發言即可對話。\n`/ask` 提問 · `/clear` 清除記憶 · `/status` 狀態 · `/model` 換模型 · `/remember` 記住資訊（後四項僅管理員）。")
	case "ask":
		b.handleAsk(s, i, data, userID, cfg)
	case "clear", "status", "model", "remember":
		if !contains(cfg.Discord.AdminUserIDs, userID) {
			b.ephemeral(s, i, "權限不足：此指令僅限管理員使用。")
			return
		}
		b.handleAdmin(s, i, data, cfg)
	default:
		b.ephemeral(s, i, "未知指令。")
	}
}

func (b *Bot) handleAsk(s *discordgo.Session, i *discordgo.Interaction, data discordgo.ApplicationCommandInteractionData, userID string, cfg *config.Config) {
	if i.GuildID != "" && len(cfg.Discord.AllowedGuildIDs) > 0 &&
		!contains(cfg.Discord.AllowedGuildIDs, i.GuildID) {
		b.ephemeral(s, i, "此伺服器未被授權使用。")
		return
	}
	opt := data.GetOption("prompt")
	prompt := ""
	if opt != nil {
		prompt = opt.StringValue()
	}
	if strings.TrimSpace(prompt) == "" {
		b.ephemeral(s, i, "請提供問題內容。")
		return
	}
	if err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		b.log.Printf("延遲回覆互動失敗: %v", err)
		return
	}
	scope := "channel:" + i.ChannelID
	if i.GuildID == "" {
		scope = "dm:" + userID
	}
	userText := fmt.Sprintf("[%s] %s", "slash-user", prompt)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	ctx = withMessageRef(ctx, MessageRef{
		ChannelID: i.ChannelID, GuildID: i.GuildID, UserID: userID,
		Username: "slash-user", Scope: scope, IsAdmin: contains(cfg.Discord.AdminUserIDs, userID),
	})
	outputs := &tools.Outputs{}
	ctx = tools.WithOutputs(ctx, outputs)
	pending := &PendingCollector{}
	ctx = withPending(ctx, pending)
	gen, err := b.generate(ctx, cfg, scope, userText, nil, true)
	content := ""
	if err != nil {
		content = b.recordError(cfg, err)
	} else {
		content = formatGeneration(cfg, gen)
		if cfg.Memory.Enabled {
			_ = b.mem.AddExchange(scope, userText, gen.Reply)
			_ = b.mem.IncrStat("messages")
		}
	}
	footerName := gen.ModelName
	if footerName == "" {
		footerName = b.currentModelName(cfg)
	}
	content = withFooter(footerName, content)
	runes := []rune(content)
	if len(runes) > 1900 {
		content = string(runes[:1900]) + "…（已截斷）"
	}
	if _, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &content}); err != nil {
		b.log.Printf("編輯互動回覆失敗: %s", util.Redact(err.Error(), b.secrets()))
	}
	b.sendFiles(s, i.ChannelID, outputs.Files)
	b.sendPendingButtons(s, i.ChannelID, pending.Items)
}

func (b *Bot) handleAdmin(s *discordgo.Session, i *discordgo.Interaction, data discordgo.ApplicationCommandInteractionData, cfg *config.Config) {
	scope := "channel:" + i.ChannelID
	switch data.Name {
	case "clear":
		if err := b.mem.Clear(scope); err != nil {
			b.ephemeral(s, i, "清除失敗: "+util.Redact(err.Error(), b.secrets()))
			return
		}
		b.ephemeral(s, i, "已清除此頻道的短期對話記憶（長期記憶保留）。")
	case "status":
		st := b.Status()
		b.ephemeral(s, i, fmt.Sprintf(
			"上線時間：%v\n供應商：%v\n模型：%v\n回覆模式：%v\n已處理訊息：%v",
			st["uptime"], st["provider"], st["model"], st["reply_mode"], st["messages"]))
	case "model":
		st := b.Status()
		names := make([]string, 0)
		seen := map[string]bool{}
		for _, m := range cfg.LLM.Models {
			if m.Enabled && !seen[m.Name] {
				seen[m.Name] = true
				names = append(names, m.Name)
			}
		}
		b.ephemeral(s, i, fmt.Sprintf("目前模型：%v\n上游 ID：%v\n可用模型名稱：%v",
			st["provider"], st["model"], strings.Join(names, "、")))
	case "remember":
		key, val := data.GetOption("key"), data.GetOption("value")
		if key == nil || val == nil || key.StringValue() == "" || val.StringValue() == "" {
			b.ephemeral(s, i, "請提供 key 與 value。")
			return
		}
		if err := b.mem.Remember(scope, key.StringValue(), val.StringValue()); err != nil {
			b.ephemeral(s, i, "寫入失敗: "+util.Redact(err.Error(), b.secrets()))
			return
		}
		b.ephemeral(s, i, "已記住：**"+key.StringValue()+"**")
	}
}

func (b *Bot) ephemeral(s *discordgo.Session, i *discordgo.Interaction, text string) {
	cfg := b.store.Get()
	content := withFooter(b.currentModelName(cfg), text)
	if len([]rune(content)) > 1900 {
		content = string([]rune(content)[:1900])
	}
	if err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		b.log.Printf("回覆互動失敗: %s", util.Redact(err.Error(), b.secrets()))
	}
}
