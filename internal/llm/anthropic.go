package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"discord-agent/internal/config"
)

type anthropicProvider struct {
	cfg    config.ModelConfig
	client *http.Client
}

func (p *anthropicProvider) Name() string { return "anthropic" }

func (p *anthropicProvider) Chat(ctx context.Context, req Request) (Response, error) {
	key := apiKey(p.cfg)
	if key == "" {
		return Response{}, fmt.Errorf("尚未設定 API key")
	}
	var systemParts []string
	messages := make([]any, 0, len(req.Messages))
	for i := 0; i < len(req.Messages); i++ {
		m := req.Messages[i]
		switch m.Role {
		case "system":
			systemParts = append(systemParts, m.Content)
		case "tool":
			blocks := make([]any, 0)
			for i < len(req.Messages) && req.Messages[i].Role == "tool" {
				t := req.Messages[i]
				blocks = append(blocks, map[string]any{
					"type": "tool_result", "tool_use_id": t.ToolCallID, "content": t.Content,
				})
				i++
			}
			i--
			messages = append(messages, map[string]any{"role": "user", "content": blocks})
		case "assistant":
			blocks := make([]any, 0)
			if m.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, map[string]any{
					"type": "tool_use", "id": tc.ID, "name": tc.Name, "input": argsObject(tc.Arguments),
				})
			}
			messages = append(messages, map[string]any{"role": "assistant", "content": blocks})
		default:
			blocks := []any{map[string]any{"type": "text", "text": m.Content}}
			for _, img := range m.Images {
				blocks = append(blocks, map[string]any{
					"type": "image",
					"source": map[string]any{
						"type": "base64", "media_type": img.MimeType, "data": img.Data,
					},
				})
			}
			messages = append(messages, map[string]any{"role": "user", "content": blocks})
		}
	}
	body := map[string]any{
		"model": req.Model, "messages": messages, "max_tokens": req.MaxTokens,
		"temperature": req.Temperature,
	}
	if len(systemParts) > 0 {
		body["system"] = strings.Join(systemParts, "\n\n")
	}
	if len(req.Tools) > 0 {
		tools := make([]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"name": t.Name, "description": t.Description, "input_schema": t.Parameters,
			})
		}
		body["tools"] = tools
	}
	payload, _ := json.Marshal(body)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL(p.cfg, "https://api.anthropic.com/v1")+"/messages", bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("x-api-key", key)
	hreq.Header.Set("anthropic-version", "2023-06-01")
	resp, err := p.client.Do(hreq)
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, httpError(resp)
	}
	defer resp.Body.Close()
	var out struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("解析 Anthropic 回應失敗: %w", err)
	}
	result := Response{InputTokens: out.Usage.InputTokens, OutputTokens: out.Usage.OutputTokens}
	var text strings.Builder
	for _, block := range out.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID: block.ID, Name: block.Name, Arguments: string(block.Input),
			})
		}
	}
	result.Content = text.String()
	if result.Content == "" && len(result.ToolCalls) == 0 {
		return Response{}, fmt.Errorf("Anthropic 沒有回傳文字或工具呼叫")
	}
	return result, nil
}
