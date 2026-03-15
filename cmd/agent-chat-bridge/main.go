package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/bot"
	"github.com/vanadis-ai/agent-chat-bridge/internal/claude"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
)

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to config file")
	pidFile := flag.String("pidfile", "", "write PID to this file")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	if *debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	if *pidFile != "" {
		if err := os.WriteFile(*pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
			slog.Error("failed to write pidfile", "error", err)
			os.Exit(1)
		}
		defer os.Remove(*pidFile)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bots := startBots(ctx, cfg)
	waitForShutdown(cancel, bots)
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err == nil {
		xdg := filepath.Join(home, ".config", "agent-chat-bridge", "config.yaml")
		if _, err := os.Stat(xdg); err == nil {
			return xdg
		}
	}
	return "configs/config.yaml"
}

func startBots(ctx context.Context, cfg *config.Config) []*bot.Bot {
	var bots []*bot.Bot

	for name, botCfg := range cfg.TelegramBots {
		api, err := tgbotapi.NewBotAPI(botCfg.Token)
		if err != nil {
			slog.Error("failed to create bot API", "bot", name, "error", err)
			continue
		}
		slog.Info("bot authenticated", "name", name, "username", api.Self.UserName)

		sessions := claude.NewSessionStore(botCfg.Sessions)
		b := bot.NewBot(name, botCfg, cfg.Claude, api, sessions)
		bots = append(bots, b)
		go b.Start(ctx)
	}
	return bots
}

func waitForShutdown(cancel context.CancelFunc, bots []*bot.Bot) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	slog.Info("received signal, shutting down", "signal", sig)
	cancel()

	var wg sync.WaitGroup
	for _, b := range bots {
		wg.Add(1)
		go func(b *bot.Bot) {
			defer wg.Done()
			b.Stop()
		}(b)
	}
	wg.Wait()
	slog.Info("shutdown complete")
}
