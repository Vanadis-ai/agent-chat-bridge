package bot

import (
	"context"
	"log/slog"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/claude"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
)

// Bot represents a single Telegram bot instance.
type Bot struct {
	name     string
	config   config.BotConfig
	claude   config.ClaudeConfig
	sender   TelegramSender
	sessions *claude.SessionStore
	stopFunc context.CancelFunc

	mu             sync.Mutex
	activeRequests map[int64]context.CancelFunc
}

// NewBot creates a new bot instance.
func NewBot(name string, cfg config.BotConfig, claudeCfg config.ClaudeConfig, sender TelegramSender, sessions *claude.SessionStore) *Bot {
	return &Bot{
		name:           name,
		config:         cfg,
		claude:         claudeCfg,
		sender:         sender,
		sessions:       sessions,
		activeRequests: make(map[int64]context.CancelFunc),
	}
}

// Start begins the bot's polling loop.
func (b *Bot) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	b.stopFunc = cancel

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.sender.GetUpdatesChan(u)
	slog.Info("bot started", "name", b.name)

	for {
		select {
		case <-ctx.Done():
			slog.Info("bot stopping", "name", b.name)
			return
		case update := <-updates:
			b.handleUpdate(update)
		}
	}
}

// Stop cancels the bot's polling loop.
func (b *Bot) Stop() {
	if b.stopFunc != nil {
		b.stopFunc()
	}
}

func (b *Bot) setActive(userID int64, cancel context.CancelFunc) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.activeRequests[userID]; exists {
		return false
	}
	b.activeRequests[userID] = cancel
	return true
}

func (b *Bot) clearActive(userID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.activeRequests, userID)
}

func (b *Bot) cancelActive(userID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	cancel, exists := b.activeRequests[userID]
	if !exists {
		return false
	}
	cancel()
	delete(b.activeRequests, userID)
	return true
}

func (b *Bot) hasActive(userID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, exists := b.activeRequests[userID]
	return exists
}

func (b *Bot) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.sender.Send(msg); err != nil {
		slog.Error("failed to send message", "bot", b.name, "error", err)
	}
}
