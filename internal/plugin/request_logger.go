package plugin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

func init() {
	Register("request_logger", func() core.Plugin { return &RequestLogger{} })
}

// RequestLogger logs every incoming message. Pass-through: always returns nil response.
type RequestLogger struct {
	logFile string
}

// Init reads optional "log_file" from config.
func (p *RequestLogger) Init(cfg map[string]any) error {
	if cfg == nil {
		return nil
	}
	v, ok := cfg["log_file"]
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("request_logger: log_file must be a string, got %T", v)
	}
	p.logFile = s
	return nil
}

// HandleMessage logs message info and passes through.
// Does NOT close deltaCh. Does NOT write to deltaCh.
func (p *RequestLogger) HandleMessage(
	_ context.Context,
	msg core.ChatMessage,
	_ chan<- core.StreamDelta,
) (*core.Response, error) {
	slog.Info("plugin: incoming message",
		"user", msg.UserID,
		"chat", msg.ChatID,
		"text_len", len(msg.Text),
		"attachments", len(msg.Attachments),
	)
	return nil, nil
}
