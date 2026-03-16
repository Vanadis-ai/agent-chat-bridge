package telegram

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/formatter"
)

const (
	editInterval = 2 * time.Second
	maxMsgLen    = 4096
)

// Streamer progressively edits a Telegram message with incoming text.
type Streamer struct {
	sender  TelegramSender
	chatID  int64
	msgID   int
	mu      sync.Mutex
	buf     strings.Builder
	lastLen int
	timer   *time.Timer
}

// NewStreamer creates a streamer for a given chat.
func NewStreamer(sender TelegramSender, chatID int64) *Streamer {
	return &Streamer{
		sender: sender,
		chatID: chatID,
	}
}

// SendInitial sends the first "thinking" message and captures its ID.
func (s *Streamer) SendInitial() error {
	msg := tgbotapi.NewMessage(s.chatID, "...")
	sent, err := s.sender.Send(msg)
	if err != nil {
		return err
	}
	s.msgID = sent.MessageID
	return nil
}

// Append adds text to the buffer and schedules an edit.
func (s *Streamer) Append(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.WriteString(text)
	if s.timer == nil {
		s.timer = time.AfterFunc(editInterval, s.flush)
	}
}

// HasContent returns true if any text has been appended to the buffer.
func (s *Streamer) HasContent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Len() > 0
}

// EditPlaceholder replaces the initial "..." message with the given text.
// Used for error messages or non-streamed responses.
func (s *Streamer) EditPlaceholder(text string) {
	s.editMessage(text)
}

// Finalize sends the final complete message, splitting if needed.
func (s *Streamer) Finalize() {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	fullText := s.buf.String()
	s.mu.Unlock()

	if fullText == "" {
		return
	}
	chunks := formatter.Split(fullText)
	if len(chunks) == 0 {
		return
	}
	s.editMessage(chunks[0])
	for i := 1; i < len(chunks); i++ {
		s.sendNew(chunks[i])
	}
}

func (s *Streamer) flush() {
	s.mu.Lock()
	currentLen := s.buf.Len()
	changed := currentLen != s.lastLen
	if !changed || currentLen == 0 {
		s.timer = nil
		s.mu.Unlock()
		return
	}
	text := s.buf.String()
	s.lastLen = currentLen
	s.timer = nil
	s.mu.Unlock()

	display := text
	if len(display) > maxMsgLen {
		display = display[len(display)-maxMsgLen:]
	}
	s.editMessage(display)

	s.mu.Lock()
	if s.buf.Len() != s.lastLen {
		s.timer = time.AfterFunc(editInterval, s.flush)
	}
	s.mu.Unlock()
}

func (s *Streamer) editMessage(text string) {
	edit := tgbotapi.NewEditMessageText(s.chatID, s.msgID, text)
	if _, err := s.sender.Request(edit); err != nil {
		slog.Error("failed to edit message", "error", err)
	}
}

func (s *Streamer) sendNew(text string) {
	msg := tgbotapi.NewMessage(s.chatID, text)
	if _, err := s.sender.Send(msg); err != nil {
		slog.Error("failed to send message", "error", err)
	}
}
