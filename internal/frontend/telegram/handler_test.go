package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

func newTestFrontendWithHandler(
	bridge *mockBridge,
	handler core.MessageHandler,
) (*Frontend, *mockSender) {
	sender := newMockSender()
	f := &Frontend{
		name:       "testbot",
		config:     FrontendConfig{Users: map[int64]*config.UserConfig{100: {WorkingDir: "/tmp"}}},
		sender:     sender,
		bridge:     bridge,
		downloader: &mockDownloader{},
		handler:    handler,
	}
	return f, sender
}

func TestRunRequestErrRequestActive(t *testing.T) {
	bridge := &mockBridge{}
	handler := stubHandler(nil, nil, core.ErrRequestActive)
	f, sender := newTestFrontendWithHandler(bridge, handler)

	chatMsg := core.ChatMessage{
		ChatID: "100", UserID: "100", Text: "hello",
	}
	f.runRequest(context.Background(), 100, chatMsg)

	found := false
	for _, text := range sender.allSentTexts() {
		if strings.Contains(text, "Previous request is still running") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Previous request is still running' message for ErrRequestActive")
	}
}

func TestRunRequestHasActivePreCheck(t *testing.T) {
	bridge := &mockBridge{hasActiveVal: true}
	handlerCalled := false
	handler := func(ctx context.Context, msg core.ChatMessage, deltaCh chan<- core.StreamDelta) (*core.Response, error) {
		handlerCalled = true
		return nil, nil
	}
	f, sender := newTestFrontendWithHandler(bridge, handler)

	chatMsg := core.ChatMessage{
		ChatID: "100", UserID: "100", Text: "hello",
	}
	f.runRequest(context.Background(), 100, chatMsg)

	if handlerCalled {
		t.Error("handler should not be called when HasActive is true")
	}
	text := sender.lastSentText()
	if !strings.Contains(text, "Previous request is still running") {
		t.Errorf("response = %q, want 'Previous request is still running'", text)
	}
}

func TestRunRequestStreamingDrain(t *testing.T) {
	bridge := &mockBridge{}
	deltas := []string{"Hello", " ", "world", "!"}
	resp := &core.Response{SessionID: "sess-1", CostUSD: 0.01, NumTurns: 1}
	handler := stubHandler(deltas, resp, nil)
	f, sender := newTestFrontendWithHandler(bridge, handler)

	chatMsg := core.ChatMessage{
		ChatID: "100", UserID: "100", Text: "hello",
	}

	done := make(chan struct{})
	go func() {
		f.runRequest(context.Background(), 100, chatMsg)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runRequest timed out")
	}

	// Verify we got messages (initial "..." + possibly edits + finalize)
	if len(sender.sentMessages) < 1 {
		t.Error("expected at least one sent message")
	}
}

func TestRunRequestHandlerError(t *testing.T) {
	bridge := &mockBridge{}
	handler := stubHandler(nil, nil, errors.New("timeout"))
	f, sender := newTestFrontendWithHandler(bridge, handler)

	chatMsg := core.ChatMessage{
		ChatID: "100", UserID: "100", Text: "hello",
	}
	f.runRequest(context.Background(), 100, chatMsg)

	found := false
	for _, text := range sender.allSentTexts() {
		if strings.Contains(text, "Error: timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error: timeout' message")
	}
}

func TestRunRequestBackendError(t *testing.T) {
	bridge := &mockBridge{}
	resp := &core.Response{IsError: true, Error: "quota exceeded"}
	handler := stubHandler(nil, resp, nil)
	f, sender := newTestFrontendWithHandler(bridge, handler)

	chatMsg := core.ChatMessage{
		ChatID: "100", UserID: "100", Text: "hello",
	}
	f.runRequest(context.Background(), 100, chatMsg)

	found := false
	for _, text := range sender.allSentTexts() {
		if strings.Contains(text, "quota exceeded") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'quota exceeded' error message")
	}
}

func TestHandleUpdateUnauthorized(t *testing.T) {
	bridge := &mockBridge{}
	f, sender := newTestFrontendWithHandler(bridge, nil)

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 999, Type: "private"},
			From:      &tgbotapi.User{ID: 999, UserName: "unknown"},
			Text:      "hello",
		},
	}
	f.handleUpdate(context.Background(), update)

	text := sender.lastSentText()
	if !strings.Contains(text, "not authorized") {
		t.Errorf("response = %q, want 'not authorized'", text)
	}
}

func TestHandleUpdateGroupChat(t *testing.T) {
	bridge := &mockBridge{}
	f, sender := newTestFrontendWithHandler(bridge, nil)

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 100, Type: "group"},
			From:      &tgbotapi.User{ID: 100},
			Text:      "hello",
		},
	}
	f.handleUpdate(context.Background(), update)

	if len(sender.sentMessages) != 0 {
		t.Error("group chat messages should be silently rejected")
	}
}
