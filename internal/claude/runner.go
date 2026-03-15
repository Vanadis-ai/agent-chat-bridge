package claude

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	claudecode "github.com/severity1/claude-agent-sdk-go"
)

// AgentDef holds agent definition passed to Claude CLI.
type AgentDef struct {
	Name        string
	Description string
	Prompt      string
	Tools       []string // nil = default tools, empty = no tools, non-empty = only listed
}

// RunConfig holds parameters for a single Claude invocation.
type RunConfig struct {
	Prompt         string
	WorkingDir     string
	Model          string
	PermissionMode string
	SystemPrompt   string
	SessionID      string
	CLIPath        string
	TimeoutMinutes int
	Agent          *AgentDef
}

// StreamDelta is a chunk of text emitted during streaming.
type StreamDelta struct {
	Text string
}

// RunResult is the final result of a Claude invocation.
type RunResult struct {
	SessionID string
	IsError   bool
	Error     string
	CostUSD   float64
	NumTurns  int
}

// Run executes a Claude CLI query with streaming.
// It sends text deltas to the deltaCh channel and returns the final result.
func Run(ctx context.Context, cfg RunConfig, deltaCh chan<- StreamDelta) (*RunResult, error) {
	defer close(deltaCh)

	opts := buildOptions(cfg)

	slog.Info("claude run starting",
		"model", cfg.Model,
		"cwd", cfg.WorkingDir,
		"session", cfg.SessionID,
		"prompt_len", len(cfg.Prompt),
	)

	var result *RunResult

	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		if err := client.Query(ctx, cfg.Prompt); err != nil {
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
		return &RunResult{IsError: true, Error: "no result received"}, nil
	}

	return result, nil
}

func handleMessage(msg claudecode.Message, deltaCh chan<- StreamDelta, current *RunResult) *RunResult {
	slog.Debug("claude message received", "type", fmt.Sprintf("%T", msg))

	switch m := msg.(type) {
	case *claudecode.StreamEvent:
		text := extractDeltaText(m)
		if text != "" {
			deltaCh <- StreamDelta{Text: text}
		}
	case *claudecode.AssistantMessage:
		text := extractAssistantText(m)
		if text != "" {
			deltaCh <- StreamDelta{Text: text}
		}
	case *claudecode.ResultMessage:
		r := &RunResult{
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
		slog.Info("claude run complete",
			"session", m.SessionID,
			"turns", m.NumTurns,
			"cost_usd", r.CostUSD,
			"is_error", m.IsError,
		)
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

func extractAssistantText(m *claudecode.AssistantMessage) string {
	var parts []string
	for _, block := range m.Content {
		if tb, ok := block.(*claudecode.TextBlock); ok {
			parts = append(parts, tb.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func buildOptions(cfg RunConfig) []claudecode.Option {
	opts := []claudecode.Option{
		claudecode.WithPartialStreaming(),
	}

	if cfg.CLIPath != "" {
		opts = append(opts, claudecode.WithCLIPath(cfg.CLIPath))
	}
	if cfg.WorkingDir != "" {
		opts = append(opts, claudecode.WithCwd(cfg.WorkingDir))
	}
	if cfg.Model != "" {
		opts = append(opts, claudecode.WithModel(cfg.Model))
	}
	if cfg.SystemPrompt != "" {
		opts = append(opts, claudecode.WithAppendSystemPrompt(cfg.SystemPrompt))
	}
	if cfg.SessionID != "" {
		opts = append(opts, claudecode.WithResume(cfg.SessionID))
	}
	if cfg.Agent != nil {
		opts = append(opts, buildAgentOptions(cfg.Agent)...)
	}

	pm := mapPermissionMode(cfg.PermissionMode)
	opts = append(opts, claudecode.WithPermissionMode(pm))

	// Filter CLAUDECODE env vars to avoid nested session detection.
	opts = append(opts, claudecode.WithEnv(filteredEnv()))

	opts = append(opts, claudecode.WithStderrCallback(func(line string) {
		slog.Debug("claude stderr", "line", line)
	}))

	return opts
}

func buildAgentOptions(agent *AgentDef) []claudecode.Option {
	var opts []claudecode.Option

	def := claudecode.AgentDefinition{
		Description: agent.Description,
		Prompt:      agent.Prompt,
	}
	if len(agent.Tools) > 0 {
		def.Tools = agent.Tools
	}
	opts = append(opts, claudecode.WithAgent(agent.Name, def))

	// Activate the agent for this session.
	name := agent.Name
	extraArgs := map[string]*string{"--agent": &name}

	// Empty tools list means disable all tools via --tools "".
	if agent.Tools != nil && len(agent.Tools) == 0 {
		noTools := ""
		extraArgs["--tools"] = &noTools
	}

	opts = append(opts, claudecode.WithExtraArgs(extraArgs))
	return opts
}

func mapPermissionMode(mode string) claudecode.PermissionMode {
	switch mode {
	case "bypassPermissions":
		return claudecode.PermissionModeBypassPermissions
	case "plan":
		return claudecode.PermissionModePlan
	case "acceptEdits":
		return claudecode.PermissionModeAcceptEdits
	default:
		return claudecode.PermissionModeDefault
	}
}

func filteredEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if shouldFilterEnv(k) {
			continue
		}
		env[k] = v
	}
	return env
}

func shouldFilterEnv(key string) bool {
	if key == "CLAUDECODE" {
		return true
	}
	prefixes := []string{"CLAUDE_CODE_", "CLAUDE_MANAGER_", "OTEL_"}
	for _, p := range prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}
