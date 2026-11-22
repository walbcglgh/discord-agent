package discord

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"discord-agent/internal/llm"
	"discord-agent/internal/tools"

	"github.com/bwmarrin/discordgo"
)

func RegisterEmojiTools(reg *tools.Registry, b *Bot) {
	reg.AddTool(llm.ToolDef{
		Name: "list_emojis",
		Description: "列出伺服器的自訂表情符號名稱與 ID。看不懂字面意思時可要求附上圖片，" +
			"若模型沒有影像能力就純靠名稱猜。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"include_images": map[string]any{"type": "boolean", "description": "是否附上表情圖片，最多 20 張"},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		ref, ok := messageRefFrom(ctx)
		if !ok || ref.GuildID == "" {
			return "", fmt.Errorf("私訊不處理表情符號工具")
		}
		var in struct {
			IncludeImages bool `json:"include_images"`
		}
		if len(args) > 0 && string(args) != "null" {
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
		}
		emojis, err := b.session.GuildEmojis(ref.GuildID)
		if err != nil {
			return "", err
		}
		var lines []string
		client := &http.Client{Timeout: 15 * time.Second}
		images := 0
		for _, e := range emojis {
			lines = append(lines, fmt.Sprintf("%s : %s", e.Name, e.ID))
			if !in.IncludeImages || images >= 20 {
				continue
			}
			ext := "png"
			if e.Animated {
				ext = "gif"
			}
			resp, err := client.Get("https://cdn.discordapp.com/emojis/" + e.ID + "." + ext + "?size=64")
			if err != nil {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(resp.Body, 512000))
			resp.Body.Close()
			if err != nil || len(data) == 0 {
				continue
			}
			mime := "image/png"
			if e.Animated {
				mime = "image/gif"
			}
			tools.AddImage(ctx, llm.Image{MimeType: mime, Data: base64.StdEncoding.EncodeToString(data)})
			images++
		}
		if len(lines) == 0 {
			return "這個伺服器沒有自訂表情符號。", nil
		}
		if in.IncludeImages {
			lines = append(lines, "（已附上表情圖片，若看不懂或看不到就依名稱判斷）")
		}
		return strings.Join(lines, "\n"), nil
	})

	reg.AddTool(llm.ToolDef{
		Name: "add_reaction",
		Description: "對觸發你的訊息加上表情符號反應。emoji 可填自訂表情名稱、ID 或 unicode emoji；" +
			"也可指定同頻道的 message_id。私訊不可用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"emoji":      map[string]any{"type": "string"},
				"message_id": map[string]any{"type": "string", "description": "省略表示目前這則訊息"},
			},
			"required": []string{"emoji"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		ref, ok := messageRefFrom(ctx)
		if !ok || ref.GuildID == "" || ref.ChannelID == "" {
			return "", fmt.Errorf("私訊不處理表情符號工具")
		}
		var in struct {
			Emoji     string `json:"emoji"`
			MessageID string `json:"message_id"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return "", err
		}
		emoji := strings.TrimSpace(in.Emoji)
		if emoji == "" {
			return "", fmt.Errorf("emoji 不可為空")
		}
		if !strings.Contains(emoji, ":") {
			if id, err := strconv.ParseUint(emoji, 10, 64); err == nil {
				emojis, err := b.session.GuildEmojis(ref.GuildID)
				if err != nil {
					return "", err
				}
				for _, e := range emojis {
					if e.ID == strconv.FormatUint(id, 10) {
						emoji = e.MessageFormat()
						break
					}
				}
			} else {
				emojis, err := b.session.GuildEmojis(ref.GuildID)
				if err != nil {
					return "", err
				}
				for _, e := range emojis {
					if strings.EqualFold(e.Name, emoji) {
						emoji = e.MessageFormat()
						break
					}
				}
			}
		}
		messageID := in.MessageID
		if messageID == "" {
			messageID = ref.MessageID
		}
		if messageID == "" {
			return "", fmt.Errorf("找不到目標訊息")
		}
		if messageID != ref.MessageID {
			msg, err := b.session.ChannelMessage(ref.ChannelID, messageID)
			if err != nil || msg.ChannelID != ref.ChannelID {
				return "", fmt.Errorf("只能對同頻道的訊息加反應")
			}
		}
		if err := b.session.MessageReactionAdd(ref.ChannelID, messageID, emoji); err != nil {
			return "", err
		}
		return "已加上表情符號反應。", nil
	})
}
