package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"discord-agent/internal/config"
)

type geminiProvider struct {
	cfg    config.ModelConfig
	client *http.Client
}

func (p *geminiProvider) Name() string { return "gemini" }

func (p *geminiProvider) Chat(ctx context.Context, req Request) (Response, error) {
	key := apiKey(p.cfg)
	if key == "" {
		return Response{}, fmt.Errorf("尚未設定 API key")
	}
	contents := make([]any, 0, len(req.Messages))
	var systemParts []string
	for i := 0; i < len(req.Messages); i++ {
		m := req.Messages[i]
		switch m.Role {
		case "system":
			systemParts = append(systemParts, m.Content)
		case "tool":
			parts := make([]any, 0)
			for i < len(req.Messages) && req.Messages[i].Role == "tool" {
				t := req.Messages[i]
				parts = append(parts, map[string]any{
					"functionResponse": map[string]any{
						"name": t.Name, "response": map[string]any{"result": t.Content},
					},
				})
				i++
			}
			i--
			contents = append(contents, map[string]any{"role": "user", "parts": parts})
		case "assistant":
			parts := make([]any, 0)
			if m.Content != "" {
				parts = append(parts, map[string]any{"text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{"name": tc.Name, "args": argsObject(tc.Arguments)},
				})
			}
			contents = append(contents, map[string]any{"role": "model", "parts": parts})
		default:
			parts := []any{map[string]any{"text": m.Content}}
			for _, img := range m.Images {
				parts = append(parts, map[string]any{
					"inline_data": map[string]any{"mime_type": img.MimeType, "data": img.Data},
				})
			}
			contents = append(contents, map[string]any{"role": "user", "parts": parts})
		}
	}
	body := map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"maxOutputTokens": req.MaxTokens, "temperature": req.Temperature,
		},
	}
	if len(systemParts) > 0 {
		body["systemInstruction"] = map[string]any{
			"parts": []any{map[string]any{"text": strings.Join(systemParts, "\n\n")}},
		}
	}
	if len(req.Tools) > 0 {
		decls := make([]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			decls = append(decls, map[string]any{
				"name": t.Name, "description": t.Description, "parameters": t.Parameters,
			})
		}
		body["tools"] = []any{map[string]any{"functionDeclarations": decls}}
	}
	payload, _ := json.Marshal(body)
	endpoint := baseURL(p.cfg, "https://generativelanguage.googleapis.com/v1beta") +
		"/models/" + url.PathEscape(req.Model) + ":generateContent?key=" + url.QueryEscape(key)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(hreq)
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, httpError(resp)
	}
	defer resp.Body.Close()
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("解析 Gemini 回應失敗: %w", err)
	}
	if len(out.Candidates) == 0 {
		return Response{}, fmt.Errorf("Gemini 沒有回傳 candidate（可能被安全策略阻擋）")
	}
	result := Response{InputTokens: out.UsageMetadata.PromptTokenCount,
		OutputTokens: out.UsageMetadata.CandidatesTokenCount}
	var text strings.Builder
	for _, part := range out.Candidates[0].Content.Parts {
		if part.Text != "" {
			text.WriteString(part.Text)
		}
		if part.FunctionCall.Name != "" {
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID: "", Name: part.FunctionCall.Name, Arguments: string(part.FunctionCall.Args),
			})
		}
	}
	result.Content = text.String()
	return result, nil
}
