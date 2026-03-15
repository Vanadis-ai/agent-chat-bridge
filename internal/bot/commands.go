package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		b.cmdStart(msg)
	case "help":
		b.cmdHelp(msg)
	case "new":
		b.cmdNew(msg)
	case "stop":
		b.cmdStop(msg)
	case "status":
		b.cmdStatus(msg)
	default:
		b.sendText(msg.Chat.ID, "Unknown command. Use /help for available commands.")
	}
}

func (b *Bot) cmdStart(msg *tgbotapi.Message) {
	text := fmt.Sprintf(
		"Welcome! I am %s bot.\n\n"+
			"Commands:\n"+
			"/new - Start a new conversation\n"+
			"/stop - Cancel current request\n"+
			"/status - Show current status\n"+
			"/help - Show this help",
		b.name,
	)
	b.sendText(msg.Chat.ID, text)
}

func (b *Bot) cmdHelp(msg *tgbotapi.Message) {
	b.cmdStart(msg)
}

func (b *Bot) cmdNew(msg *tgbotapi.Message) {
	userID := msg.From.ID
	b.sessions.Reset(userID)
	b.sendText(msg.Chat.ID, "Session reset. Starting a new conversation.")
}

func (b *Bot) cmdStop(msg *tgbotapi.Message) {
	userID := msg.From.ID
	if b.cancelActive(userID) {
		b.sendText(msg.Chat.ID, "Request cancelled.")
	} else {
		b.sendText(msg.Chat.ID, "No active request to cancel.")
	}
}

func (b *Bot) cmdStatus(msg *tgbotapi.Message) {
	userID := msg.From.ID
	active := b.hasActive(userID)
	sessionID := b.sessions.Get(userID)

	status := "No active request."
	if active {
		status = "Request in progress."
	}

	if sessionID != "" {
		status += fmt.Sprintf("\nSession: %s", sessionID[:8])
	} else {
		status += "\nNo active session."
	}

	b.sendText(msg.Chat.ID, status)
}
