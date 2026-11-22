package discord

import (
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"discord-agent/internal/llm"
	"discord-agent/internal/util"

	"github.com/bwmarrin/discordgo"
)

// imageAttachments downloads up to four images and passes them straight to
// the upstream model. Models without vision will simply say they cannot see it.
func (b *Bot) imageAttachments(m *discordgo.MessageCreate) []llm.Image {
	var out []llm.Image
	client := &http.Client{Timeout: 20 * time.Second}
	for _, a := range m.Attachments {
		if len(out) >= 4 {
			break
		}
		if !strings.HasPrefix(a.ContentType, "image/") || a.Size > 8<<20 {
			continue
		}
		resp, err := client.Get(a.URL)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		mime := a.ContentType
		if i := strings.IndexByte(mime, ';'); i > 0 {
			mime = mime[:i]
		}
		out = append(out, llm.Image{MimeType: mime, Data: base64.StdEncoding.EncodeToString(data)})
	}
	return out
}

func (b *Bot) sendFiles(s *discordgo.Session, channelID string, paths []string) {
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		_, err = s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Files:           []*discordgo.File{{Name: filepath.Base(path), Reader: f}},
			AllowedMentions: allowedUserMentions(),
		})
		f.Close()
		_ = os.Remove(path)
		if err != nil {
			b.log.Printf("傳送檔案失敗: %s", util.Redact(err.Error(), b.secrets()))
		}
	}
}
