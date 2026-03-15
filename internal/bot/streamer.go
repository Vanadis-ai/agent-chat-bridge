package bot

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
	text := s.buf.String()
	textLen := len(text)
	changed := textLen != s.lastLen
	s.lastLen = textLen
	s.timer = nil
	s.mu.Unlock()

	if !changed || text == "" {
		return
	}

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
