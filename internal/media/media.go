package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/llm"
	"discord-agent/internal/tools"
	"discord-agent/internal/util"
)

func TempDir() string { return filepath.Join("data", "tmp") }

// Transcribe sends audio to Groq whisper-large-v3.
func Transcribe(ctx context.Context, cfg config.MediaConfig, path string) (string, error) {
	key := config.Secret(cfg.GroqAPIKeyEnv, cfg.GroqAPIKey)
	if key == "" {
		return "", fmt.Errorf("尚未設定 Groq API Key")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	_ = w.WriteField("model", "whisper-large-v3")
	_ = w.WriteField("response_format", "text")
	_ = w.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.groq.com/openai/v1/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 500000))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Groq HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(b, &out) == nil && out.Text != "" {
		return strings.TrimSpace(out.Text), nil
	}
	return strings.TrimSpace(string(b)), nil
}

// Speak uses edge-tts (python3 -m edge_tts) and returns the generated mp3 path.
func Speak(ctx context.Context, cfg config.MediaConfig, text string) (string, error) {
	if _, err := exec.LookPath("python3"); err != nil {
		return "", fmt.Errorf("伺服器沒有 python3 / edge-tts")
	}
	if err := os.MkdirAll(TempDir(), 0700); err != nil {
		return "", err
	}
	path := filepath.Join(TempDir(), fmt.Sprintf("tts_%d.mp3", time.Now().UnixNano()))
	voice := cfg.TTSVoice
	if voice == "" {
		voice = "zh-TW-HsiaoChenNeural"
	}
	cmd := exec.CommandContext(ctx, "python3", "-m", "edge_tts",
		"--voice", voice, "--text", text, "--write-media", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("edge-tts 失敗: %s", util.Redact(string(out), []string{text}))
	}
	return path, nil
}

// RegisterTTSTool adds a tool whose mp3 is delivered via tools.Outputs.
func RegisterTTSTool(reg *tools.Registry, cfg func() config.MediaConfig) {
	reg.AddTool(llm.ToolDef{
		Name: "speak",
		Description: "把一段文字合成為語音並直接傳送 mp3 給使用者。" +
			"超過 2 分鐘的音訊做不到，請直接告知使用者。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "要合成的文字"},
			},
			"required": []string{"text"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var in struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return "", err
		}
		text := strings.TrimSpace(in.Text)
		if text == "" {
			return "", fmt.Errorf("text 不可為空")
		}
		c := cfg()
		limit := c.MaxTTSChars
		if limit < 1 {
			limit = 500
		}
		if len([]rune(text)) > limit {
			return "抱歉，超過 2 分鐘的音訊我做不到。", nil
		}
		path, err := Speak(ctx, c, text)
		if err != nil {
			return "", err
		}
		tools.AddFile(ctx, path)
		return "語音已產生。", nil
	})
}
