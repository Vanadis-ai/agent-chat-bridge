package plugin

import (
	"context"
	"testing"

	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

func TestRequestLoggerRegistered(t *testing.T) {
	p, err := Create("request_logger")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, ok := p.(*RequestLogger); !ok {
		t.Errorf("type = %T, want *RequestLogger", p)
	}
}

func TestRequestLoggerInitWithConfig(t *testing.T) {
	p := &RequestLogger{}
	err := p.Init(map[string]any{"log_file": "/tmp/test.log"})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}
	if p.logFile != "/tmp/test.log" {
		t.Errorf("logFile = %q, want %q", p.logFile, "/tmp/test.log")
	}
}

func TestRequestLoggerInitEmptyConfig(t *testing.T) {
	p := &RequestLogger{}
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init nil error: %v", err)
	}
	if err := p.Init(map[string]any{}); err != nil {
		t.Fatalf("Init empty error: %v", err)
	}
}

func TestRequestLoggerInitBadType(t *testing.T) {
	p := &RequestLogger{}
	err := p.Init(map[string]any{"log_file": 123})
	if err == nil {
		t.Fatal("expected error for non-string log_file")
	}
}

func TestRequestLoggerHandleMessageReturnsNil(t *testing.T) {
	p := &RequestLogger{}
	_ = p.Init(nil)

	msg := core.ChatMessage{
		ChatID: "100", UserID: "200", Text: "hello",
		Attachments: []core.Attachment{{Type: core.AttachmentImage}},
	}
	deltaCh := make(chan core.StreamDelta, 10)

	resp, err := p.HandleMessage(context.Background(), msg, deltaCh)
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	if resp != nil {
		t.Error("response should be nil for pass-through plugin")
	}
}

func TestRequestLoggerDoesNotCloseDeltaCh(t *testing.T) {
	p := &RequestLogger{}
	_ = p.Init(nil)

	deltaCh := make(chan core.StreamDelta, 10)
	msg := core.ChatMessage{ChatID: "1", UserID: "1", Text: "test"}

	_, _ = p.HandleMessage(context.Background(), msg, deltaCh)

	// Verify channel is still open by sending to it (should not panic)
	deltaCh <- core.StreamDelta{Text: "still open"}
	select {
	case d := <-deltaCh:
		if d.Text != "still open" {
			t.Errorf("got %q, want %q", d.Text, "still open")
		}
	default:
		t.Error("channel should have a value")
	}
}
