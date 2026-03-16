package telegram

import (
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
)

func newTestMsg(chatID, userID int64, text string) *tgbotapi.Message {
	msg := &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: chatID, Type: "private"},
		From:      &tgbotapi.User{ID: userID},
		Text:      text,
	}
	if strings.HasPrefix(text, "/") {
		msg.Entities = []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len(text)},
		}
	}
	return msg
}

func newTestFrontend(bridge *mockBridge) (*Frontend, *mockSender) {
	sender := newMockSender()
	f := &Frontend{
		name:   "testbot",
		config: FrontendConfig{Users: map[int64]*config.UserConfig{100: {WorkingDir: "/tmp"}}},
		sender: sender,
		bridge: bridge,
	}
	return f, sender
}

func TestCmdNew(t *testing.T) {
	bridge := &mockBridge{}
	f, sender := newTestFrontend(bridge)

	f.handleCommand(newTestMsg(100, 100, "/new"))

	if !bridge.resetCalled {
		t.Error("ResetSession should be called")
	}
	if bridge.lastFrontend != "testbot" {
		t.Errorf("frontend = %q, want %q", bridge.lastFrontend, "testbot")
	}
	if bridge.lastChatID != "100" {
		t.Errorf("chatID = %q, want %q", bridge.lastChatID, "100")
	}
	text := sender.lastSentText()
	if !strings.Contains(text, "reset") {
		t.Errorf("response = %q, want to contain 'reset'", text)
	}
}

func TestCmdStopActive(t *testing.T) {
	bridge := &mockBridge{cancelReturn: true}
	f, sender := newTestFrontend(bridge)

	f.handleCommand(newTestMsg(100, 100, "/stop"))

	if !bridge.cancelCalled {
		t.Error("CancelRequest should be called")
	}
	text := sender.lastSentText()
	if !strings.Contains(text, "cancelled") {
		t.Errorf("response = %q, want to contain 'cancelled'", text)
	}
}

func TestCmdStopNoActive(t *testing.T) {
	bridge := &mockBridge{cancelReturn: false}
	f, sender := newTestFrontend(bridge)

	f.handleCommand(newTestMsg(100, 100, "/stop"))

	text := sender.lastSentText()
	if !strings.Contains(text, "No active request") {
		t.Errorf("response = %q, want to contain 'No active request'", text)
	}
}

func TestCmdStatusActive(t *testing.T) {
	bridge := &mockBridge{hasActiveVal: true, sessionIDVal: "abcdef1234567890"}
	f, sender := newTestFrontend(bridge)

	f.handleCommand(newTestMsg(100, 100, "/status"))

	text := sender.lastSentText()
	if !strings.Contains(text, "Request in progress") {
		t.Errorf("response = %q, want to contain 'Request in progress'", text)
	}
	if !strings.Contains(text, "abcdef12") {
		t.Errorf("response = %q, want to contain session prefix", text)
	}
}

func TestCmdStatusShortSessionID(t *testing.T) {
	bridge := &mockBridge{hasActiveVal: false, sessionIDVal: "abc"}
	f, sender := newTestFrontend(bridge)

	f.handleCommand(newTestMsg(100, 100, "/status"))

	text := sender.lastSentText()
	if !strings.Contains(text, "abc") {
		t.Errorf("response = %q, want to contain short session ID", text)
	}
}

func TestCmdStatusNoSession(t *testing.T) {
	bridge := &mockBridge{hasActiveVal: false, sessionIDVal: ""}
	f, sender := newTestFrontend(bridge)

	f.handleCommand(newTestMsg(100, 100, "/status"))

	text := sender.lastSentText()
	if !strings.Contains(text, "No active request") {
		t.Errorf("response = %q, want to contain 'No active request'", text)
	}
	if !strings.Contains(text, "No active session") {
		t.Errorf("response = %q, want to contain 'No active session'", text)
	}
}

func TestCmdStart(t *testing.T) {
	bridge := &mockBridge{}
	f, sender := newTestFrontend(bridge)

	f.handleCommand(newTestMsg(100, 100, "/start"))

	text := sender.lastSentText()
	if !strings.Contains(text, "Welcome") {
		t.Errorf("response = %q, want to contain 'Welcome'", text)
	}
}

func TestCmdUnknown(t *testing.T) {
	bridge := &mockBridge{}
	f, sender := newTestFrontend(bridge)

	f.handleCommand(newTestMsg(100, 100, "/unknown"))

	text := sender.lastSentText()
	if !strings.Contains(text, "Unknown command") {
		t.Errorf("response = %q, want to contain 'Unknown command'", text)
	}
}
