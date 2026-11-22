package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/llm"
	"discord-agent/internal/tools"

	"github.com/bwmarrin/discordgo"
)

type uiPayload struct {
	RequesterID string
	Username    string
	Scope       string
	ChannelID   string
	GuildID     string
	IsAdmin     bool
	Labels      map[string]string
	Kind        string
	Expires     time.Time
}

func RegisterUITools(reg *tools.Registry, b *Bot) {
	reg.AddTool(llm.ToolDef{
		Name: "send_buttons",
		Description: "送出 1~5 個按鈕給觸發你的人，只有他能按；使用者的選擇會回饋給你再回覆。" +
			"適合讓使用者確認或二選一。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text":   map[string]any{"type": "string", "description": "按鈕上方的說明文字"},
				"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "按鈕文字，最多 5 個"},
			},
			"required": []string{"text", "labels"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		ref, err := requireRequester(ctx)
		if err != nil {
			return "", err
		}
		var in struct {
			Text   string   `json:"text"`
			Labels []string `json:"labels"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return "", err
		}
		if len(in.Labels) == 0 || len(in.Labels) > 5 {
			return "", fmt.Errorf("labels 必須是 1~5 個")
		}
		token := config.NewID("u_")
		payload := uiPayload{
			RequesterID: ref.UserID, Username: ref.Username, Scope: ref.Scope,
			ChannelID: ref.ChannelID, GuildID: ref.GuildID, IsAdmin: ref.IsAdmin,
			Labels: map[string]string{}, Kind: "按鈕", Expires: time.Now().Add(15 * time.Minute),
		}
		buttons := make([]discordgo.MessageComponent, 0, len(in.Labels))
		for i, label := range in.Labels {
			key := strconv.Itoa(i)
			payload.Labels[key] = label
			buttons = append(buttons, discordgo.Button{
				Label: label, Style: discordgo.PrimaryButton,
				CustomID: "ui_btn_" + token + "_" + key,
			})
		}
		b.uiMu.Lock()
		b.uiPayloads[token] = payload
		b.uiMu.Unlock()
		_, err = b.session.ChannelMessageSendComplex(ref.ChannelID, &discordgo.MessageSend{
			Content:         in.Text,
			Components:      []discordgo.MessageComponent{discordgo.ActionsRow{Components: buttons}},
			AllowedMentions: allowedSpecificUser(ref.UserID),
		})
		if err != nil {
			return "", err
		}
		return "已送出按鈕，等待使用者的選擇。", nil
	})

	reg.AddTool(llm.ToolDef{
		Name: "send_select_menu",
		Description: "送出一個下拉選單給觸發你的人，只有他能選；使用者的選擇會回饋給你再回覆。" +
			"適合三個以上的選項。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text":    map[string]any{"type": "string"},
				"options": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "選項，最多 25 個"},
			},
			"required": []string{"text", "options"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		ref, err := requireRequester(ctx)
		if err != nil {
			return "", err
		}
		var in struct {
			Text    string   `json:"text"`
			Options []string `json:"options"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return "", err
		}
		if len(in.Options) < 2 || len(in.Options) > 25 {
			return "", fmt.Errorf("options 必須是 2~25 個")
		}
		token := config.NewID("u_")
		payload := uiPayload{
			RequesterID: ref.UserID, Username: ref.Username, Scope: ref.Scope,
			ChannelID: ref.ChannelID, GuildID: ref.GuildID, IsAdmin: ref.IsAdmin,
			Labels: map[string]string{}, Kind: "選單", Expires: time.Now().Add(15 * time.Minute),
		}
		options := make([]discordgo.SelectMenuOption, 0, len(in.Options))
		for i, label := range in.Options {
			key := strconv.Itoa(i)
			payload.Labels[key] = label
			options = append(options, discordgo.SelectMenuOption{Label: label, Value: key})
		}
		b.uiMu.Lock()
		b.uiPayloads[token] = payload
		b.uiMu.Unlock()
		_, err = b.session.ChannelMessageSendComplex(ref.ChannelID, &discordgo.MessageSend{
			Content: in.Text,
			Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "ui_sel_" + token, Placeholder: "請選擇", Options: options},
			}}},
			AllowedMentions: allowedSpecificUser(ref.UserID),
		})
		if err != nil {
			return "", err
		}
		return "已送出選單，等待使用者的選擇。", nil
	})

	reg.AddTool(llm.ToolDef{
		Name:        "send_embed",
		Description: "送出 Discord Embed 排版訊息。由你決定標題、內文與欄位怎麼排。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":       map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				"color":       map[string]any{"type": "integer", "description": "10 進位 RGB，例如 5814783"},
				"fields": map[string]any{"type": "array", "items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":  map[string]any{"type": "string"},
						"value": map[string]any{"type": "string"},
					},
				}},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		ref, err := requireRequester(ctx)
		if err != nil {
			return "", err
		}
		var in struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Color       int    `json:"color"`
			Fields      []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"fields"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return "", err
		}
		embed := &discordgo.MessageEmbed{Title: in.Title, Description: in.Description, Color: in.Color}
		for _, f := range in.Fields {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: f.Name, Value: f.Value})
		}
		_, err = b.session.ChannelMessageSendComplex(ref.ChannelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{embed}, AllowedMentions: allowedSpecificUser(ref.UserID),
		})
		if err != nil {
			return "", err
		}
		return "已送出 Embed。", nil
	})
}

func requireRequester(ctx context.Context) (MessageRef, error) {
	ref, ok := messageRefFrom(ctx)
	if !ok || ref.UserID == "" {
		return ref, fmt.Errorf("無法識別訊息發起者")
	}
	return ref, nil
}

func (b *Bot) handleUIComponent(s *discordgo.Session, ev *discordgo.InteractionCreate, data discordgo.MessageComponentInteractionData) {
	i := ev.Interaction
	userID := ""
	if i.User != nil {
		userID = i.User.ID
	} else if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}
	token, key := "", ""
	switch {
	case strings.HasPrefix(data.CustomID, "ui_btn_"):
		parts := strings.Split(strings.TrimPrefix(data.CustomID, "ui_btn_"), "_")
		if len(parts) < 2 {
			return
		}
		token, key = parts[0], parts[1]
	case strings.HasPrefix(data.CustomID, "ui_sel_"):
		token = strings.TrimPrefix(data.CustomID, "ui_sel_")
		if len(data.Values) > 0 {
			key = data.Values[0]
		}
	default:
		return
	}
	b.uiMu.Lock()
	payload, ok := b.uiPayloads[token]
	if ok {
		delete(b.uiPayloads, token)
	}
	b.uiMu.Unlock()
	if !ok || time.Now().After(payload.Expires) {
		_ = s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "這項選擇已過期。", Flags: discordgo.MessageFlagsEphemeral},
		})
		return
	}
	if userID != payload.RequesterID {
		_ = s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "只有被回覆的使用者可以操作。", Flags: discordgo.MessageFlagsEphemeral},
		})
		return
	}
	choice := payload.Labels[key]
	if choice == "" {
		choice = key
	}
	shown := fmt.Sprintf("你選擇了%s：%s", payload.Kind, choice)
	_ = s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{Content: shown, Components: []discordgo.MessageComponent{}},
	})

	cfg := b.store.Get()
	userText := fmt.Sprintf("[%s] 選擇了%s：%s", payload.Username, payload.Kind, choice)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	messageID := ""
	if i.Message != nil {
		messageID = i.Message.ID
	}
	ctx = withMessageRef(ctx, MessageRef{
		ChannelID: payload.ChannelID, MessageID: messageID, GuildID: payload.GuildID,
		UserID: payload.RequesterID, Username: payload.Username,
		IsAdmin: payload.IsAdmin, Scope: payload.Scope,
	})
	outputs := &tools.Outputs{}
	ctx = tools.WithOutputs(ctx, outputs)
	pending := &PendingCollector{}
	ctx = withPending(ctx, pending)
	gen, err := b.generate(ctx, cfg, payload.Scope, userText, nil, true)
	if err != nil {
		b.replyNotice(s, payload.ChannelID, b.recordError(cfg, err), cfg)
		return
	}
	b.replyGenerated(s, payload.ChannelID, gen, cfg)
	b.sendFiles(s, payload.ChannelID, outputs.Files)
	b.sendPendingButtons(s, payload.ChannelID, pending.Items)
}
