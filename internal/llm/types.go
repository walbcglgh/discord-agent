package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"discord-agent/internal/config"
)

type Image struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64, without data: prefix
}

func (i Image) DataURL() string { return "data:" + i.MimeType + ";base64," + i.Data }

type Message struct {
	Role       string     `json:"role"` // system | user | assistant | tool
	Content    string     `json:"content,omitempty"`
	Images     []Image    `json:"images,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // raw JSON object as string
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Request struct {
	Model       string
	Messages    []Message
	Tools       []ToolDef
	MaxTokens   int
	Temperature float64
}

type Response struct {
	Content      string
	ToolCalls    []ToolCall
	InputTokens  int
	OutputTokens int
}

type Provider interface {
	Name() string
	Chat(ctx context.Context, req Request) (Response, error)
}

func New(mc config.ModelConfig) (Provider, error) {
	switch strings.ToLower(mc.Protocol) {
	case config.ProtocolOpenAIChat:
		return &openAIProvider{cfg: mc, client: httpClient()}, nil
	case config.ProtocolOpenAIResponses:
		return &responsesProvider{cfg: mc, client: httpClient()}, nil
	case config.ProtocolAnthropic:
		return &anthropicProvider{cfg: mc, client: httpClient()}, nil
	case config.ProtocolGoogle, "gemini":
		return &geminiProvider{cfg: mc, client: httpClient()}, nil
	default:
		return nil, fmt.Errorf("不支援的協議: %s", mc.Protocol)
	}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second}
}

// apiKey resolves the panel-entered key first, then the environment variable.
func apiKey(mc config.ModelConfig) string {
	return config.Secret(mc.APIKeyEnv, mc.APIKey)
}

func baseURL(mc config.ModelConfig, fallback string) string {
	u := strings.TrimSpace(mc.BaseURL)
	if u == "" {
		u = fallback
	}
	return strings.TrimRight(u, "/")
}

func httpError(resp *http.Response) error {
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("API 回傳 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}

func argsObject(raw string) json.RawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return json.RawMessage(`{}`)
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		b, _ := json.Marshal(map[string]string{"value": raw})
		return b
	}
	// Anthropic/Gemini require an object; wrap scalar values.
	if _, ok := v.(map[string]any); !ok {
		b, _ := json.Marshal(map[string]any{"value": v})
		return b
	}
	return json.RawMessage(raw)
}
