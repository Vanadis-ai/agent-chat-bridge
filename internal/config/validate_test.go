package config

import (
	"strings"
	"testing"
)

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
	if !strings.Contains(err.Error(), "claude.binary") {
		t.Errorf("error should mention claude.binary, got: %v", err)
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
		t.Fatal("expected error for empty bots")
	}
	if !strings.Contains(err.Error(), "at least one bot") {
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
	if !strings.Contains(err.Error(), "mybot") || !strings.Contains(err.Error(), "token") {
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
	if !strings.Contains(errStr, "mybot") {
		t.Errorf("should mention bot name, got: %v", err)
	}
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
		t.Fatal("expected error for invalid bot name")
	}
	if !strings.Contains(err.Error(), "My-Bot") {
		t.Errorf("got: %v", err)
	}
}

func TestAgentMissingName(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    agent:
      description: "A translator"
      prompt: "Translate text"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing agent name")
	}
	if !strings.Contains(err.Error(), "agent.name") {
		t.Errorf("got: %v", err)
	}
}

func TestAgentMissingDescription(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    agent:
      name: "translator"
      prompt: "Translate text"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing agent description")
	}
	if !strings.Contains(err.Error(), "agent.description") {
		t.Errorf("got: %v", err)
	}
}

func TestAgentMissingPrompt(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    agent:
      name: "translator"
      description: "A translator"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing agent prompt")
	}
	if !strings.Contains(err.Error(), "agent.prompt") {
		t.Errorf("got: %v", err)
	}
}

func TestAgentAndSystemPromptMutuallyExclusive(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    append_system_prompt: "Extra instructions"
    agent:
      name: "translator"
      description: "A translator"
      prompt: "Translate text"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for agent + append_system_prompt")
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
    model: "sonnet"
    agent:
      name: "translator"
      description: "Translates text between languages"
      prompt: "You are a translator. Only translate text."
      tools: []
    users:
      1:
        working_dir: "/tmp"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bot := cfg.TelegramBots["translator"]
	if bot.Agent == nil {
		t.Fatal("expected agent to be set")
	}
	if bot.Agent.Name != "translator" {
		t.Errorf("agent.name = %q", bot.Agent.Name)
	}
	if bot.Agent.Description != "Translates text between languages" {
		t.Errorf("agent.description = %q", bot.Agent.Description)
	}
	if bot.Agent.Tools == nil {
		t.Fatal("expected agent.tools to be non-nil empty slice")
	}
	if len(bot.Agent.Tools) != 0 {
		t.Errorf("agent.tools = %v, want empty", bot.Agent.Tools)
	}
}

func TestAgentWithTools(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  mybot:
    token: "tok"
    agent:
      name: "reader"
      description: "Reads files"
      prompt: "You can only read files."
      tools:
        - Read
        - Glob
    users:
      1:
        working_dir: "/tmp"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tools := cfg.TelegramBots["mybot"].Agent.Tools
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0] != "Read" || tools[1] != "Glob" {
		t.Errorf("tools = %v", tools)
	}
}
