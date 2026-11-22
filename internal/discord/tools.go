package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"discord-agent/internal/llm"
	"discord-agent/internal/tools"

	"github.com/bwmarrin/discordgo"
)

// RegisterContextTool gives the model a way to inspect the conversation above
// the message that mentioned it. The model decides the count (max 100).
func RegisterContextTool(reg *tools.Registry, s *discordgo.Session) {
	reg.AddTool(llm.ToolDef{
		Name: "read_channel_history",
		Description: "讀取觸發你回覆的訊息之前的頻道訊息，也可用 channel_id 讀同伺服器的其他頻道。" +
			"limit 由你決定，最多 100 則；建議一次只查 10 則，查太多會耗光 tokens。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type": "integer", "minimum": 1, "maximum": 100,
					"description": "要抓取的訊息數量，建議 10，最多 100",
				},
				"channel_id": map[string]any{
					"type": "string", "description": "要讀取的頻道 ID；省略表示當前頻道，只能讀同伺服器",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		ref, ok := messageRefFrom(ctx)
		if !ok || ref.ChannelID == "" {
			return "", fmt.Errorf("目前沒有可勘查的頻道上下文")
		}
		var in struct {
			Limit     int    `json:"limit"`
			ChannelID string `json:"channel_id"`
		}
		if len(args) > 0 && string(args) != "null" {
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
		}
		if in.Limit < 1 {
			in.Limit = 10
		}
		if in.Limit > 100 {
			in.Limit = 100
		}
		channelID, beforeID := ref.ChannelID, ref.MessageID
		if in.ChannelID != "" && in.ChannelID != ref.ChannelID {
			ch, err := s.Channel(in.ChannelID)
			if err != nil || ch.GuildID != ref.GuildID {
				return "", fmt.Errorf("只能讀取同一伺服器的頻道")
			}
			channelID, beforeID = in.ChannelID, ""
		}
		msgs, err := s.ChannelMessages(channelID, in.Limit, beforeID, "", "")
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		for i := len(msgs) - 1; i >= 0; i-- {
			m := msgs[i]
			name := "unknown"
			if m.Author != nil {
				name = m.Author.Username
			}
			fmt.Fprintf(&sb, "[%s] %s\n", name, strings.ReplaceAll(m.Content, "\n", " "))
			for _, a := range m.Attachments {
				fmt.Fprintf(&sb, "[附件 %s]\n", a.Filename)
			}
		}
		if sb.Len() == 0 {
			return "（該則訊息之前沒有其他訊息）", nil
		}
		r := []rune(sb.String())
		if len(r) > 20000 {
			return string(r[:20000]) + "\n（已截斷）", nil
		}
		return sb.String(), nil
	})
}
