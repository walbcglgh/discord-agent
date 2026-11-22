package vps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/llm"
	"discord-agent/internal/tools"
	"discord-agent/internal/util"

	"golang.org/x/crypto/ssh"
)

var blocked = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-rf\b`),
	regexp.MustCompile(`(?i)\bmkfs\b`),
	regexp.MustCompile(`(?i)\bdd\s+if=`),
	regexp.MustCompile(`(?i)\b(shutdown|reboot|halt|poweroff)\b`),
	regexp.MustCompile(`(?i)\b(ifconfig\.me|ipify|icanhazip|checkip|myip|whatismyip)\b`),
	regexp.MustCompile(`(?i)\b(hostname\s+-I|ip\s+addr|ifconfig)\b`),
	regexp.MustCompile(`(?i)\b(curl|wget|ssh-keygen|sudo|su|visudo|usermod|useradd|passwd)\b`),
	regexp.MustCompile(`(?i)\bcat\s+[^\n]*(\.pem|id_rsa|id_ed25519)`),
}

var ipRe = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
var pemRe = regexp.MustCompile(`(?s)-----BEGIN [^-]+-----.*?-----END [^-]+-----`)

// Register exposes exactly one opaque capability to the model.
func Register(reg *tools.Registry, store *config.Store, logger *log.Logger) {
	reg.AddTool(llm.ToolDef{
		Name: "run_vps_command",
		Description: "一台你可以用來運行指令的 VPS，已給予 root 權限，但每次只能運行 12 小時；" +
			"若連不上，表示可能需要喚醒，請告訴使用者。這是唯一的操作方式。" +
			"禁止提及 IP、網域名稱、連接埠、SSH、金鑰、.pem 檔案或任何伺服器識別資訊；" +
			"禁止查詢對外 IP、給予完整伺服器管理員權限，或暗示任何連線細節。" +
			"也請避開危險指令。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要執行的單一指令"},
			},
			"required": []string{"command"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		cfg := store.Get().VPS
		if !cfg.Enabled {
			return "", fmt.Errorf("目前沒有可用的 VPS")
		}
		var in struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return "", err
		}
		command := strings.TrimSpace(in.Command)
		if command == "" {
			return "", fmt.Errorf("command 不可為空")
		}
		for _, re := range blocked {
			if re.MatchString(command) {
				return "這個指令不允許執行。", nil
			}
		}
		out, err := exec(ctx, cfg, command)
		if err != nil && cfg.WakeURL != "" {
			wake(ctx, cfg.WakeURL)
			out, err = exec(ctx, cfg, command)
		}
		if err != nil {
			logger.Printf("VPS 連線失敗: %s", util.Redact(err.Error(), secrets(cfg)))
			return "VPS 暫時無法使用，可能需要喚醒；請通知管理員。", nil
		}
		return redact(out, cfg), nil
	})
}

func secrets(cfg config.VPSConfig) []string {
	return []string{
		cfg.Host, cfg.User, strconv.Itoa(cfg.Port),
		config.Secret(cfg.PasswordEnv, cfg.Password),
		config.Secret(cfg.KeyEnv, cfg.Key), cfg.Passphrase, cfg.WakeURL,
	}
}

func redact(s string, cfg config.VPSConfig) string {
	for _, v := range secrets(cfg) {
		if len(v) >= 3 {
			s = strings.ReplaceAll(s, v, "[已隱藏]")
		}
	}
	s = ipRe.ReplaceAllString(s, "[IP 已隱藏]")
	s = pemRe.ReplaceAllString(s, "[金鑰已隱藏]")
	s = strings.ReplaceAll(s, "ssh-rsa", "[金鑰已隱藏]")
	s = strings.ReplaceAll(s, "ssh-ed25519", "[金鑰已隱藏]")
	return util.Redact(s, secrets(cfg))
}

func authMethods(cfg config.VPSConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if cfg.AuthType == "password" {
		pw := config.Secret(cfg.PasswordEnv, cfg.Password)
		if pw == "" {
			return nil, fmt.Errorf("缺少 VPS 密碼")
		}
		methods = append(methods, ssh.Password(pw))
		return methods, nil
	}
	key := config.Secret(cfg.KeyEnv, cfg.Key)
	if key == "" {
		return nil, fmt.Errorf("缺少 VPS SSH key")
	}
	var signer ssh.Signer
	var err error
	if cfg.Passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(cfg.Passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(key))
	}
	if err != nil {
		return nil, err
	}
	return append(methods, ssh.PublicKeys(signer)), nil
}

func exec(ctx context.Context, cfg config.VPSConfig, command string) (string, error) {
	methods, err := authMethods(cfg)
	if err != nil {
		return "", err
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), &ssh.ClientConfig{
		User: cfg.User, Auth: methods, Timeout: 15 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return "", err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Start(command); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	select {
	case err := <-done:
		out := stdout.String()
		if stderr.Len() > 0 {
			out += "\n" + stderr.String()
		}
		if err != nil && strings.TrimSpace(out) == "" {
			return "", err
		}
		return out, nil
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGINT)
		return "", ctx.Err()
	case <-time.After(30 * time.Second):
		_ = session.Signal(ssh.SIGINT)
		return "", fmt.Errorf("指令逾時")
	}
}

func wake(ctx context.Context, url string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
