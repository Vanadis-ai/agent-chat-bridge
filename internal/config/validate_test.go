package config

import (
	"strings"
	"testing"
)

// Legacy format validation tests (through Load which converts to new format)

func TestMissingClaudeBinary(t *testing.T) {
	yaml := `
claude: {}
telegram_bots:
  test:
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "binary") {
		t.Errorf("error should mention binary, got: %v", err)
	}
}

func TestEmptyBots(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots: {}
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for empty frontends")
	}
	if !strings.Contains(err.Error(), "at least one frontend") {
		t.Errorf("got: %v", err)
	}
}

func TestBotWithoutToken(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("got: %v", err)
	}
}

func TestBotWithoutUsers(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    users: {}
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for empty users")
	}
	if !strings.Contains(err.Error(), "at least one user") {
		t.Errorf("got: %v", err)
	}
}

func TestUserWithoutWorkingDir(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    users:
      42:
        voice_dir: "/tmp/voice"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing working_dir")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "42") {
		t.Errorf("should mention user ID, got: %v", err)
	}
	if !strings.Contains(errStr, "working_dir") {
		t.Errorf("should mention working_dir, got: %v", err)
	}
}

func TestDuplicateTokens(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  bot_a:
    token: "same_token"
    users:
      1:
        working_dir: "/tmp/a"
  bot_b:
    token: "same_token"
    users:
      1:
        working_dir: "/tmp/b"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for duplicate tokens")
	}
	if !strings.Contains(err.Error(), "duplicate token") {
		t.Errorf("got: %v", err)
	}
}

func TestInvalidBotName(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  My-Bot:
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
	if !strings.Contains(err.Error(), "My-Bot") {
		t.Errorf("got: %v", err)
	}
}

func TestAgentMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		agent   string
		wantErr string
	}{
		{"no name", `agent: {description: "d", prompt: "p"}`, "agent.name"},
		{"no desc", `agent: {name: "n", prompt: "p"}`, "agent.description"},
		{"no prompt", `agent: {name: "n", description: "d"}`, "agent.prompt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    ` + tc.agent + `
    users:
      1:
        working_dir: "/tmp"
`
			_, err := Load(writeConfig(t, yaml))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("got: %v, want substring: %s", err, tc.wantErr)
			}
		})
	}
}

func TestAgentAndSystemPromptMutuallyExclusive(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    append_system_prompt: "Extra"
    agent:
      name: "t"
      description: "d"
      prompt: "p"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("got: %v", err)
	}
}

func TestValidAgentConfig(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  translator:
    token: "tok"
    agent:
      name: "translator"
      description: "Translates text"
      prompt: "Translate."
      tools: []
    users:
      1:
        working_dir: "/tmp"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fc := cfg.Frontends["translator"]
	if fc.Agent == nil {
		t.Fatal("expected agent")
	}
	if fc.Agent.Name != "translator" {
		t.Errorf("agent.name = %q", fc.Agent.Name)
	}
	if fc.Agent.Tools == nil || len(fc.Agent.Tools) != 0 {
		t.Errorf("agent.tools = %v, want empty", fc.Agent.Tools)
	}
}

// New format validation tests

func TestNewFormatBackendMissingType(t *testing.T) {
	yaml := `
backends:
  b1:
    binary: "/usr/bin/claude"
frontends:
  f1:
    type: telegram
    backend: b1
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing backend type")
	}
	if !strings.Contains(err.Error(), "type is required") {
		t.Errorf("got: %v", err)
	}
}

func TestNewFormatFrontendBadBackendRef(t *testing.T) {
	yaml := `
backends:
  b1:
    type: claude
    binary: "/usr/bin/claude"
frontends:
  f1:
    type: telegram
    backend: nonexistent
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for bad backend ref")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("got: %v", err)
	}
}

func TestNewFormatPluginDuplicateNames(t *testing.T) {
	yaml := `
backends:
  b1:
    type: claude
    binary: "/usr/bin/claude"
frontends:
  f1:
    type: telegram
    backend: b1
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
plugins:
  - name: logger
    enabled: true
  - name: logger
    enabled: true
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for duplicate plugin names")
	}
	if !strings.Contains(err.Error(), "duplicate plugin") {
		t.Errorf("got: %v", err)
	}
}

func TestNewFormatPluginMissingName(t *testing.T) {
	yaml := `
backends:
  b1:
    type: claude
    binary: "/usr/bin/claude"
frontends:
  f1:
    type: telegram
    backend: b1
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
plugins:
  - enabled: true
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing plugin name")
	}
	if !strings.Contains(err.Error(), "plugin name") {
		t.Errorf("got: %v", err)
	}
}
