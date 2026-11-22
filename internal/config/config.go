package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	ProtocolOpenAIChat      = "openai-chat"
	ProtocolOpenAIResponses = "openai-responses"
	ProtocolAnthropic       = "anthropic"
	ProtocolGoogle          = "google"
)

type Config struct {
	AssistantName      string         `json:"assistant_name"`
	UnavailableMessage string         `json:"unavailable_message"`
	Display            DisplayConfig  `json:"display"`
	Discord            DiscordConfig  `json:"discord"`
	Panel              PanelConfig    `json:"panel"`
	LLM                LLMConfig      `json:"llm"`
	MCP                MCPConfig      `json:"mcp"`
	VPS                VPSConfig      `json:"vps"`
	Media              MediaConfig    `json:"media"`
	Chat               ChatConfig     `json:"chat"`
	Memory             MemoryConfig   `json:"memory"`
	Tools              ToolsConfig    `json:"tools"`
	Database           DatabaseConfig `json:"database"`
}

type DisplayConfig struct {
	Thinking  bool `json:"thinking"`
	ToolCalls bool `json:"tool_calls"`
	RawOutput bool `json:"raw_output"`
}

type DiscordConfig struct {
	TokenEnv          string   `json:"token_env"`
	Token             string   `json:"token,omitempty"`
	ReplyMode         string   `json:"reply_mode"`
	AllowedChannelIDs []string `json:"allowed_channel_ids"`
	AdminUserIDs      []string `json:"admin_user_ids"`
	AllowedGuildIDs   []string `json:"allowed_guild_ids"`
	DMEnabled         bool     `json:"dm_enabled"`
	TypingIndicator   bool     `json:"typing_indicator"`
	RateLimitSeconds  int      `json:"rate_limit_seconds"`
	MaxReplyChars     int      `json:"max_reply_chars"`
}

type PanelConfig struct {
	Enabled      bool     `json:"enabled"`
	Listen       string   `json:"listen"`
	Username     string   `json:"username"`
	PasswordHash string   `json:"password_hash"`
	SessionHours int      `json:"session_hours"`
	AllowedIPs   []string `json:"allowed_ips"`
}

type LLMConfig struct {
	Models []ModelConfig `json:"models"`
	// Legacy fields are only used to migrate old configs; the panel never shows them.
	LegacyDefaultProvider string                    `json:"default_provider,omitempty"`
	LegacyProviders       map[string]ProviderConfig `json:"providers,omitempty"`
}

// ModelConfig is an independent upstream model. Name is the routing group and
// the text shown in the footer; multiple models may share the same Name.
type ModelConfig struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Protocol    string  `json:"protocol"`
	BaseURL     string  `json:"base_url,omitempty"`
	APIKeyEnv   string  `json:"api_key_env,omitempty"`
	APIKey      string  `json:"api_key,omitempty"`
	ModelID     string  `json:"model_id"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Enabled     bool    `json:"enabled"`
}

// ProviderConfig is kept only for reading old config.json files.
type ProviderConfig struct {
	Type        string  `json:"type"`
	BaseURL     string  `json:"base_url,omitempty"`
	APIKeyEnv   string  `json:"api_key_env,omitempty"`
	APIKey      string  `json:"api_key,omitempty"`
	Model       string  `json:"model"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Enabled     bool    `json:"enabled"`
}

type MCPConfig struct {
	Servers []MCPServerConfig `json:"servers"`
}

type MCPServerConfig struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	URL               string   `json:"url"`
	AuthType          string   `json:"auth_type"` // none | api_key | oauth
	APIKeyEnv         string   `json:"api_key_env,omitempty"`
	APIKey            string   `json:"api_key,omitempty"`
	OAuthTokenEnv     string   `json:"oauth_token_env,omitempty"`
	OAuthAccessToken  string   `json:"oauth_access_token,omitempty"`
	OAuthRefreshToken string   `json:"oauth_refresh_token,omitempty"`
	OAuthTokenURL     string   `json:"oauth_token_url,omitempty"`
	OAuthClientID     string   `json:"oauth_client_id,omitempty"`
	OAuthClientSecret string   `json:"oauth_client_secret,omitempty"`
	OAuthScopes       []string `json:"oauth_scopes,omitempty"`
	Enabled           bool     `json:"enabled"`
}

// VPSConfig is admin-only. These values are never exposed to the model.
type VPSConfig struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	AuthType    string `json:"auth_type"` // password | key
	PasswordEnv string `json:"password_env,omitempty"`
	Password    string `json:"password,omitempty"`
	KeyEnv      string `json:"key_env,omitempty"`
	Key         string `json:"key,omitempty"`
	Passphrase  string `json:"passphrase,omitempty"`
	WakeURL     string `json:"wake_url,omitempty"`
}

type MediaConfig struct {
	GroqAPIKeyEnv   string `json:"groq_api_key_env,omitempty"`
	GroqAPIKey      string `json:"groq_api_key,omitempty"`
	TTSVoice        string `json:"tts_voice"`
	MaxTTSChars     int    `json:"max_tts_chars"`
	MaxAudioSeconds int    `json:"max_audio_seconds"`
}

type ChatConfig struct {
	SystemPrompt    string  `json:"system_prompt"`
	ContextMessages int     `json:"context_messages"`
	MaxOutputTokens int     `json:"max_output_tokens"`
	Temperature     float64 `json:"temperature"`
	EnableTools     bool    `json:"enable_tools"`
	MaxToolRounds   int     `json:"max_tool_rounds"`
	SummaryEvery    int     `json:"summary_every"`
	ReplyLanguage   string  `json:"reply_language"`
}

type MemoryConfig struct {
	Enabled  bool `json:"enabled"`
	MaxFacts int  `json:"max_facts"`
}

type ToolsConfig struct {
	Calculator       bool            `json:"calculator"`
	Clock            bool            `json:"clock"`
	WebSearch        WebSearchConfig `json:"web_search"`
	AllowMemoryWrite bool            `json:"allow_memory_write"`
}

type WebSearchConfig struct {
	Enabled    bool   `json:"enabled"`
	Provider   string `json:"provider"`
	APIKeyEnv  string `json:"api_key_env,omitempty"`
	APIKey     string `json:"api_key,omitempty"`
	MaxResults int    `json:"max_results"`
}

type DatabaseConfig struct {
	Path string `json:"path"`
}

func Default() *Config {
	return &Config{
		AssistantName:      "工作小助手",
		UnavailableMessage: "(；ω；｀) {assistant}暫時無法回覆，請稍後再試",
		Display:            DisplayConfig{},
		Discord: DiscordConfig{
			TokenEnv: "DISCORD_BOT_TOKEN", Token: "", ReplyMode: "hybrid",
			AllowedChannelIDs: []string{}, AdminUserIDs: []string{}, AllowedGuildIDs: []string{},
			DMEnabled: false, TypingIndicator: true, RateLimitSeconds: 2, MaxReplyChars: 1900,
		},
		Panel: PanelConfig{
			Enabled: false, Listen: "127.0.0.1:8080", Username: "admin",
			SessionHours: 24, AllowedIPs: []string{},
		},
		LLM: LLMConfig{Models: []ModelConfig{}},
		MCP: MCPConfig{Servers: []MCPServerConfig{}},
		VPS: VPSConfig{Port: 22, User: "root", AuthType: "key", KeyEnv: "VPS_SSH_KEY"},
		Media: MediaConfig{
			GroqAPIKeyEnv: "GROQ_API_KEY", TTSVoice: "zh-TW-HsiaoChenNeural",
			MaxTTSChars: 500, MaxAudioSeconds: 120,
		},
		Chat: ChatConfig{
			SystemPrompt:    "你是 Discord 伺服器的 AI 助理。語氣簡潔、專業、友善，不寫冗詞贅句，使用繁體中文回覆。",
			ContextMessages: 20, MaxOutputTokens: 1024, Temperature: 0.7,
			EnableTools: true, MaxToolRounds: 3, SummaryEvery: 20, ReplyLanguage: "zh-TW",
		},
		Memory: MemoryConfig{Enabled: true, MaxFacts: 50},
		Tools: ToolsConfig{
			Calculator: true, Clock: true, AllowMemoryWrite: false,
			WebSearch: WebSearchConfig{Enabled: false, Provider: "tavily", APIKeyEnv: "TAVILY_API_KEY", MaxResults: 3},
		},
		Database: DatabaseConfig{Path: "data/agent.db"},
	}
}

func NewID(prefix string) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return prefix + fmt.Sprintf("%d", os.Getpid())
	}
	return prefix + hex.EncodeToString(b)
}

func (c *Config) Clone() (*Config, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var out Config
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type Store struct {
	mu   sync.RWMutex
	path string
	cfg  *Config
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load() error {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		if err := writeAtomic(s.path, cfg); err != nil {
			return err
		}
		s.set(cfg)
		return nil
	}
	if err != nil {
		return err
	}
	cfg := Default()
	if err := json.Unmarshal(b, cfg); err != nil {
		return fmt.Errorf("設定檔 JSON 解析失敗: %w", err)
	}
	migrateLegacy(cfg)
	if err := validate(cfg); err != nil {
		return err
	}
	s.set(cfg)
	return nil
}

func (s *Store) set(cfg *Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

func migrateLegacy(cfg *Config) {
	if len(cfg.LLM.Models) > 0 || len(cfg.LLM.LegacyProviders) == 0 {
		return
	}
	for name, p := range cfg.LLM.LegacyProviders {
		protocol := ProtocolOpenAIChat
		switch p.Type {
		case "anthropic":
			protocol = ProtocolAnthropic
		case "gemini", "google":
			protocol = ProtocolGoogle
		}
		cfg.LLM.Models = append(cfg.LLM.Models, ModelConfig{
			ID: NewID("m_"), Name: name, Protocol: protocol, BaseURL: p.BaseURL,
			APIKeyEnv: p.APIKeyEnv, APIKey: p.APIKey, ModelID: p.Model,
			MaxTokens: p.MaxTokens, Temperature: p.Temperature, Enabled: p.Enabled,
		})
	}
	cfg.LLM.LegacyProviders = nil
	cfg.LLM.LegacyDefaultProvider = ""
}

// Reload re-reads config.json so a separately running panel can hot-update
// the long-running agent process.
func (s *Store) Reload() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	cfg := Default()
	if err := json.Unmarshal(b, cfg); err != nil {
		return err
	}
	migrateLegacy(cfg)
	if err := validate(cfg); err != nil {
		return err
	}
	s.set(cfg)
	return nil
}

func (s *Store) Get() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Store) Replace(cfg *Config) error {
	if err := validate(cfg); err != nil {
		return err
	}
	if err := writeAtomic(s.path, cfg); err != nil {
		return err
	}
	s.set(cfg)
	return nil
}

func (s *Store) Path() string { return s.path }

func writeAtomic(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func validProtocol(p string) bool {
	switch p {
	case ProtocolOpenAIChat, ProtocolOpenAIResponses, ProtocolAnthropic, ProtocolGoogle:
		return true
	}
	return false
}

func validate(c *Config) error {
	if strings.TrimSpace(c.AssistantName) == "" {
		return errors.New("assistant_name 不可為空")
	}
	if strings.TrimSpace(c.UnavailableMessage) == "" {
		return errors.New("unavailable_message 不可為空")
	}
	switch c.Discord.ReplyMode {
	case "hybrid", "mention", "channel":
	default:
		return errors.New("discord.reply_mode 必須是 hybrid、mention 或 channel")
	}
	if strings.TrimSpace(c.Discord.TokenEnv) == "" && strings.TrimSpace(c.Discord.Token) == "" {
		return errors.New("必須設定 discord.token_env 或 discord.token")
	}
	if c.Discord.MaxReplyChars < 500 || c.Discord.MaxReplyChars > 2000 {
		return errors.New("discord.max_reply_chars 必須在 500~2000")
	}
	if c.Chat.ContextMessages < 1 || c.Chat.ContextMessages > 200 {
		return errors.New("chat.context_messages 必須在 1~200")
	}
	if c.Chat.MaxToolRounds < 0 || c.Chat.MaxToolRounds > 10 {
		return errors.New("chat.max_tool_rounds 必須在 0~10")
	}
	for _, m := range c.LLM.Models {
		if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.ModelID) == "" {
			return errors.New("每個模型的 name 與 model_id 都不可為空")
		}
		if !validProtocol(m.Protocol) {
			return fmt.Errorf("模型 %q 的 protocol 不支援", m.Name)
		}
		if m.Enabled && m.APIKey == "" && m.APIKeyEnv == "" {
			return fmt.Errorf("已啟用的模型 %q 必須設定 api_key 或 api_key_env", m.Name)
		}
	}
	for _, s := range c.MCP.Servers {
		if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.URL) == "" {
			return errors.New("每個 MCP 伺服器的 name 與 url 都不可為空")
		}
		switch s.AuthType {
		case "none", "api_key", "oauth":
		default:
			return fmt.Errorf("MCP %q 的 auth_type 必須是 none、api_key 或 oauth", s.Name)
		}
	}
	if c.VPS.Enabled {
		if strings.TrimSpace(c.VPS.Host) == "" || c.VPS.Port < 1 || c.VPS.Port > 65535 {
			return errors.New("VPS host 或 port 不合法")
		}
		switch c.VPS.AuthType {
		case "password", "key":
		default:
			return errors.New("VPS auth_type 必須是 password 或 key")
		}
	}
	if c.Media.MaxTTSChars < 1 {
		c.Media.MaxTTSChars = 500
	}
	if c.Media.MaxAudioSeconds < 1 {
		c.Media.MaxAudioSeconds = 120
	}
	if c.Panel.Enabled {
		if c.Panel.Username == "" || (c.Panel.PasswordHash == "" && strings.TrimSpace(os.Getenv("PANEL_PASSWORD")) == "") {
			return errors.New("啟用面板時必須設定 username 與 password_hash")
		}
		if c.Panel.SessionHours < 1 || c.Panel.SessionHours > 720 {
			return errors.New("panel.session_hours 必須在 1~720")
		}
	}
	return nil
}
