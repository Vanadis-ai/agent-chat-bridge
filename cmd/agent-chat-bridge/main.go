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
	claude "github.com/vanadis-ai/agent-chat-bridge/internal/backend/claude"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
	"github.com/vanadis-ai/agent-chat-bridge/internal/frontend/telegram"
	"github.com/vanadis-ai/agent-chat-bridge/internal/plugin"
)

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to config file")
	pidFile := flag.String("pidfile", "", "write PID to this file")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	if *debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	if err := run(*configPath, *pidFile); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath, pidFile string) error {
	if pidFile != "" {
		if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
			return fmt.Errorf("write pidfile: %w", err)
		}
		defer os.Remove(pidFile)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	sessions := buildSessions(cfg.Frontends)
	backends := buildBackends(cfg.Backends)
	routing := buildRouting(cfg.Frontends)

	plugins, err := plugin.LoadPlugins(cfg.Plugins)
	if err != nil {
		return fmt.Errorf("load plugins: %w", err)
	}

	bridge := core.NewBridge(routing, backends, plugins, sessions)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	frontends := startFrontends(ctx, cfg.Frontends, bridge)
	if len(frontends) == 0 {
		return fmt.Errorf("no frontends started")
	}

	errCh := make(chan error, len(frontends))
	launchFrontends(ctx, frontends, bridge, errCh)

	go watchFailures(errCh, len(frontends), cancel)

	waitForShutdown(ctx, cancel, frontends, errCh)
	return nil
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

func buildSessions(frontends map[string]config.FrontendConfig) map[string]core.SessionStore {
	sessions := make(map[string]core.SessionStore, len(frontends))
	for name, fc := range frontends {
		sessions[name] = core.NewFileSessionStore(fc.Sessions)
	}
	return sessions
}

func buildBackends(backends map[string]config.BackendConfig) map[string]core.LLMBackend {
	result := make(map[string]core.LLMBackend, len(backends))
	for name, bc := range backends {
		switch bc.Type {
		case config.BackendTypeClaude:
			result[name] = claude.NewClaudeBackend(bc.Binary, bc.TimeoutMinutes)
		default:
			slog.Warn("unknown backend type, skipping", "name", name, "type", bc.Type)
		}
	}
	return result
}

func buildRouting(frontends map[string]config.FrontendConfig) map[string]core.FrontendRouting {
	routing := make(map[string]core.FrontendRouting, len(frontends))
	for name, fc := range frontends {
		routing[name] = core.FrontendRouting{
			BackendName:    fc.Backend,
			Model:          fc.Model,
			PermissionMode: fc.PermissionMode,
			SystemPrompt:   fc.AppendSystemPrompt,
			Agent:          mapAgent(fc.Agent),
		}
	}
	return routing
}

func startFrontends(ctx context.Context, cfgFrontends map[string]config.FrontendConfig, bridge *core.Bridge) []core.ChatFrontend {
	var frontends []core.ChatFrontend
	for name, fc := range cfgFrontends {
		switch fc.Type {
		case config.FrontendTypeTelegram:
			fe := createTelegramFrontend(name, fc, bridge)
			if fe != nil {
				frontends = append(frontends, fe)
			}
		default:
			slog.Warn("unknown frontend type, skipping", "name", name, "type", fc.Type)
		}
	}
	return frontends
}

func createTelegramFrontend(name string, fc config.FrontendConfig, bridge *core.Bridge) core.ChatFrontend {
	api, err := tgbotapi.NewBotAPI(fc.Token)
	if err != nil {
		slog.Error("failed to create bot API", "bot", name, "error", err)
		return nil
	}
	slog.Info("bot authenticated", "name", name, "username", api.Self.UserName)

	return telegram.NewFrontend(name, telegram.FrontendConfig{
		Token: fc.Token,
		Users: fc.Users,
	}, api, bridge)
}

func launchFrontends(ctx context.Context, frontends []core.ChatFrontend, bridge *core.Bridge, errCh chan<- error) {
	for _, fe := range frontends {
		h := bridge.Handler(fe.Name())
		go func(fe core.ChatFrontend, h core.MessageHandler) {
			if err := fe.Start(ctx, h); err != nil && ctx.Err() == nil {
				slog.Error("frontend failed", "name", fe.Name(), "error", err)
				errCh <- err
			}
		}(fe, h)
	}
}

func watchFailures(errCh <-chan error, total int, cancel context.CancelFunc) {
	var failCount int
	for range errCh {
		failCount++
		if failCount >= total {
			slog.Error("all frontends failed, shutting down")
			cancel()
			return
		}
	}
}

func waitForShutdown(ctx context.Context, cancel context.CancelFunc, frontends []core.ChatFrontend, errCh chan error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("received signal, shutting down", "signal", sig)
	case <-ctx.Done():
		slog.Info("context cancelled, shutting down")
	}
	cancel()

	var wg sync.WaitGroup
	for _, fe := range frontends {
		wg.Add(1)
		go func(fe core.ChatFrontend) {
			defer wg.Done()
			if err := fe.Stop(); err != nil {
				slog.Error("frontend stop error", "name", fe.Name(), "error", err)
			}
		}(fe)
	}
	wg.Wait()
	close(errCh)
	slog.Info("shutdown complete")
}

func mapAgent(cfg *config.AgentConfig) *core.AgentDef {
	if cfg == nil {
		return nil
	}
	return &core.AgentDef{
		Name:        cfg.Name,
		Description: cfg.Description,
		Prompt:      cfg.Prompt,
		Tools:       cfg.Tools,
	}
}

