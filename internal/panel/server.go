package panel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

type session struct {
	expires time.Time
	csrf    string
}

type loginFail struct {
	count  int
	locked time.Time
}

type Server struct {
	store  *config.Store
	mem    *memory.Store
	bot    *discord.Bot
	mcpMgr *mcp.Manager
	logs   *util.LogBuffer
	logger *log.Logger
	srv    *http.Server

	mu        sync.Mutex
	sessions  map[string]session
	loginFail map[string]loginFail
}

func New(store *config.Store, mem *memory.Store, bot *discord.Bot, mgr *mcp.Manager, logs *util.LogBuffer, logger *log.Logger) *Server {
	return &Server{
		store: store, mem: mem, bot: bot, mcpMgr: mgr, logs: logs, logger: logger,
		sessions: make(map[string]session), loginFail: make(map[string]loginFail),
	}
}

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
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.withAuth(s.handleLogout))
	mux.HandleFunc("/", s.withAuth(s.handleOverview))
	mux.HandleFunc("/settings", s.withAuth(s.handleSettings))
	mux.HandleFunc("/models", s.withAuth(s.handleModels))
	mux.HandleFunc("/models/add", s.withAuth(s.handleModelAdd))
	mux.HandleFunc("/models/discover", s.withAuth(s.handleModelDiscover))
	mux.HandleFunc("/models/bulk", s.withAuth(s.handleModelBulk))
	mux.HandleFunc("/models/delete", s.withAuth(s.handleModelDelete))
	mux.HandleFunc("/mcp", s.withAuth(s.handleMCP))
	mux.HandleFunc("/mcp/add", s.withAuth(s.handleMCPAdd))
	mux.HandleFunc("/mcp/delete", s.withAuth(s.handleMCPDelete))
	mux.HandleFunc("/mcp/toggle", s.withAuth(s.handleMCPToggle))
	mux.HandleFunc("/vps", s.withAuth(s.handleVPS))
	mux.HandleFunc("/vps/save", s.withAuth(s.handleVPSSave))
	mux.HandleFunc("/media", s.withAuth(s.handleMedia))
	mux.HandleFunc("/media/save", s.withAuth(s.handleMediaSave))
	mux.HandleFunc("/commands", s.withAuth(s.handleCommands))
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
			SameSite: http.SameSiteLaxMode, MaxAge: 600,
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
			SameSite: http.SameSiteLaxMode, MaxAge: 600,
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
		SameSite: http.SameSiteLaxMode, MaxAge: cfg.Panel.SessionHours * 3600,
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
	http.SetCookie(w, &http.Cookie{Name: "agent_session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	cfg := s.store.Get()
	env := map[string]bool{}
	names := []string{cfg.Discord.TokenEnv, "PANEL_PASSWORD"}
	for _, m := range cfg.LLM.Models {
		if m.APIKeyEnv != "" {
			names = append(names, m.APIKeyEnv)
		}
	}
	for _, s := range cfg.MCP.Servers {
		for _, n := range []string{s.APIKeyEnv, s.OAuthTokenEnv} {
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
	status := s.status()
	invite := ""
	if id, ok := status["bot_id"].(string); ok && id != "" {
		invite = "https://discord.com/oauth2/authorize?client_id=" + id +
			"&permissions=2201170807942&scope=" + url.QueryEscape("bot applications.commands")
	}
	data := struct {
		Status    map[string]any
		Config    *config.Config
		Env       map[string]bool
		InviteURL string
		CSRF      string
		Message   string
	}{
		Status: status, Config: cfg, Env: env, InviteURL: invite, CSRF: sess.csrf,
		Message: r.URL.Query().Get("msg"),
	}
	s.render(w, "overview", data)
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
	data := struct {
		JSON               string
		CSRF               string
		AssistantName      string
		UnavailableMessage string
		Display            config.DisplayConfig
		Message            string
		Error              string
	}{
		JSON: string(b), CSRF: sess.csrf,
		AssistantName: cfg.AssistantName, UnavailableMessage: cfg.UnavailableMessage,
		Display: cfg.Display, Message: msg, Error: errMsg,
	}
	s.render(w, "settings", data)
}

type modelsData struct {
	Models       []config.ModelConfig
	CSRF         string
	Message      string
	Error        string
	Discovered   []string
	DiscProtocol string
	DiscBaseURL  string
	DiscKeyEnv   string
	DiscKey      string
}

func (s *Server) renderModels(w http.ResponseWriter, r *http.Request, d modelsData) {
	sess, _ := s.sessionOf(r)
	d.Models = s.store.Get().LLM.Models
	d.CSRF = sess.csrf
	if d.Message == "" {
		d.Message = r.URL.Query().Get("msg")
	}
	if d.Error == "" {
		d.Error = r.URL.Query().Get("err")
	}
	s.render(w, "models", d)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	s.renderModels(w, r, modelsData{})
}

func modelFromForm(r *http.Request) (config.ModelConfig, error) {
	maxTokens, _ := strconv.Atoi(r.FormValue("max_tokens"))
	temp, _ := strconv.ParseFloat(r.FormValue("temperature"), 64)
	m := config.ModelConfig{
		ID: r.FormValue("id"), Name: strings.TrimSpace(r.FormValue("name")),
		Protocol: r.FormValue("protocol"), BaseURL: strings.TrimSpace(r.FormValue("base_url")),
		APIKeyEnv: strings.TrimSpace(r.FormValue("api_key_env")), APIKey: strings.TrimSpace(r.FormValue("api_key")),
		ModelID: strings.TrimSpace(r.FormValue("model_id")), MaxTokens: maxTokens,
		Temperature: temp, Enabled: r.FormValue("enabled") == "on",
	}
	if m.Name == "" || m.ModelID == "" {
		return m, fmt.Errorf("name 與 model_id 不可為空")
	}
	return m, nil
}

func (s *Server) handleModelAdd(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	m, err := modelFromForm(r)
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	m.ID = config.NewID("m_")
	next.LLM.Models = append(next.LLM.Models, m)
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/models?msg="+url.QueryEscape("已新增模型 "+m.Name), http.StatusSeeOther)
}

func (s *Server) handleModelDiscover(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	base := strings.TrimSpace(r.FormValue("disc_base_url"))
	keyEnv := strings.TrimSpace(r.FormValue("disc_key_env"))
	key := strings.TrimSpace(r.FormValue("disc_key"))
	protocol := r.FormValue("disc_protocol")
	ids, err := llm.DiscoverModels(r.Context(), base, config.Secret(keyEnv, key))
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	s.renderModels(w, r, modelsData{
		Discovered: ids, DiscProtocol: protocol, DiscBaseURL: base,
		DiscKeyEnv: keyEnv, DiscKey: key,
	})
}

func (s *Server) handleModelBulk(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	if r.Method != http.MethodPost || r.FormValue("csrf") != sess.csrf {
		http.Error(w, "請求無效", http.StatusForbidden)
		return
	}
	count, _ := strconv.Atoi(r.FormValue("count"))
	next, err := s.store.Get().Clone()
	if err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	added := 0
	for i := 0; i < count; i++ {
		if r.FormValue(fmt.Sprintf("use_%d", i)) != "on" {
			continue
		}
		id := strings.TrimSpace(r.FormValue(fmt.Sprintf("id_%d", i)))
		name := strings.TrimSpace(r.FormValue(fmt.Sprintf("name_%d", i)))
		if id == "" || name == "" {
			continue
		}
		next.LLM.Models = append(next.LLM.Models, config.ModelConfig{
			ID: config.NewID("m_"), Name: name, Protocol: r.FormValue("protocol"),
			BaseURL: r.FormValue("base_url"), APIKeyEnv: r.FormValue("key_env"),
			APIKey: r.FormValue("api_key"), ModelID: id,
			MaxTokens: 1024, Temperature: 0.7, Enabled: true,
		})
		added++
	}
	if added == 0 {
		http.Redirect(w, r, "/models?err="+url.QueryEscape("沒有勾選任何模型"), http.StatusSeeOther)
		return
	}
	if err := s.store.Replace(next); err != nil {
		http.Redirect(w, r, "/models?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/models?msg="+url.QueryEscape(fmt.Sprintf("已新增 %d 個模型", added)), http.StatusSeeOther)
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

type mcpData struct {
	Servers []config.MCPServerConfig
	CSRF    string
	Message string
	Error   string
}

func (s *Server) renderMCP(w http.ResponseWriter, r *http.Request, d mcpData) {
	sess, _ := s.sessionOf(r)
	d.Servers = s.store.Get().MCP.Servers
	d.CSRF = sess.csrf
	if d.Message == "" {
		d.Message = r.URL.Query().Get("msg")
	}
	if d.Error == "" {
		d.Error = r.URL.Query().Get("err")
	}
	s.render(w, "mcp", d)
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	s.renderMCP(w, r, mcpData{})
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
		OAuthTokenURL:     strings.TrimSpace(r.FormValue("oauth_token_url")),
		OAuthClientID:     strings.TrimSpace(r.FormValue("oauth_client_id")),
		OAuthClientSecret: strings.TrimSpace(r.FormValue("oauth_client_secret")),
		OAuthScopes:       scopes, Enabled: true,
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
	csrf, msg, errMsg := s.simpleData(r)
	cfg := s.store.Get()
	s.render(w, "discord", struct {
		Discord config.DiscordConfig
		CSRF    string
		Message string
		Error   string
	}{cfg.Discord, csrf, msg, errMsg})
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

func (s *Server) simpleData(r *http.Request) (string, string, string) {
	sess, _ := s.sessionOf(r)
	return sess.csrf, r.URL.Query().Get("msg"), r.URL.Query().Get("err")
}

func (s *Server) handleVPS(w http.ResponseWriter, r *http.Request) {
	csrf, msg, errMsg := s.simpleData(r)
	cfg := s.store.Get()
	s.render(w, "vps", struct {
		VPS     config.VPSConfig
		CSRF    string
		Message string
		Error   string
	}{cfg.VPS, csrf, msg, errMsg})
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
	csrf, msg, errMsg := s.simpleData(r)
	cfg := s.store.Get()
	s.render(w, "media", struct {
		Media   config.MediaConfig
		CSRF    string
		Message string
		Error   string
	}{cfg.Media, csrf, msg, errMsg})
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
	sess, _ := s.sessionOf(r)
	lines := s.logs.Lines(200)
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	data := struct {
		Lines []string
		CSRF  string
	}{Lines: lines, CSRF: sess.csrf}
	s.render(w, "logs", data)
}

func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionOf(r)
	entries, err := s.mem.Errors(50)
	if err != nil {
		s.logger.Printf("讀取錯誤紀錄失敗: %v", err)
	}
	data := struct {
		Entries []memory.ErrorEntry
		CSRF    string
	}{Entries: entries, CSRF: sess.csrf}
	s.render(w, "errors", data)
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
