package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/llm"
	"discord-agent/internal/tools"
	"discord-agent/internal/util"
)

type Manager struct {
	store   *config.Store
	tools   *tools.Registry
	log     *log.Logger
	mu      sync.Mutex
	tokenMu sync.RWMutex
	nextID  atomic.Int64
	tokens  map[string]string
}

func New(store *config.Store, reg *tools.Registry, logger *log.Logger) *Manager {
	return &Manager{store: store, tools: reg, log: logger, tokens: make(map[string]string)}
}

// Refresh re-discovers tools from all enabled MCP servers.
func (m *Manager) Refresh(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools.ClearDynamic()
	cfg := m.store.Get()
	for _, s := range cfg.MCP.Servers {
		if !s.Enabled {
			continue
		}
		defs, err := m.discover(ctx, s)
		if err != nil {
			m.log.Printf("MCP %s 連線失敗: %s", s.Name, util.Redact(err.Error(), m.secrets()))
			continue
		}
		for _, d := range defs {
			def := d
			m.tools.AddDynamicTool(def.Def, func(ctx context.Context, args json.RawMessage) (string, error) {
				return m.callTool(ctx, def.Server, def.Original, args)
			})
		}
		m.log.Printf("MCP %s 已註冊 %d 個工具", s.Name, len(defs))
	}
}

type discovered struct {
	Def      llm.ToolDef
	Server   config.MCPServerConfig
	Original string
}

func (m *Manager) secrets() []string {
	cfg := m.store.Get()
	list := []string{}
	for _, s := range cfg.MCP.Servers {
		list = append(list, s.APIKey, osGet(s.APIKeyEnv), s.OAuthAccessToken,
			osGet(s.OAuthTokenEnv), s.OAuthRefreshToken, s.OAuthClientSecret)
	}
	return list
}

func osGet(name string) string {
	if name == "" {
		return ""
	}
	return os.Getenv(name)
}

func (m *Manager) token(s config.MCPServerConfig) string {
	switch s.AuthType {
	case "api_key":
		return config.Secret(s.APIKeyEnv, s.APIKey)
	case "oauth":
		m.tokenMu.RLock()
		t := m.tokens[s.ID]
		m.tokenMu.RUnlock()
		if t != "" {
			return t
		}
		return config.Secret(s.OAuthTokenEnv, s.OAuthAccessToken)
	default:
		return ""
	}
}

func (m *Manager) discover(ctx context.Context, s config.MCPServerConfig) ([]discovered, error) {
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := m.rpc(ctx, s, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "selana-agent", "version": "1.0"},
	}, &init); err != nil {
		return nil, err
	}
	var listed struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := m.rpc(ctx, s, "tools/list", map[string]any{}, &listed); err != nil {
		return nil, err
	}
	out := make([]discovered, 0, len(listed.Tools))
	for _, t := range listed.Tools {
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, discovered{
			Def: llm.ToolDef{
				Name: toolName(s.ID, t.Name), Description: t.Description, Parameters: schema,
			},
			Server: s, Original: t.Name,
		})
	}
	return out, nil
}

func (m *Manager) callTool(ctx context.Context, s config.MCPServerConfig, name string, args json.RawMessage) (string, error) {
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage(`{}`)
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	err := m.rpc(ctx, s, "tools/call", map[string]any{
		"name": name, "arguments": json.RawMessage(args),
	}, &out)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, c := range out.Content {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	result := strings.Join(parts, "\n")
	if out.IsError {
		return "", fmt.Errorf("MCP 工具回傳錯誤: %s", result)
	}
	return result, nil
}

func (m *Manager) rpc(ctx context.Context, s config.MCPServerConfig, method string, params any, out any) error {
	body := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	body["id"] = m.nextID.Add(1)
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token := m.token(s); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized && s.AuthType == "oauth" && s.OAuthRefreshToken != "" {
		if err := m.refresh(ctx, s); err == nil {
			return m.rpc(ctx, s, method, params, out)
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("MCP HTTP %d: %s", resp.StatusCode, string(b))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 500000))
	if err != nil {
		return err
	}
	data := raw
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		data = nil
		sc := bufio.NewScanner(bytes.NewReader(raw))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "data:") {
				data = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if len(data) == 0 {
			return fmt.Errorf("SSE 沒有資料事件")
		}
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("解析 MCP 回應失敗: %w", err)
	}
	if envelope.Error.Message != "" {
		return fmt.Errorf("MCP 錯誤: %s", envelope.Error.Message)
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) refresh(ctx context.Context, s config.MCPServerConfig) error {
	if s.OAuthTokenURL == "" {
		return fmt.Errorf("缺少 oauth_token_url")
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {s.OAuthRefreshToken}}
	if s.OAuthClientID != "" {
		form.Set("client_id", s.OAuthClientID)
	}
	if s.OAuthClientSecret != "" {
		form.Set("client_secret", s.OAuthClientSecret)
	}
	if len(s.OAuthScopes) > 0 {
		form.Set("scope", strings.Join(s.OAuthScopes, " "))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.OAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("OAuth refresh HTTP %d", resp.StatusCode)
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if out.AccessToken == "" {
		return fmt.Errorf("OAuth 沒有回傳 access_token")
	}
	m.tokenMu.Lock()
	m.tokens[s.ID] = out.AccessToken
	if out.RefreshToken != "" {
		m.tokens[s.ID+"_refresh"] = out.RefreshToken
	}
	m.tokenMu.Unlock()
	return nil
}

func toolName(serverID, original string) string {
	var b strings.Builder
	b.WriteString("mcp_")
	b.WriteString(sanitize(serverID))
	b.WriteString("_")
	b.WriteString(sanitize(original))
	name := b.String()
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
