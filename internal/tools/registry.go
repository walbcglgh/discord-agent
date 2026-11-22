package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/llm"
)

type MemoryWriter interface {
	Remember(scope, key, value string) error
}

type scopeKey struct{}

func WithScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func scopeFrom(ctx context.Context) string {
	if s, ok := ctx.Value(scopeKey{}).(string); ok {
		return s
	}
	return "global"
}

type handler func(context.Context, json.RawMessage) (string, error)

type Registry struct {
	mu       sync.RWMutex
	defs     []llm.ToolDef
	handlers map[string]handler
	dynamic  map[string]bool
}

func New(cfg config.ToolsConfig, mem MemoryWriter) *Registry {
	r := &Registry{handlers: make(map[string]handler), dynamic: make(map[string]bool)}
	if cfg.Calculator {
		r.add("calculator", "計算數學運算式，支援 + - * / 與括號。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"expression": map[string]any{"type": "string", "description": "運算式，例如 (12+3)*2"},
				},
				"required": []string{"expression"},
			}, calcHandler)
	}
	if cfg.Clock {
		r.add("get_time", "取得目前的日期與時間。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"timezone": map[string]any{"type": "string", "description": "IANA 時區，例如 Asia/Taipei；可省略"},
				},
			}, clockHandler)
	}
	if cfg.WebSearch.Enabled {
		r.add("web_search", "搜尋網路並回傳標題、網址與摘要。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜尋關鍵字"},
				},
				"required": []string{"query"},
			}, webSearchHandler(cfg.WebSearch))
	}
	if cfg.AllowMemoryWrite && mem != nil {
		r.add("remember", "把使用者的偏好或重要事實存入長期記憶。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":   map[string]any{"type": "string", "description": "簡短鍵名，例如 暱稱"},
					"value": map[string]any{"type": "string", "description": "要記住的內容"},
				},
				"required": []string{"key", "value"},
			}, func(ctx context.Context, args json.RawMessage) (string, error) {
				var in struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}
				if err := json.Unmarshal(args, &in); err != nil {
					return "", err
				}
				if in.Key == "" || in.Value == "" {
					return "", fmt.Errorf("key 與 value 不可為空")
				}
				if err := mem.Remember(scopeFrom(ctx), in.Key, in.Value); err != nil {
					return "", err
				}
				return "已記住：" + in.Key, nil
			})
	}
	return r
}

func (r *Registry) add(name, desc string, params map[string]any, h handler) {
	r.defs = append(r.defs, llm.ToolDef{Name: name, Description: desc, Parameters: params})
	r.handlers[name] = h
}

// AddTool registers a permanent tool (Discord, VPS, media, etc.).
func (r *Registry) AddTool(def llm.ToolDef, fn func(context.Context, json.RawMessage) (string, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs = append(r.defs, def)
	r.handlers[def.Name] = fn
	delete(r.dynamic, def.Name)
}

// AddDynamicTool registers a tool that can be cleared on MCP refresh.
func (r *Registry) AddDynamicTool(def llm.ToolDef, fn func(context.Context, json.RawMessage) (string, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs = append(r.defs, def)
	r.handlers[def.Name] = fn
	r.dynamic[def.Name] = true
}

// ClearDynamic removes all MCP-provided tools before a refresh.
func (r *Registry) ClearDynamic() {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := make([]llm.ToolDef, 0, len(r.defs))
	for _, d := range r.defs {
		if r.dynamic[d.Name] {
			delete(r.handlers, d.Name)
			delete(r.dynamic, d.Name)
			continue
		}
		kept = append(kept, d)
	}
	r.defs = kept
}

func (r *Registry) Definitions() []llm.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]llm.ToolDef(nil), r.defs...)
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	h, ok := r.handlers[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("未知工具: %s", name)
	}
	return h(ctx, args)
}

func calcHandler(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	v, err := evalExpr(in.Expression)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%.10g", v), nil
}

func clockHandler(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Timezone string `json:"timezone"`
	}
	if len(args) > 0 && string(args) != "null" {
		if err := json.Unmarshal(args, &in); err != nil {
			return "", err
		}
	}
	loc := time.UTC
	if in.Timezone != "" {
		l, err := time.LoadLocation(in.Timezone)
		if err != nil {
			return "", fmt.Errorf("未知時區 %q: %w", in.Timezone, err)
		}
		loc = l
	}
	return time.Now().In(loc).Format("2006-01-02 15:04:05 MST (Mon)"), nil
}

func webSearchHandler(cfg config.WebSearchConfig) handler {
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		key := config.Secret(cfg.APIKeyEnv, cfg.APIKey)
		if key == "" {
			return "", fmt.Errorf("尚未設定網頁搜尋 API key")
		}
		var in struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Query) == "" {
			return "", fmt.Errorf("query 不可為空")
		}
		payload, _ := json.Marshal(map[string]any{
			"api_key": key, "query": in.Query, "search_depth": "basic",
			"max_results": cfg.MaxResults,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 25 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 20000))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("搜尋 API HTTP %d: %s", resp.StatusCode, string(body))
		}
		var out struct {
			Results []struct {
				Title   string `json:"title"`
				URL     string `json:"url"`
				Content string `json:"content"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return "", err
		}
		var sb strings.Builder
		for i, res := range out.Results {
			fmt.Fprintf(&sb, "%d. %s\n%s\n%s\n\n", i+1, res.Title, res.URL, res.Content)
		}
		if sb.Len() == 0 {
			return "找不到相關結果。", nil
		}
		return sb.String(), nil
	}
}
