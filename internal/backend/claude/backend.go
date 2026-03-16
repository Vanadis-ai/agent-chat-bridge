package claude

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	claudecode "github.com/severity1/claude-agent-sdk-go"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

// ClaudeBackend implements core.LLMBackend using Claude CLI SDK.
type ClaudeBackend struct {
	binary         string
	timeoutMinutes int
	env            map[string]string // computed once at construction
}

// NewClaudeBackend creates a backend from config options.
func NewClaudeBackend(binary string, timeoutMinutes int) *ClaudeBackend {
	return &ClaudeBackend{
		binary:         binary,
		timeoutMinutes: timeoutMinutes,
		env:            filteredEnv(),
	}
}

// Run executes a Claude CLI query with streaming.
// Does NOT close deltaCh -- the caller (Bridge) owns the close.
// All writes to deltaCh finish before this method returns.
func (b *ClaudeBackend) Run(
	ctx context.Context,
	req core.BackendRequest,
	deltaCh chan<- core.StreamDelta,
) (*core.Response, error) {
	if b.timeoutMinutes > 0 {
		timeout := time.Duration(b.timeoutMinutes) * time.Minute
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	opts := buildOptions(b, req)

	slog.Info("claude backend run starting",
		"model", req.Model,
		"cwd", req.WorkingDir,
		"session", req.SessionID,
		"prompt_len", len(req.Prompt),
	)

	var result *core.Response

	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		if err := client.Query(ctx, req.Prompt); err != nil {
			return fmt.Errorf("query: %w", err)
		}

		msgChan := client.ReceiveMessages(ctx)
		for {
			select {
			case msg := <-msgChan:
				if msg == nil {
					return nil
				}
				result = handleMessage(msg, deltaCh, result)
				if result != nil && result.SessionID != "" {
					return nil
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}, opts...)

	if err != nil {
		return nil, fmt.Errorf("claude run: %w", err)
	}

	if result == nil {
		return &core.Response{IsError: true, Error: "no result received"}, nil
	}

	return result, nil
}

func handleMessage(
	msg claudecode.Message,
	deltaCh chan<- core.StreamDelta,
	current *core.Response,
) *core.Response {
	switch m := msg.(type) {
	case *claudecode.StreamEvent:
		text := extractDeltaText(m)
		if text != "" {
			deltaCh <- core.StreamDelta{Text: text}
		}
	case *claudecode.AssistantMessage:
		// Intentionally not sending to deltaCh here.
		// With partial streaming enabled, StreamEvent deltas already
		// contain the full text. AssistantMessage would duplicate it.
	case *claudecode.ResultMessage:
		r := &core.Response{
			SessionID: m.SessionID,
			IsError:   m.IsError,
			NumTurns:  m.NumTurns,
		}
		if m.TotalCostUSD != nil {
			r.CostUSD = *m.TotalCostUSD
		}
		if m.IsError && m.Result != nil {
			r.Error = *m.Result
		}
		return r
	}
	return current
}

func extractDeltaText(e *claudecode.StreamEvent) string {
	eventType, ok := e.Event["type"].(string)
	if !ok {
		return ""
	}
	if eventType != "content_block_delta" {
		return ""
	}
	delta, ok := e.Event["delta"].(map[string]any)
	if !ok {
		return ""
	}
	text, ok := delta["text"].(string)
	if !ok {
		return ""
	}
	return text
}
