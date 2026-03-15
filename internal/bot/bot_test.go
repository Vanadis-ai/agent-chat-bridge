package bot

import (
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/claude"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
)

type mockSender struct {
	mu       sync.Mutex
	messages []tgbotapi.Chattable
	msgID    int
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, c)
	m.msgID++
	return tgbotapi.Message{MessageID: m.msgID}, nil
}

func (m *mockSender) GetFileDirectURL(fileID string) (string, error) {
	return "http://localhost/file/" + fileID, nil
}

func (m *mockSender) GetFile(cfg tgbotapi.FileConfig) (tgbotapi.File, error) {
	return tgbotapi.File{}, nil
}

func (m *mockSender) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, c)
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (m *mockSender) GetUpdatesChan(ucfg tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(chan tgbotapi.Update)
}

func (m *mockSender) lastMessageText() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) == 0 {
		return ""
	}
	last := m.messages[len(m.messages)-1]
	if msg, ok := last.(tgbotapi.MessageConfig); ok {
		return msg.Text
	}
	return ""
}

func (m *mockSender) messageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func newTestBot(sender *mockSender) *Bot {
	cfg := config.BotConfig{
		Token: "test_token",
		Users: map[int64]*config.UserConfig{
			123: {WorkingDir: "/tmp", VoiceDir: "/tmp/voice", FilesDir: "/tmp/files"},
		},
	}
	claudeCfg := config.ClaudeConfig{
		Binary:         "/nonexistent",
		TimeoutMinutes: 1,
		MaxConcurrent:  5,
	}
	sessions := claude.NewSessionStore("/tmp/test_sessions.json")
	return NewBot("test", cfg, claudeCfg, sender, sessions)
}

func TestCommandDispatch(t *testing.T) {
	tests := map[string]struct {
		command string
		expect  string
	}{
		"start":   {command: "/start", expect: "Welcome"},
		"help":    {command: "/help", expect: "Welcome"},
		"new":     {command: "/new", expect: "reset"},
		"status":  {command: "/status", expect: "No active request"},
		"unknown": {command: "/foo", expect: "Unknown command"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sender := &mockSender{}
			b := newTestBot(sender)

			msg := &tgbotapi.Message{
				Chat:     &tgbotapi.Chat{ID: 1, Type: "private"},
				From:     &tgbotapi.User{ID: 123},
				Text:     tc.command,
				Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Length: len(tc.command)}},
			}

			b.handleUpdate(tgbotapi.Update{Message: msg})

			text := sender.lastMessageText()
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.expect)) {
				t.Errorf("response %q should contain %q", text, tc.expect)
			}
		})
	}
}

func TestTextMessageStartsClaude(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)

	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 1, Type: "private"},
		From: &tgbotapi.User{ID: 123},
		Text: "hello world",
	}

	b.handleUpdate(tgbotapi.Update{Message: msg})

	// Give goroutine time to start and send initial "..." message.
	time.Sleep(200 * time.Millisecond)

	if !b.hasActive(123) {
		// Claude may have already failed (no binary), but initial message should be sent.
	}

	if sender.messageCount() == 0 {
		t.Error("expected at least one message (initial '...')")
	}
}

func TestActiveRequestRejection(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)

	b.mu.Lock()
	b.activeRequests[123] = func() {}
	b.mu.Unlock()

	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 1, Type: "private"},
		From: &tgbotapi.User{ID: 123},
		Text: "hello",
	}

	b.handleUpdate(tgbotapi.Update{Message: msg})

	text := sender.lastMessageText()
	if !strings.Contains(text, "still running") {
		t.Errorf("response %q should mention still running", text)
	}
}

func TestStopCommand(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)

	cancelled := false
	b.mu.Lock()
	b.activeRequests[123] = func() { cancelled = true }
	b.mu.Unlock()

	msg := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 1, Type: "private"},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/stop",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Length: 5}},
	}

	b.handleUpdate(tgbotapi.Update{Message: msg})

	if !cancelled {
		t.Error("expected cancel function to be called")
	}

	text := sender.lastMessageText()
	if !strings.Contains(text, "cancelled") {
		t.Errorf("response %q should mention cancelled", text)
	}
}

func TestUnauthorizedUserRejected(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)

	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 1, Type: "private"},
		From: &tgbotapi.User{ID: 999},
		Text: "hello",
	}

	b.handleUpdate(tgbotapi.Update{Message: msg})

	text := sender.lastMessageText()
	if !strings.Contains(text, "not authorized") {
		t.Errorf("response %q should mention not authorized", text)
	}
}

func TestGroupChatRejected(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)

	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 1, Type: "group"},
		From: &tgbotapi.User{ID: 123},
		Text: "hello",
	}

	b.handleUpdate(tgbotapi.Update{Message: msg})

	if sender.messageCount() != 0 {
		t.Errorf("expected no messages for group chat, got %d", sender.messageCount())
	}
}
