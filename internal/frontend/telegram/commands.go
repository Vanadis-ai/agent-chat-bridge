package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (f *Frontend) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		f.cmdStart(msg)
	case "help":
		f.cmdHelp(msg)
	case "new":
		f.cmdNew(msg)
	case "stop":
		f.cmdStop(msg)
	case "status":
		f.cmdStatus(msg)
	default:
		f.sendText(msg.Chat.ID, "Unknown command. Use /help for available commands.")
	}
}

func (f *Frontend) cmdStart(msg *tgbotapi.Message) {
	text := fmt.Sprintf(
		"Welcome! I am %s bot.\n\n"+
			"Commands:\n"+
			"/new - Start a new conversation\n"+
			"/stop - Cancel current request\n"+
			"/status - Show current status\n"+
			"/help - Show this help",
		f.name,
	)
	f.sendText(msg.Chat.ID, text)
}

func (f *Frontend) cmdHelp(msg *tgbotapi.Message) {
	f.cmdStart(msg)
}

func (f *Frontend) cmdNew(msg *tgbotapi.Message) {
	chatID := formatID(msg.Chat.ID)
	userID := formatID(msg.From.ID)
	f.bridge.ResetSession(f.name, chatID, userID)
	f.sendText(msg.Chat.ID, "Session reset. Starting a new conversation.")
}

func (f *Frontend) cmdStop(msg *tgbotapi.Message) {
	chatID := formatID(msg.Chat.ID)
	userID := formatID(msg.From.ID)
	if f.bridge.CancelRequest(f.name, chatID, userID) {
		f.sendText(msg.Chat.ID, "Request cancelled.")
	} else {
		f.sendText(msg.Chat.ID, "No active request to cancel.")
	}
}

func (f *Frontend) cmdStatus(msg *tgbotapi.Message) {
	chatID := formatID(msg.Chat.ID)
	userID := formatID(msg.From.ID)

	active := f.bridge.HasActive(f.name, chatID, userID)
	sessionID := f.bridge.SessionID(f.name, chatID, userID)

	status := "No active request."
	if active {
		status = "Request in progress."
	}
	if sessionID != "" {
		display := sessionID
		if len(display) > 8 {
			display = display[:8]
		}
		status += fmt.Sprintf("\nSession: %s", display)
	} else {
		status += "\nNo active session."
	}
	f.sendText(msg.Chat.ID, status)
}
