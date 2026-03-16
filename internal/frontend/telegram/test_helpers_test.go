package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

// mockBridge implements core.BridgeCallbacks for testing.
type mockBridge struct {
	cancelReturn bool
	hasActiveVal bool
	sessionIDVal string

	cancelCalled bool
	resetCalled  bool
	lastFrontend string
	lastChatID   string
	lastUserID   string
}

func (m *mockBridge) CancelRequest(frontend, chatID, userID string) bool {
	m.cancelCalled = true
	m.lastFrontend = frontend
	m.lastChatID = chatID
	m.lastUserID = userID
	return m.cancelReturn
}

func (m *mockBridge) ResetSession(frontend, chatID, userID string) {
	m.resetCalled = true
	m.lastFrontend = frontend
	m.lastChatID = chatID
	m.lastUserID = userID
}

func (m *mockBridge) HasActive(frontend, chatID, userID string) bool {
	m.lastFrontend = frontend
	m.lastChatID = chatID
	m.lastUserID = userID
	return m.hasActiveVal
}

func (m *mockBridge) SessionID(frontend, chatID, userID string) string {
	return m.sessionIDVal
}

// mockSender implements TelegramSender for testing.
type mockSender struct {
	sentMessages []tgbotapi.Chattable
	nextMsgID    int
}

func newMockSender() *mockSender {
	return &mockSender{nextMsgID: 1}
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sentMessages = append(m.sentMessages, c)
	msg := tgbotapi.Message{MessageID: m.nextMsgID}
	m.nextMsgID++
	return msg, nil
}

func (m *mockSender) GetFileDirectURL(fileID string) (string, error) {
	return "http://test/" + fileID, nil
}

func (m *mockSender) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	m.sentMessages = append(m.sentMessages, c)
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (m *mockSender) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(tgbotapi.UpdatesChannel)
}

func (m *mockSender) lastSentText() string {
	if len(m.sentMessages) == 0 {
		return ""
	}
	last := m.sentMessages[len(m.sentMessages)-1]
	return extractText(last)
}

func (m *mockSender) allSentTexts() []string {
	var texts []string
	for _, msg := range m.sentMessages {
		if t := extractText(msg); t != "" {
			texts = append(texts, t)
		}
	}
	return texts
}

func extractText(c tgbotapi.Chattable) string {
	switch v := c.(type) {
	case tgbotapi.MessageConfig:
		return v.Text
	case tgbotapi.EditMessageTextConfig:
		return v.Text
	default:
		return ""
	}
}

// mockDownloader implements fileDownloader for testing.
type mockDownloader struct {
	result *downloadResult
	err    error
}

func (m *mockDownloader) saveVoice(_ *tgbotapi.Voice, _ *config.UserConfig) (*downloadResult, error) {
	return m.result, m.err
}

func (m *mockDownloader) saveAudio(_ *tgbotapi.Audio, _ *config.UserConfig) (*downloadResult, error) {
	return m.result, m.err
}

func (m *mockDownloader) saveDocument(_ *tgbotapi.Document, _ *config.UserConfig) (*downloadResult, error) {
	return m.result, m.err
}

func (m *mockDownloader) savePhoto(_ []tgbotapi.PhotoSize, _ *config.UserConfig) (*downloadResult, error) {
	return m.result, m.err
}

func (m *mockDownloader) saveVideo(_ *tgbotapi.Video, _ *config.UserConfig) (*downloadResult, error) {
	return m.result, m.err
}

// stubHandler returns a MessageHandler that writes deltas, closes deltaCh
// (simulating Bridge's defer close), and returns the given response.
func stubHandler(deltas []string, resp *core.Response, err error) core.MessageHandler {
	return func(ctx context.Context, msg core.ChatMessage, deltaCh chan<- core.StreamDelta) (*core.Response, error) {
		defer close(deltaCh)
		for _, d := range deltas {
			deltaCh <- core.StreamDelta{Text: d}
		}
		return resp, err
	}
}
