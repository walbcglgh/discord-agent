package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"discord-agent/internal/config"
)

type openAIProvider struct {
	cfg    config.ModelConfig
	client *http.Client
}

func (p *openAIProvider) Name() string { return "openai-chat" }

func (p *openAIProvider) Chat(ctx context.Context, req Request) (Response, error) {
	key := apiKey(p.cfg)
	if key == "" {
		return Response{}, fmt.Errorf("尚未設定 API key")
	}
	messages := make([]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case "tool":
			messages = append(messages, map[string]any{
				"role": "tool", "tool_call_id": m.ToolCallID, "content": m.Content,
			})
		case "assistant":
			item := map[string]any{"role": "assistant", "content": m.Content}
			if len(m.ToolCalls) > 0 {
				calls := make([]any, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					calls = append(calls, map[string]any{
						"id": tc.ID, "type": "function",
						"function": map[string]any{"name": tc.Name, "arguments": argsObject(tc.Arguments)},
					})
				}
				item["tool_calls"] = calls
			}
			messages = append(messages, item)
		default:
			if len(m.Images) == 0 {
				messages = append(messages, map[string]any{"role": m.Role, "content": m.Content})
				break
			}
			content := []any{map[string]any{"type": "text", "text": m.Content}}
			for _, img := range m.Images {
				content = append(content, map[string]any{
					"type": "image_url", "image_url": map[string]any{"url": img.DataURL()},
				})
			}
			messages = append(messages, map[string]any{"role": m.Role, "content": content})
		}
	}
	body := map[string]any{
		"model": req.Model, "messages": messages,
		"max_tokens": req.MaxTokens, "temperature": req.Temperature,
	}
	if len(req.Tools) > 0 {
		tools := make([]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": t.Name, "description": t.Description, "parameters": t.Parameters,
				},
			})
		}
		body["tools"] = tools
	}
	payload, _ := json.Marshal(body)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL(p.cfg, "https://api.openai.com/v1")+"/chat/completions", bytes.NewReader(payload))
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
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("解析 OpenAI 回應失敗: %w", err)
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("API 沒有回傳任何 choice")
	}
	result := Response{Content: out.Choices[0].Message.Content,
		InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens}
	for _, tc := range out.Choices[0].Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Arguments: string(tc.Function.Arguments),
		})
	}
	return result, nil
}
