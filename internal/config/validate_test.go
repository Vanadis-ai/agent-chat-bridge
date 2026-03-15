package config

import (
	"strings"
	"testing"
)

func TestMissingClaudeBinary(t *testing.T) {
	yaml := `
claude: {}
bots:
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
bots: {}
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
bots:
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
bots:
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
bots:
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
bots:
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
bots:
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
