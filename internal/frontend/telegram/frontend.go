package telegram

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

// FrontendConfig holds only what the Telegram frontend needs.
// Model, PermissionMode, SystemPrompt, Agent, Sessions live in Bridge routing.
type FrontendConfig struct {
	Token string
	Users map[int64]*config.UserConfig
}

// Frontend implements core.ChatFrontend for Telegram.
type Frontend struct {
	name       string
	config     FrontendConfig
	sender     TelegramSender
	handler    core.MessageHandler
	bridge     core.BridgeCallbacks
	downloader fileDownloader
	stopFunc   context.CancelFunc
}

// NewFrontend creates a new Telegram frontend instance.
func NewFrontend(
	name string,
	cfg FrontendConfig,
	sender TelegramSender,
	bridge core.BridgeCallbacks,
) *Frontend {
	return &Frontend{
		name:       name,
		config:     cfg,
		sender:     sender,
		bridge:     bridge,
		downloader: newDownloader(sender, name),
	}
}

// Start begins the polling loop. Blocks until ctx is cancelled.
func (f *Frontend) Start(ctx context.Context, handler core.MessageHandler) error {
	ctx, cancel := context.WithCancel(ctx)
	f.stopFunc = cancel
	f.handler = handler

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := f.sender.GetUpdatesChan(u)

	slog.Info("frontend started", "name", f.name)

	for {
		select {
		case <-ctx.Done():
			slog.Info("frontend stopping", "name", f.name)
			return nil
		case update, ok := <-updates:
			if !ok {
				slog.Error("updates channel closed", "name", f.name)
				return fmt.Errorf("frontend %s: updates channel closed", f.name)
			}
			f.handleUpdate(ctx, update)
		}
	}
}

// Stop gracefully shuts down the frontend.
func (f *Frontend) Stop() error {
	if f.stopFunc != nil {
		f.stopFunc()
	}
	return nil
}

// Name returns the frontend instance name.
func (f *Frontend) Name() string {
	return f.name
}

func (f *Frontend) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := f.sender.Send(msg); err != nil {
		slog.Error("failed to send message", "bot", f.name, "error", err)
	}
}
