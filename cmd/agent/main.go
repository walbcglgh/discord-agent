package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/discord"
	"discord-agent/internal/mcp"
	"discord-agent/internal/media"
	"discord-agent/internal/memory"
	"discord-agent/internal/panel"
	"discord-agent/internal/tools"
	"discord-agent/internal/util"
	"discord-agent/internal/vps"

	"golang.org/x/crypto/bcrypt"
)

const version = "1.0.0"

func main() {
	configPath := flag.String("config", "config.json", "設定檔路徑")
	envPath := flag.String("env", ".env", "環境變數檔案路徑")
	initOnly := flag.Bool("init", false, "只建立預設設定檔後結束")
	resetPW := flag.Bool("reset-panel-password", false, "重設面板密碼後結束")
	panelOnly := flag.Bool("panel", false, "只啟動一次性管理面板，Ctrl+C 結束")
	showVersion := flag.Bool("version", false, "顯示版本")
	flag.Parse()

	if *showVersion {
		fmt.Println("Selana Discord Agent", version)
		return
	}
	if err := config.LoadEnvFile(*envPath); err != nil {
		fmt.Fprintln(os.Stderr, "載入 .env 失敗:", err)
		os.Exit(1)
	}

	store := config.NewStore(*configPath)
	if err := store.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "載入設定失敗:", err)
		os.Exit(1)
	}

	if *initOnly {
		fmt.Println("已建立預設設定檔:", store.Path())
		return
	}
	if *resetPW {
		resetPassword(store)
		return
	}

	buf := util.NewLogBuffer(500)
	logger := log.New(io.MultiWriter(os.Stdout, buf), "[agent] ", log.LstdFlags)
	logger.Printf("Selana Discord Agent v%s 啟動中", version)

	cfg := store.Get()
	if err := ensurePanelPassword(store, cfg, logger); err != nil {
		logger.Fatalf("面板密碼設定失敗: %v", err)
	}
	cfg = store.Get()

	mem, err := memory.Open(cfg.Database.Path, cfg.Memory.MaxFacts)
	if err != nil {
		logger.Fatalf("開啟資料庫失敗: %v", err)
	}
	defer mem.Close()

	toolReg := tools.New(cfg.Tools, mem)
	mcpMgr := mcp.New(store, toolReg, logger)
	mcpMgr.Refresh(context.Background())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	if *panelOnly {
		web := panel.New(store, mem, nil, mcpMgr, buf, logger)
		handoff := make(chan struct{})
		web.SetHandoff(handoff)
		if err := web.Start(); err != nil {
			logger.Fatalf("啟動面板失敗: %v", err)
		}
		logger.Printf("面板已啟動：http://%s ，按 Ctrl+C 結束，或按「完成並啟動 Bot」交接", cfg.Panel.Listen)
		select {
		case <-stop:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = web.Shutdown(ctx)
			logger.Println("面板已關閉")
			return
		case <-handoff:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = web.Shutdown(ctx)
			cancel()
			logger.Println("面板已關閉，正在啟動 Bot…")
		}
		cfg = store.Get()
		mcpMgr.Refresh(context.Background())
	}

	token := config.Secret(cfg.Discord.TokenEnv, cfg.Discord.Token)
	if token == "" {
		logger.Fatalf("未設定 %s，請在 .env 或 systemd EnvironmentFile 中提供 Discord Bot Token", cfg.Discord.TokenEnv)
	}
	bot, err := discord.New(token, store, mem, toolReg, logger)
	if err != nil {
		logger.Fatalf("建立 Discord 工作階段失敗: %v", err)
	}
	discord.RegisterContextTool(toolReg, bot.Session())
	discord.RegisterModerationTools(toolReg, bot)
	discord.RegisterUITools(toolReg, bot)
	discord.RegisterEmojiTools(toolReg, bot)
	vps.Register(toolReg, store, logger)
	media.RegisterTTSTool(toolReg, func() config.MediaConfig { return store.Get().Media })

	if err := bot.Start(); err != nil {
		logger.Fatalf("Discord 連線失敗: %v", err)
	}
	logger.Printf("已連線，可用模型數量：%d", len(cfg.LLM.Models))
	done := make(chan struct{})
	go watchConfig(store, mcpMgr, logger, done)

	<-stop
	close(done)
	logger.Println("收到停止訊號，正在關閉…")
	_ = bot.Stop()
	logger.Println("已安全關閉")
}

// watchConfig lets a separately started one-shot panel hot-update the bot.
func watchConfig(store *config.Store, mgr *mcp.Manager, logger *log.Logger, done <-chan struct{}) {
	last, _ := os.Stat(store.Path())
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			info, err := os.Stat(store.Path())
			if err != nil || last != nil && info.ModTime().Equal(last.ModTime()) {
				continue
			}
			last = info
			if err := store.Reload(); err != nil {
				logger.Printf("熱重載設定失敗: %v", err)
				continue
			}
			mgr.Refresh(context.Background())
			logger.Printf("設定已熱重載")
		}
	}
}

func ensurePanelPassword(store *config.Store, cfg *config.Config, logger *log.Logger) error {
	if cfg.Panel.PasswordHash != "" {
		return nil
	}
	pw := os.Getenv("PANEL_PASSWORD")
	generated := false
	if pw == "" {
		pw = randomString(16)
		generated = true
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	next, err := cfg.Clone()
	if err != nil {
		return err
	}
	next.Panel.PasswordHash = string(hash)
	if err := store.Replace(next); err != nil {
		return err
	}
	if generated {
		logger.Printf("已產生面板初始密碼：%s（請盡快登入並透過 PANEL_PASSWORD 環境變數管理）", pw)
	} else {
		logger.Printf("已從 PANEL_PASSWORD 設定面板密碼")
	}
	return nil
}

func resetPassword(store *config.Store) {
	cfg := store.Get()
	pw := os.Getenv("PANEL_PASSWORD")
	generated := false
	if pw == "" {
		pw = randomString(16)
		generated = true
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "雜湊失敗:", err)
		os.Exit(1)
	}
	next, err := cfg.Clone()
	if err != nil {
		fmt.Fprintln(os.Stderr, "複製設定失敗:", err)
		os.Exit(1)
	}
	next.Panel.PasswordHash = string(hash)
	if err := store.Replace(next); err != nil {
		fmt.Fprintln(os.Stderr, "寫入失敗:", err)
		os.Exit(1)
	}
	fmt.Println("面板密碼已重設。")
	if generated {
		fmt.Println("新密碼：", pw)
	}
}

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)[:n]
}
