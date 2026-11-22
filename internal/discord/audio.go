package discord

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/media"

	"github.com/bwmarrin/discordgo"
)

type audioResult struct {
	Text    string
	TooLong bool
}

func (b *Bot) audioText(ctx context.Context, m *discordgo.MessageCreate, cfg *config.Config) audioResult {
	limit := cfg.Media.MaxAudioSeconds
	if limit < 1 {
		limit = 120
	}
	client := &http.Client{Timeout: 30 * time.Second}
	result := audioResult{}
	for _, a := range m.Attachments {
		name := strings.ToLower(a.Filename)
		if !strings.HasPrefix(a.ContentType, "audio/") &&
			!strings.HasSuffix(name, ".mp3") && !strings.HasSuffix(name, ".wav") &&
			!strings.HasSuffix(name, ".ogg") && !strings.HasSuffix(name, ".m4a") &&
			!strings.HasSuffix(name, ".webm") {
			continue
		}
		if a.Size > 25<<20 {
			result.TooLong = true
			return result
		}
		resp, err := client.Get(a.URL)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 25<<20))
		resp.Body.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		if err := os.MkdirAll(media.TempDir(), 0700); err != nil {
			continue
		}
		path := filepath.Join(media.TempDir(), fmt.Sprintf("audio_%d_%s", time.Now().UnixNano(), filepath.Base(name)))
		if err := os.WriteFile(path, data, 0600); err != nil {
			continue
		}
		seconds := audioSeconds(ctx, path, int64(len(data)))
		if seconds > float64(limit) {
			result.TooLong = true
			os.Remove(path)
			return result
		}
		text, err := media.Transcribe(ctx, cfg.Media, path)
		os.Remove(path)
		if err == nil && strings.TrimSpace(text) != "" {
			if result.Text != "" {
				result.Text += "\n"
			}
			result.Text += strings.TrimSpace(text)
		}
	}
	return result
}

func audioSeconds(ctx context.Context, path string, size int64) float64 {
	if bin, err := exec.LookPath("ffprobe"); err == nil {
		out, err := exec.CommandContext(ctx, bin, "-v", "error", "-show_entries",
			"format=duration", "-of", "default=nw=1:nk=1", path).Output()
		if err == nil {
			if v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
				return v
			}
		}
	}
	// 128kbps fallback: ~16000 bytes per second.
	return float64(size) / 16000
}
