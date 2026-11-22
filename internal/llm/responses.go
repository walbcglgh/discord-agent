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

type responsesProvider struct {
	cfg    config.ModelConfig
	client *http.Client
}

func (p *responsesProvider) Name() string { return "openai-responses" }

func (p *responsesProvider) Chat(ctx context.Context, req Request) (Response, error) {
	key := apiKey(p.cfg)
	if key == "" {
		return Response{}, fmt.Errorf("尚未設定 %s 的 API key", p.cfg.APIKeyEnv)
	}
	var instructions []string
	input := make([]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			instructions = append(instructions, m.Content)
		case "tool":
			input = append(input, map[string]any{
				"type": "function_call_output", "call_id": m.ToolCallID, "output": m.Content,
			})
		case "assistant":
			if m.Content != "" {
				input = append(input, map[string]any{
					"role":    "assistant",
					"content": []any{map[string]any{"type": "output_text", "text": m.Content}},
				})
			}
			for _, tc := range m.ToolCalls {
				input = append(input, map[string]any{
					"type": "function_call", "call_id": tc.ID,
					"name": tc.Name, "arguments": argsObject(tc.Arguments),
				})
			}
		default:
			parts := []any{map[string]any{"type": "input_text", "text": m.Content}}
			for _, img := range m.Images {
				parts = append(parts, map[string]any{"type": "input_image", "image_url": img.DataURL()})
			}
			input = append(input, map[string]any{"role": "user", "content": parts})
		}
	}
	body := map[string]any{
		"model": req.Model, "input": input,
		"max_output_tokens": req.MaxTokens, "temperature": req.Temperature,
	}
	if len(instructions) > 0 {
		body["instructions"] = strings.Join(instructions, "\n\n")
	}
	if len(req.Tools) > 0 {
		tools := make([]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function", "name": t.Name,
				"description": t.Description, "parameters": t.Parameters,
			})
		}
		body["tools"] = tools
	}
	payload, _ := json.Marshal(body)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL(p.cfg, "https://api.openai.com/v1")+"/responses", bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+key)
	resp, err := p.client.Do(hreq)
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, httpError(resp)
	}
	defer resp.Body.Close()
	var out struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			CallID    string          `json:"call_id"`
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("解析 Responses 回應失敗: %w", err)
	}
	result := Response{InputTokens: out.Usage.InputTokens, OutputTokens: out.Usage.OutputTokens}
	var text strings.Builder
	for _, item := range out.Output {
		for _, c := range item.Content {
			if c.Text != "" {
				text.WriteString(c.Text)
			}
		}
		if item.Type == "function_call" && item.Name != "" {
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID: item.CallID, Name: item.Name, Arguments: string(item.Arguments),
			})
		}
	}
	result.Content = text.String()
	if result.Content == "" && len(result.ToolCalls) == 0 {
		return Response{}, fmt.Errorf("Responses API 沒有回傳內容")
	}
	return result, nil
}
