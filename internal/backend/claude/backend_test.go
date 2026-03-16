package claude

import (
	"testing"

	claudecode "github.com/severity1/claude-agent-sdk-go"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

func TestMapPermissionMode(t *testing.T) {
	tests := []struct {
		input string
		want  claudecode.PermissionMode
	}{
		{"bypassPermissions", claudecode.PermissionModeBypassPermissions},
		{"plan", claudecode.PermissionModePlan},
		{"acceptEdits", claudecode.PermissionModeAcceptEdits},
		{"", claudecode.PermissionModeDefault},
		{"unknown", claudecode.PermissionModeDefault},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := mapPermissionMode(tc.input)
			if got != tc.want {
				t.Errorf("mapPermissionMode(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestShouldFilterEnv(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"CLAUDECODE", true},
		{"CLAUDE_CODE_XYZ", true},
		{"CLAUDE_MANAGER_FOO", true},
		{"OTEL_TRACE", true},
		{"HOME", false},
		{"PATH", false},
		{"CLAUDE_API_KEY", false},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got := shouldFilterEnv(tc.key)
			if got != tc.want {
				t.Errorf("shouldFilterEnv(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

func TestFilteredEnv(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_TEST", "val")
	t.Setenv("OTEL_FOO", "bar")
	t.Setenv("MY_SAFE_VAR", "ok")

	env := filteredEnv()

	if _, exists := env["CLAUDECODE"]; exists {
		t.Error("CLAUDECODE should be filtered")
	}
	if _, exists := env["CLAUDE_CODE_TEST"]; exists {
		t.Error("CLAUDE_CODE_TEST should be filtered")
	}
	if _, exists := env["OTEL_FOO"]; exists {
		t.Error("OTEL_FOO should be filtered")
	}
	if v, exists := env["MY_SAFE_VAR"]; !exists || v != "ok" {
		t.Errorf("MY_SAFE_VAR = %q, want %q", v, "ok")
	}
}

func TestBuildOptionsBasic(t *testing.T) {
	b := NewClaudeBackend("/usr/bin/claude", 10)
	req := core.BackendRequest{
		WorkingDir:     "/home/user",
		Model:          "sonnet",
		PermissionMode: "plan",
		SystemPrompt:   "You are helpful.",
		SessionID:      "sess-123",
	}

	opts := buildOptions(b, req)

	// Verify opts is non-empty (contains at least partial streaming, CLI path,
	// cwd, model, system prompt, resume, permission mode, env, stderr callback)
	if len(opts) < 5 {
		t.Errorf("expected at least 5 options, got %d", len(opts))
	}
}

func TestBuildOptionsNoAgent(t *testing.T) {
	b := NewClaudeBackend("/usr/bin/claude", 10)
	req := core.BackendRequest{Prompt: "hello"}

	opts := buildOptions(b, req)

	// Should still produce options (partial streaming, permission mode, env, stderr)
	if len(opts) == 0 {
		t.Error("expected non-empty options even without agent")
	}
}

func TestBuildOptionsWithAgent(t *testing.T) {
	b := NewClaudeBackend("/usr/bin/claude", 10)
	req := core.BackendRequest{
		Prompt: "hello",
		Agent: &core.AgentDef{
			Name:        "coder",
			Description: "Code assistant",
			Prompt:      "You write code.",
			Tools:       []string{"Read", "Bash"},
		},
	}

	opts := buildOptions(b, req)

	// With agent, should have more options than without
	optsNoAgent := buildOptions(b, core.BackendRequest{Prompt: "hello"})
	if len(opts) <= len(optsNoAgent) {
		t.Errorf("agent options should increase count: with=%d, without=%d",
			len(opts), len(optsNoAgent))
	}
}

func TestBuildAgentOptionsTyped(t *testing.T) {
	agent := &core.AgentDef{
		Name:        "coder",
		Description: "Code assistant",
		Prompt:      "You write code.",
		Tools:       []string{"Read", "Bash"},
	}

	opts := buildAgentOptions(agent)
	if len(opts) == 0 {
		t.Error("expected non-empty agent options")
	}
}

func TestBuildAgentOptionsEmptyTools(t *testing.T) {
	agent := &core.AgentDef{
		Name:        "restricted",
		Description: "No tools",
		Prompt:      "You have no tools.",
		Tools:       []string{},
	}

	opts := buildAgentOptions(agent)
	if len(opts) == 0 {
		t.Error("expected non-empty agent options with empty tools")
	}
}

func TestBuildAgentOptionsNilTools(t *testing.T) {
	agent := &core.AgentDef{
		Name:        "default",
		Description: "Default tools",
		Prompt:      "You have default tools.",
		Tools:       nil,
	}

	opts := buildAgentOptions(agent)
	if len(opts) == 0 {
		t.Error("expected non-empty agent options with nil tools")
	}
}

func TestHandleMessageStreamEvent(t *testing.T) {
	deltaCh := make(chan core.StreamDelta, 10)
	event := &claudecode.StreamEvent{
		Event: map[string]any{
			"type": "content_block_delta",
			"delta": map[string]any{
				"text": "hello world",
			},
		},
	}

	result := handleMessage(event, deltaCh, nil)

	if result != nil {
		t.Error("stream event should not produce a result")
	}
	select {
	case delta := <-deltaCh:
		if delta.Text != "hello world" {
			t.Errorf("delta text = %q, want %q", delta.Text, "hello world")
		}
	default:
		t.Error("expected delta in channel")
	}
}

func TestHandleMessageStreamEventNonDelta(t *testing.T) {
	deltaCh := make(chan core.StreamDelta, 10)
	event := &claudecode.StreamEvent{
		Event: map[string]any{
			"type": "content_block_start",
		},
	}

	handleMessage(event, deltaCh, nil)

	select {
	case <-deltaCh:
		t.Error("non-delta event should not produce a delta")
	default:
	}
}

func TestHandleMessageResultMessage(t *testing.T) {
	deltaCh := make(chan core.StreamDelta, 10)
	cost := 0.05
	msg := &claudecode.ResultMessage{
		SessionID:    "sess-456",
		IsError:      false,
		NumTurns:     3,
		TotalCostUSD: &cost,
	}

	result := handleMessage(msg, deltaCh, nil)

	if result == nil {
		t.Fatal("result message should produce a result")
	}
	if result.SessionID != "sess-456" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-456")
	}
	if result.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want %f", result.CostUSD, 0.05)
	}
	if result.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want %d", result.NumTurns, 3)
	}
	if result.IsError {
		t.Error("should not be error")
	}
}

func TestHandleMessageErrorResult(t *testing.T) {
	deltaCh := make(chan core.StreamDelta, 10)
	errText := "something failed"
	msg := &claudecode.ResultMessage{
		SessionID: "sess-789",
		IsError:   true,
		Result:    &errText,
	}

	result := handleMessage(msg, deltaCh, nil)

	if result == nil {
		t.Fatal("error result message should produce a result")
	}
	if !result.IsError {
		t.Error("should be error")
	}
	if result.Error != "something failed" {
		t.Errorf("Error = %q, want %q", result.Error, "something failed")
	}
}

func TestExtractDeltaText(t *testing.T) {
	tests := []struct {
		name  string
		event map[string]any
		want  string
	}{
		{
			name:  "valid delta",
			event: map[string]any{"type": "content_block_delta", "delta": map[string]any{"text": "hi"}},
			want:  "hi",
		},
		{
			name:  "wrong type",
			event: map[string]any{"type": "content_block_start"},
			want:  "",
		},
		{
			name:  "no type",
			event: map[string]any{},
			want:  "",
		},
		{
			name:  "no delta field",
			event: map[string]any{"type": "content_block_delta"},
			want:  "",
		},
		{
			name:  "no text in delta",
			event: map[string]any{"type": "content_block_delta", "delta": map[string]any{}},
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &claudecode.StreamEvent{Event: tc.event}
			got := extractDeltaText(e)
			if got != tc.want {
				t.Errorf("extractDeltaText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewClaudeBackend(t *testing.T) {
	b := NewClaudeBackend("/path/to/claude", 15)
	if b.binary != "/path/to/claude" {
		t.Errorf("binary = %q, want %q", b.binary, "/path/to/claude")
	}
	if b.timeoutMinutes != 15 {
		t.Errorf("timeoutMinutes = %d, want %d", b.timeoutMinutes, 15)
	}
}
