package panel

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/discord"
	"discord-agent/internal/llm"
	"discord-agent/internal/mcp"
	"discord-agent/internal/memory"
	"discord-agent/internal/util"

	"golang.org/x/crypto/bcrypt"
)

//go:embed tailwind.css
var tailwindCSS []byte

type session struct {
	expires time.Time
	csrf    string
}

type loginFail struct {
	count  int
	locked time.Time
}

type oauthState struct {
	serverID    string
	verifier    string
	redirectURI string
	expires     time.Time
}

// pageData backs the single console template. Active selects which settings
// dialog is opened automatically after a redirect.
type pageData struct {
	Active             string
	CSRF               string
	Message            string
	Error              string
	Status             map[string]any
	Config             *config.Config
	Env                map[string]bool
	InviteURL          string
	JSON               string
	AssistantName      string
	UnavailableMessage string
	Display            config.DisplayConfig
	Models             []config.ModelConfig
	Providers          []config.ProviderConfig
	ProviderCounts     map[string]int
	Servers            []config.MCPServerConfig
	Discord            config.DiscordConfig
	VPS                config.VPSConfig
	Media              config.MediaConfig
	Lines              []string
	Entries            []memory.ErrorEntry
}

type Server struct {
	store  *config.Store
	mem    *memory.Store
	bot    *discord.Bot
	mcpMgr *mcp.Manager
	logs   *util.LogBuffer
	logger *log.Logger
	srv    *http.Server

	mu          sync.Mutex
	sessions    map[string]session
	loginFail   map[string]loginFail
	oauthMu     sync.Mutex
	oauthStates map[string]oauthState
	handoff     chan struct{}
	handoffOnce sync.Once
}

func New(store *config.Store, mem *memory.Store, bot *discord.Bot, mgr *mcp.Manager, logs *util.LogBuffer, logger *log.Logger) *Server {
	return &Server{
		store: store, mem: mem, bot: bot, mcpMgr: mgr, logs: logs, logger: logger,
		sessions: make(map[string]session), loginFail: make(map[string]loginFail),
		oauthStates: make(map[string]oauthState),
	}
}

// SetHandoff makes the "finish" button signal the caller to start the bot.
func (s *Server) SetHandoff(ch chan struct{}) { s.handoff = ch }

func (s *Server) Start() error {
	cfg := s.store.Get()
	ln, err := net.Listen("tcp", cfg.Panel.Listen)
	if err != nil {
		return fmt.Errorf("啟動管理面板失敗: %w", err)
	}
	s.srv = &http.Server{Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("管理面板停止: %v", err)
		}
	}()
	s.logger.Printf("管理面板已啟動：http://%s", cfg.Panel.Listen)
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/assets/tailwind.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(tailwindCSS)
	})
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.withAuth(s.handleLogout))
	mux.HandleFunc("/", s.withAuth(s.handleOverview))
	mux.HandleFunc("/settings", s.withAuth(s.handleSettings))
	mux.HandleFunc("/models", s.withAuth(s.handleModels))
	mux.HandleFunc("/models/add", s.withAuth(s.handleModelAdd))
	mux.HandleFunc("/models/delete", s.withAuth(s.handleModelDelete))
	mux.HandleFunc("/models/list", s.withAuth(s.handleModelList))
	mux.HandleFunc("/providers/add", s.withAuth(s.handleProviderAdd))
	mux.HandleFunc("/providers/update", s.withAuth(s.handleProviderUpdate))
	mux.HandleFunc("/providers/delete", s.withAuth(s.handleProviderDelete))
	mux.HandleFunc("/mcp", s.withAuth(s.handleMCP))
	mux.HandleFunc("/mcp/add", s.withAuth(s.handleMCPAdd))
	mux.HandleFunc("/mcp/delete", s.withAuth(s.handleMCPDelete))
	mux.HandleFunc("/mcp/toggle", s.withAuth(s.handleMCPToggle))
	mux.HandleFunc("/mcp/oauth/start", s.withAuth(s.handleMCPOAuthStart))
	mux.HandleFunc("/mcp/oauth/callback", s.handleMCPOAuthCallback)
	mux.HandleFunc("/vps", s.withAuth(s.handleVPS))
	mux.HandleFunc("/vps/save", s.withAuth(s.handleVPSSave))
	mux.HandleFunc("/media", s.withAuth(s.handleMedia))
	mux.HandleFunc("/media/save", s.withAuth(s.handleMediaSave))
	mux.HandleFunc("/commands", s.withAuth(s.handleCommands))
	mux.HandleFunc("/finish", s.withAuth(s.handleFinish))
	mux.HandleFunc("/logs", s.withAuth(s.handleLogs))
	mux.HandleFunc("/errors", s.withAuth(s.handleErrors))
	mux.HandleFunc("/appearance", s.withAuth(s.handleAppearance))
	mux.HandleFunc("/discord", s.withAuth(s.handleDiscord))
	mux.HandleFunc("/discord/save", s.withAuth(s.handleDiscordSave))
	return mux
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func requestScheme(r *http.Request) string {
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		return strings.TrimSpace(strings.Split(p, ",")[0])
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		return strings.TrimSpace(strings.Split(h, ",")[0])
	}
	return r.Host
}

func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) ipAllowed(r *http.Request) bool {
	list := s.store.Get().Panel.AllowedIPs
	if len(list) == 0 {
		return true
	}
	ip := net.ParseIP(s.clientIP(r))
	if ip == nil {
		return false
	}
	for _, entry := range list {
		entry = strings.TrimSpace(entry)
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err == nil && network.Contains(ip) {
				return true
			}
		} else if net.ParseIP(entry) != nil && net.ParseIP(entry).Equal(ip) {
			return true
		}
	}
	return false
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.ipAllowed(r) {
			http.Error(w, "IP 不在允許清單中", http.StatusForbidden)
			return
		}
		c, err := r.Cookie("agent_session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		s.mu.Lock()
		sess, ok := s.sessions[c.Value]
		if ok && time.Now().After(sess.expires) {
			delete(s.sessions, c.Value)
			ok = false
		}
		s.mu.Unlock()
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *Server) sessionOf(r *http.Request) (session, bool) {
	c, err := r.Cookie("agent_session")
	if err != nil {
		return session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[c.Value]
	return sess, ok
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.ipAllowed(r) {
		http.Error(w, "IP 不在允許清單中", http.StatusForbidden)
		return
	}
	if _, ok := s.sessionOf(r); ok && r.Method == http.MethodGet {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cfg := s.store.Get()
	secure := requestScheme(r) == "https"
	csrfCookie, csrfErr := r.Cookie("login_csrf")
	csrf := ""
	if csrfErr == nil && csrfCookie.Value != "" {
		csrf = csrfCookie.Value
	} else {
		csrf = randomHex(16)
	}
	data := struct {
		Error string
		CSRF  string
	}{CSRF: csrf}
	if r.Method != http.MethodPost {
		http.SetCookie(w, &http.Cookie{
			Name: "login_csrf", Value: csrf, Path: "/", HttpOnly: true,
			SameSite: http.SameSiteLaxMode, Secure: secure, MaxAge: 600,
		})
		s.render(w, "login", data)
		return
	}
	ip := s.clientIP(r)
	s.mu.Lock()
	fail := s.loginFail[ip]
	if !fail.locked.IsZero() && time.Now().Before(fail.locked) {
		s.mu.Unlock()
		data.Error = "登入失敗次數過多，請稍後再試。"
		s.render(w, "login", data)
		return
	}
	s.mu.Unlock()

	if csrfErr != nil || r.FormValue("csrf") != csrf {
		http.SetCookie(w, &http.Cookie{
			Name: "login_csrf", Value: csrf, Path: "/", HttpOnly: true,
			SameSite: http.SameSiteLaxMode, Secure: secure, MaxAge: 600,
		})
		data.Error = "表單驗證失敗，請重試。"
		s.render(w, "login", data)
		return
	}
	password := r.FormValue("password")
	hash := []byte(cfg.Panel.PasswordHash)
	ok := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	if !ok || r.FormValue("username") != cfg.Panel.Username {
		s.mu.Lock()
		fail.count++
		if fail.count >= 5 {
			fail.locked = time.Now().Add(10 * time.Minute)
			fail.count = 0
		}
		s.loginFail[ip] = fail
		s.mu.Unlock()
		data.Error = "帳號或密碼錯誤。"
		s.render(w, "login", data)
		return
	}
	s.mu.Lock()
	delete(s.loginFail, ip)
	token := randomHex(32)
	csrfSess := randomHex(16)
	s.sessions[token] = session{
		expires: time.Now().Add(time.Duration(cfg.Panel.SessionHours) * time.Hour),
		csrf:    csrfSess,
	}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: "agent_session", Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: secure, MaxAge: cfg.Panel.SessionHours * 3600,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if c, err := r.Cookie("agent_session"); err == nil && r.FormValue("csrf") == sess.csrf {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "agent_session", Value: "", Path: "/", MaxAge: -1, Secure: requestScheme(r) == "https"})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	s.render(w, "console", s.page(r, ""))
}

// page builds the shared console data for a request. active is the dialog id
// that should reopen after a form POST redirect.
func (s *Server) page(r *http.Request, active string) pageData {
	sess, _ := s.sessionOf(r)
	cfg := s.store.Get()
	d := pageData{
		Active: active, CSRF: sess.csrf,
		Message: r.URL.Query().Get("msg"), Error: r.URL.Query().Get("err"),
		Status: s.status(), Config: cfg,
		Discord: cfg.Discord, VPS: cfg.VPS, Media: cfg.Media,
		Models: cfg.LLM.Models, Providers: cfg.LLM.Providers, Servers: cfg.MCP.Servers,
		AssistantName: cfg.AssistantName, UnavailableMessage: cfg.UnavailableMessage,
		Display: cfg.Display,
	}
	d.ProviderCounts = map[string]int{}
	for _, m := range d.Models {
		d.ProviderCounts[m.ProviderID]++
	}
	env := map[string]bool{}
	names := []string{cfg.Discord.TokenEnv, "PANEL_PASSWORD"}
	for _, m := range cfg.LLM.Models {
		if m.APIKeyEnv != "" {
			names = append(names, m.APIKeyEnv)
		}
	}
	for _, srv := range cfg.MCP.Servers {
		for _, n := range []string{srv.APIKeyEnv, srv.OAuthTokenEnv} {
			if n != "" {
				names = append(names, n)
			}
		}
	}
	if cfg.Tools.WebSearch.APIKeyEnv != "" {
		names = append(names, cfg.Tools.WebSearch.APIKeyEnv)
	}
	for _, n := range names {
		if n == "" {
			continue
		}
		env[n] = os.Getenv(n) != "" || apiKeyInline(cfg, n) != ""
	}
	d.Env = env
	if id, ok := d.Status["bot_id"].(string); ok && id != "" {
		d.InviteURL = "https://discord.com/oauth2/authorize?client_id=" + id +
			"&permissions=2201170807942&scope=" + url.QueryEscape("bot applications.commands")
	}
	return d
}

func apiKeyInline(cfg *config.Config, envName string) string {
	for _, m := range cfg.LLM.Models {
		if m.APIKeyEnv == envName && m.APIKey != "" {
			return m.APIKey
		}
	}
	for _, s := range cfg.MCP.Servers {
		if s.APIKeyEnv == envName && s.APIKey != "" {
			return s.APIKey
		}
		if s.OAuthTokenEnv == envName && s.OAuthAccessToken != "" {
			return s.OAuthAccessToken
		}
	}
	if cfg.Tools.WebSearch.APIKeyEnv == envName {
		return cfg.Tools.WebSearch.APIKey
	}
	return ""
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	cfg := s.store.Get()
	msg, errMsg := r.URL.Query().Get("msg"), r.URL.Query().Get("err")
	if r.Method == http.MethodPost {
		if r.FormValue("csrf") != sess.csrf {
			http.Error(w, "CSRF 驗證失敗", http.StatusForbidden)
			return
		}
		var next config.Config
		if err := json.Unmarshal([]byte(r.FormValue("config")), &next); err != nil {
			errMsg = "JSON 解析失敗：" + err.Error()
		} else {
			if pw := os.Getenv("PANEL_PASSWORD"); pw != "" {
				hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
				if err != nil {
					errMsg = "密碼雜湊失敗：" + err.Error()
				} else {
					next.Panel.PasswordHash = string(hash)
				}
			}
			if errMsg == "" {
				if err := s.store.Replace(&next); err != nil {
					errMsg = "儲存失敗：" + err.Error()
				} else {
					msg = "設定已儲存並立即生效。"
					s.mcpMgr.Refresh(r.Context())
					s.logger.Printf("管理面板更新了設定")
				}
			}
		}
		cfg = s.store.Get()
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	d := s.page(r, "settings")
	d.JSON = string(b)
	d.Message, d.Error = msg, errMsg
	s.render(w, "console", d)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	s.render(w, "console", s.page(r, "models"))
}

func (s *Server) handleModelAdd(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	providerID := strings.TrimSpace(r.FormValue("provider_id"))
	name := strings.TrimSpace(r.FormValue("name"))
	modelID := strings.TrimSpace(r.FormValue("model_id"))
	if name == "" || modelID == "" || providerID == "" {
		http.Redirect(w, r, "/models?err="+url.QueryEscape("提供商、模型 ID 與模型映射都不可為空"), http.StatusSeeOther)
		return
	}
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	if next.LLM.ProviderByID(providerID) == nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape("找不到提供商"), http.StatusSeeOther)
		return
	}
	next.LLM.Models = append(next.LLM.Models, config.ModelConfig{
		ID: config.NewID("m_"), ProviderID: providerID,
		Name: name, ModelID: modelID, MaxTokens: 1024, Temperature: 0.7, Enabled: true,
	})
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/models?msg="+url.QueryEscape("已新增模型 "+name), http.StatusSeeOther)
}

// handleModelList returns upstream model ids for a provider. Failures return
// an empty list so the panel simply hides suggestions and allows manual input.
func (s *Server) handleModelList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	empty := []byte(`{"models":[]}`)
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write(empty)
		return
	}
	p := s.store.Get().LLM.ProviderByID(strings.TrimSpace(r.URL.Query().Get("provider_id")))
	if p == nil {
		_, _ = w.Write(empty)
		return
	}
	ids, err := llm.DiscoverModels(r.Context(), p.BaseURL, config.Secret(p.APIKeyEnv, p.APIKey))
	if err != nil {
		_, _ = w.Write(empty)
		return
	}
	if ids == nil {
		ids = []string{}
	}
	b, _ := json.Marshal(map[string]any{"models": ids})
	_, _ = w.Write(b)
}

func (s *Server) handleProviderAdd(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	id := strings.ToLower(strings.TrimSpace(r.FormValue("id")))
	name := strings.TrimSpace(r.FormValue("name"))
	base := strings.TrimSpace(r.FormValue("base_url"))
	protocol := r.FormValue("protocol")
	if !config.ValidProviderID(id) || name == "" || base == "" || !config.IsValidProtocol(protocol) {
		http.Redirect(w, r, "/models?err="+url.QueryEscape("提供商資料不合法"), http.StatusSeeOther)
		return
	}
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	if next.LLM.ProviderByID(id) != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape("提供商 ID 已存在"), http.StatusSeeOther)
		return
	}
	next.LLM.Providers = append(next.LLM.Providers, config.ProviderConfig{
		ID: id, Name: name, BaseURL: base, Protocol: protocol,
		APIKey: strings.TrimSpace(r.FormValue("api_key")),
	})
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/models?msg="+url.QueryEscape("已新增提供商 "+name), http.StatusSeeOther)
}

func (s *Server) handleProviderUpdate(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	p := next.LLM.ProviderByID(strings.TrimSpace(r.FormValue("id")))
	if p == nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape("找不到提供商"), http.StatusSeeOther)
		return
	}
	p.Name = strings.TrimSpace(r.FormValue("name"))
	p.BaseURL = strings.TrimSpace(r.FormValue("base_url"))
	p.Protocol = r.FormValue("protocol")
	if key := strings.TrimSpace(r.FormValue("api_key")); key != "" {
		p.APIKey = key
	}
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/models?msg="+url.QueryEscape("已更新提供商 "+p.Name), http.StatusSeeOther)
}

func (s *Server) handleProviderDelete(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	providers := next.LLM.Providers[:0]
	for _, p := range next.LLM.Providers {
		if p.ID != id {
			providers = append(providers, p)
		}
	}
	next.LLM.Providers = providers
	models := next.LLM.Models[:0]
	for _, m := range next.LLM.Models {
		if m.ProviderID != id {
			models = append(models, m)
		}
	}
	next.LLM.Models = models
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/models?msg="+url.QueryEscape("已刪除提供商與旗下模型"), http.StatusSeeOther)
}

func (s *Server) handleModelDelete(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	id := r.FormValue("id")
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	kept := next.LLM.Models[:0]
	for _, m := range next.LLM.Models {
		if m.ID != id {
			kept = append(kept, m)
		}
	}
	next.LLM.Models = kept
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/models?msg="+url.QueryEscape("已刪除模型"), http.StatusSeeOther)
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	s.render(w, "console", s.page(r, "mcp"))
}

func (s *Server) handleMCPAdd(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	scopes := []string{}
	for _, v := range strings.Split(r.FormValue("oauth_scopes"), ",") {
		if v = strings.TrimSpace(v); v != "" {
			scopes = append(scopes, v)
		}
	}
	srv := config.MCPServerConfig{
		ID: config.NewID("mcp_"), Name: strings.TrimSpace(r.FormValue("name")),
		URL: strings.TrimSpace(r.FormValue("url")), AuthType: r.FormValue("auth_type"),
		APIKeyEnv: strings.TrimSpace(r.FormValue("api_key_env")), APIKey: strings.TrimSpace(r.FormValue("api_key")),
		OAuthTokenEnv:     strings.TrimSpace(r.FormValue("oauth_token_env")),
		OAuthAccessToken:  strings.TrimSpace(r.FormValue("oauth_access_token")),
		OAuthRefreshToken: strings.TrimSpace(r.FormValue("oauth_refresh_token")),
		OAuthAuthURL:      strings.TrimSpace(r.FormValue("oauth_auth_url")),
		OAuthTokenURL:     strings.TrimSpace(r.FormValue("oauth_token_url")),
		OAuthClientID:     strings.TrimSpace(r.FormValue("oauth_client_id")),
		OAuthClientSecret: strings.TrimSpace(r.FormValue("oauth_client_secret")),
		OAuthScopes:       scopes,
		OAuthUsePKCE:      r.FormValue("oauth_use_pkce") == "on",
		Enabled:           true,
	}
	if srv.Name == "" || srv.URL == "" {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape("name 與 url 不可為空"), http.StatusSeeOther)
		return
	}
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	next.MCP.Servers = append(next.MCP.Servers, srv)
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	s.mcpMgr.Refresh(r.Context())
	http.Redirect(w, r, "/mcp?msg="+url.QueryEscape("已新增 MCP 伺服器"), http.StatusSeeOther)
}

func (s *Server) handleMCPDelete(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	id := r.FormValue("id")
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	kept := next.MCP.Servers[:0]
	for _, srv := range next.MCP.Servers {
		if srv.ID != id {
			kept = append(kept, srv)
		}
	}
	next.MCP.Servers = kept
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	s.mcpMgr.Refresh(r.Context())
	http.Redirect(w, r, "/mcp?msg="+url.QueryEscape("已刪除 MCP 伺服器"), http.StatusSeeOther)
}

func (s *Server) handleMCPToggle(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	id := r.FormValue("id")
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	for i := range next.MCP.Servers {
		if next.MCP.Servers[i].ID == id {
			next.MCP.Servers[i].Enabled = !next.MCP.Servers[i].Enabled
		}
	}
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	s.mcpMgr.Refresh(r.Context())
	http.Redirect(w, r, "/mcp?msg="+url.QueryEscape("已更新 MCP 狀態"), http.StatusSeeOther)
}

func splitList(v string) []string {
	out := []string{}
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) handleDiscord(w http.ResponseWriter, r *http.Request) {
	s.render(w, "console", s.page(r, "discord"))
}

func (s *Server) handleDiscordSave(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/discord?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	d := next.Discord
	d.TokenEnv = strings.TrimSpace(r.FormValue("token_env"))
	if token := strings.TrimSpace(r.FormValue("token")); token != "" {
		d.Token = token
	}
	d.ReplyMode = r.FormValue("reply_mode")
	d.AllowedChannelIDs = splitList(r.FormValue("allowed_channel_ids"))
	d.AdminUserIDs = splitList(r.FormValue("admin_user_ids"))
	d.AllowedGuildIDs = splitList(r.FormValue("allowed_guild_ids"))
	d.DMEnabled = r.FormValue("dm_enabled") == "on"
	d.TypingIndicator = r.FormValue("typing_indicator") == "on"
	if n, _ := strconv.Atoi(r.FormValue("rate_limit_seconds")); n > 0 {
		d.RateLimitSeconds = n
	}
	if n, _ := strconv.Atoi(r.FormValue("max_reply_chars")); n >= 500 && n <= 2000 {
		d.MaxReplyChars = n
	}
	next.Discord = d
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/discord?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/discord?msg="+url.QueryEscape("Discord 設定已更新"), http.StatusSeeOther)
}

func (s *Server) status() map[string]any {
	if s.bot == nil {
		return map[string]any{
			"uptime": "面板獨立模式", "provider": "-", "model": "-",
			"reply_mode": "-", "assistant": s.store.Get().AssistantName,
			"messages": 0, "bot_id": "", "started_at": "-",
		}
	}
	return s.bot.Status()
}

func (s *Server) handleVPS(w http.ResponseWriter, r *http.Request) {
	s.render(w, "console", s.page(r, "vps"))
}

func (s *Server) handleVPSSave(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/vps?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	port, _ := strconv.Atoi(r.FormValue("port"))
	if port == 0 {
		port = 22
	}
	user := strings.TrimSpace(r.FormValue("user"))
	if user == "" {
		user = "root"
	}
	v := config.VPSConfig{
		Enabled: r.FormValue("enabled") == "on",
		Host:    strings.TrimSpace(r.FormValue("host")), Port: port, User: user,
		AuthType:    r.FormValue("auth_type"),
		PasswordEnv: strings.TrimSpace(r.FormValue("password_env")),
		KeyEnv:      strings.TrimSpace(r.FormValue("key_env")),
		WakeURL:     strings.TrimSpace(r.FormValue("wake_url")),
	}
	if pw := strings.TrimSpace(r.FormValue("password")); pw != "" {
		v.Password = pw
	} else {
		v.Password = next.VPS.Password
	}
	if key := strings.TrimSpace(r.FormValue("key")); key != "" {
		v.Key = key
	} else {
		v.Key = next.VPS.Key
	}
	if pp := strings.TrimSpace(r.FormValue("passphrase")); pp != "" {
		v.Passphrase = pp
	} else {
		v.Passphrase = next.VPS.Passphrase
	}
	next.VPS = v
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/vps?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/vps?msg="+url.QueryEscape("VPS 設定已更新"), http.StatusSeeOther)
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	s.render(w, "console", s.page(r, "media"))
}

func (s *Server) handleMediaSave(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/media?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	m := config.MediaConfig{
		GroqAPIKeyEnv: strings.TrimSpace(r.FormValue("groq_api_key_env")),
		TTSVoice:      strings.TrimSpace(r.FormValue("tts_voice")),
		MaxTTSChars:   250, MaxAudioSeconds: 120,
	}
	if n, _ := strconv.Atoi(r.FormValue("max_tts_chars")); n > 0 {
		m.MaxTTSChars = n
	}
	if n, _ := strconv.Atoi(r.FormValue("max_audio_seconds")); n > 0 {
		m.MaxAudioSeconds = n
	}
	if key := strings.TrimSpace(r.FormValue("groq_api_key")); key != "" {
		m.GroqAPIKey = key
	} else {
		m.GroqAPIKey = next.Media.GroqAPIKey
	}
	next.Media = m
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/media?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/media?msg="+url.QueryEscape("多媒體設定已更新"), http.StatusSeeOther)
}

func (s *Server) handleMCPOAuthStart(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	id := r.FormValue("id")
	var srv config.MCPServerConfig
	found := false
	for _, c := range s.store.Get().MCP.Servers {
		if c.ID == id {
			srv, found = c, true
			break
		}
	}
	if !found || srv.AuthType != "oauth" {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape("找不到 OAuth 伺服器"), http.StatusSeeOther)
		return
	}
	if strings.TrimSpace(srv.OAuthAuthURL) == "" || strings.TrimSpace(srv.OAuthClientID) == "" || strings.TrimSpace(srv.OAuthTokenURL) == "" {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape("請先填寫 authorization URL、client ID 與 token URL"), http.StatusSeeOther)
		return
	}
	redirectURI := requestScheme(r) + "://" + requestHost(r) + "/mcp/oauth/callback"
	state := randomHex(24)
	verifier := randomHex(32)
	s.oauthMu.Lock()
	s.oauthStates[state] = oauthState{
		serverID: id, verifier: verifier, redirectURI: redirectURI,
		expires: time.Now().Add(10 * time.Minute),
	}
	s.oauthMu.Unlock()
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", srv.OAuthClientID)
	q.Set("redirect_uri", redirectURI)
	if len(srv.OAuthScopes) > 0 {
		q.Set("scope", strings.Join(srv.OAuthScopes, " "))
	}
	q.Set("state", state)
	if srv.OAuthUsePKCE {
		sum := sha256.Sum256([]byte(verifier))
		q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
		q.Set("code_challenge_method", "S256")
	}
	sep := "?"
	if strings.Contains(srv.OAuthAuthURL, "?") {
		sep = "&"
	}
	http.Redirect(w, r, srv.OAuthAuthURL+sep+q.Encode(), http.StatusFound)
}

func (s *Server) handleMCPOAuthCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape("OAuth 授權失敗: "+e), http.StatusSeeOther)
		return
	}
	state := q.Get("state")
	code := q.Get("code")
	s.oauthMu.Lock()
	pending, ok := s.oauthStates[state]
	if ok {
		delete(s.oauthStates, state)
	}
	s.oauthMu.Unlock()
	if !ok || code == "" || time.Now().After(pending.expires) {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape("OAuth state 無效或已過期"), http.StatusSeeOther)
		return
	}
	var srv config.MCPServerConfig
	found := false
	cfg := s.store.Get()
	for _, c := range cfg.MCP.Servers {
		if c.ID == pending.serverID {
			srv, found = c, true
			break
		}
	}
	if !found {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape("找不到 OAuth 伺服器"), http.StatusSeeOther)
		return
	}
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {pending.redirectURI},
		"client_id":    {srv.OAuthClientID},
	}
	if srv.OAuthClientSecret != "" {
		form.Set("client_secret", srv.OAuthClientSecret)
	}
	if srv.OAuthUsePKCE {
		form.Set("code_verifier", pending.verifier)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, srv.OAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 500000))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape("OAuth token 交換失敗: HTTP "+strconv.Itoa(resp.StatusCode)), http.StatusSeeOther)
		return
	}
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape("OAuth token 解析失敗"), http.StatusSeeOther)
		return
	}
	next, err := cfg.Clone()
	if err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	for i := range next.MCP.Servers {
		if next.MCP.Servers[i].ID == srv.ID {
			next.MCP.Servers[i].OAuthAccessToken = token.AccessToken
			if token.RefreshToken != "" {
				next.MCP.Servers[i].OAuthRefreshToken = token.RefreshToken
			}
		}
	}
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/mcp?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	s.mcpMgr.Refresh(r.Context())
	http.Redirect(w, r, "/mcp?msg="+url.QueryEscape("OAuth 授權完成"), http.StatusSeeOther)
}

func (s *Server) handleFinish(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `<!doctype html><html lang="zh-Hant"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>啟動中</title><link rel="stylesheet" href="/assets/tailwind.css"></head><body class="flex min-h-screen items-center justify-center bg-slate-100 px-5"><div class="max-w-sm rounded-2xl border border-slate-200 bg-white p-7 text-center shadow-sm"><div class="text-2xl">✅</div><h1 class="mt-3 text-lg font-semibold text-slate-900">設定完成</h1><p class="mt-2 text-sm text-slate-500">面板即將關閉，Bot 正在啟動。這個分頁可以直接關掉。</p></div></body></html>`)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	if s.handoff != nil {
		s.handoffOnce.Do(func() {
			go func() {
				time.Sleep(700 * time.Millisecond)
				close(s.handoff)
			}()
		})
	}
}

func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	if s.bot == nil {
		http.Redirect(w, r, "/settings?err="+url.QueryEscape("面板獨立模式無法註冊 Discord 指令"), http.StatusSeeOther)
		return
	}
	err := s.bot.ReregisterCommands()
	if err != nil {
		http.Redirect(w, r, "/settings?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings?msg="+url.QueryEscape("Slash 指令已重新註冊。"), http.StatusSeeOther)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	lines := s.logs.Lines(200)
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	d := s.page(r, "logs")
	d.Lines = lines
	s.render(w, "console", d)
}

func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	entries, err := s.mem.Errors(50)
	if err != nil {
		s.logger.Printf("讀取錯誤紀錄失敗: %v", err)
	}
	d := s.page(r, "errors")
	d.Entries = entries
	s.render(w, "console", d)
}

func (s *Server) handleAppearance(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	cfg := s.store.Get()
	next, err := cfg.Clone()
	if err != nil {
		http.Redirect(w, r, "/settings?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	next.AssistantName = strings.TrimSpace(r.FormValue("assistant_name"))
	next.UnavailableMessage = strings.TrimSpace(r.FormValue("unavailable_message"))
	next.Display = config.DisplayConfig{
		Thinking:  r.FormValue("display_thinking") == "on",
		ToolCalls: r.FormValue("display_tool_calls") == "on",
		RawOutput: r.FormValue("display_raw_output") == "on",
	}
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/settings?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings?msg="+url.QueryEscape("助理設定已更新。"), http.StatusSeeOther)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	if err := pages.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Printf("渲染頁面失敗: %v", err)
	}
}
