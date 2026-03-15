package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

func validMinimalYAML() string {
	return `
claude:
  binary: "/usr/local/bin/claude"
bots:
  obsidian:
    token: "tok123"
    users:
      123456789:
        working_dir: "/home/user/vault"
`
}

func TestValidFullConfig(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
  timeout_minutes: 15
  max_concurrent: 3
bots:
  obsidian:
    token: "tok_obs"
    model: "opus"
    permission_mode: "plan"
    append_system_prompt: "You work with Obsidian."
    sessions: "custom_sessions.json"
    users:
      111:
        working_dir: "/home/user/vault"
        voice_dir: "/tmp/voice"
        files_dir: "/tmp/files"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertClaudeConfig(t, cfg)
	assertBotConfig(t, cfg)
}

func assertClaudeConfig(t *testing.T, cfg *Config) {
	t.Helper()
	if cfg.Claude.Binary != "/usr/local/bin/claude" {
		t.Errorf("binary = %q", cfg.Claude.Binary)
	}
	if cfg.Claude.TimeoutMinutes != 15 {
		t.Errorf("timeout = %d, want 15", cfg.Claude.TimeoutMinutes)
	}
	if cfg.Claude.MaxConcurrent != 3 {
		t.Errorf("max_concurrent = %d, want 3", cfg.Claude.MaxConcurrent)
	}
}

func assertBotConfig(t *testing.T, cfg *Config) {
	t.Helper()
	bot := cfg.Bots["obsidian"]
	if bot.Token != "tok_obs" {
		t.Errorf("token = %q", bot.Token)
	}
	if bot.Model != "opus" {
		t.Errorf("model = %q", bot.Model)
	}
	if bot.PermissionMode != "plan" {
		t.Errorf("permission_mode = %q", bot.PermissionMode)
	}
	if bot.Sessions != "custom_sessions.json" {
		t.Errorf("sessions = %q", bot.Sessions)
	}

	u := bot.Users[111]
	if u.WorkingDir != "/home/user/vault" {
		t.Errorf("working_dir = %q", u.WorkingDir)
	}
	if u.VoiceDir != "/tmp/voice" {
		t.Errorf("voice_dir = %q", u.VoiceDir)
	}
	if u.FilesDir != "/tmp/files" {
		t.Errorf("files_dir = %q", u.FilesDir)
	}
}

func TestValidMinimalConfig(t *testing.T) {
	cfg, err := Load(writeConfig(t, validMinimalYAML()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Claude.TimeoutMinutes != 10 {
		t.Errorf("timeout = %d, want 10", cfg.Claude.TimeoutMinutes)
	}
	if cfg.Claude.MaxConcurrent != 5 {
		t.Errorf("max_concurrent = %d, want 5", cfg.Claude.MaxConcurrent)
	}

	bot := cfg.Bots["obsidian"]
	if bot.PermissionMode != "bypassPermissions" {
		t.Errorf("permission_mode = %q", bot.PermissionMode)
	}
	if bot.Sessions != "obsidian_sessions.json" {
		t.Errorf("sessions = %q", bot.Sessions)
	}

	u := bot.Users[123456789]
	if u.VoiceDir != "/home/user/vault/voice_inbox" {
		t.Errorf("voice_dir = %q", u.VoiceDir)
	}
	if u.FilesDir != "/home/user/vault/files_inbox" {
		t.Errorf("files_dir = %q", u.FilesDir)
	}
}

func TestMissingConfigFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "/nonexistent/path/config.yaml") {
		t.Errorf("error should contain file path, got: %v", err)
	}
}

func TestInvalidYAML(t *testing.T) {
	content := `claude:
  binary: "/usr/local/bin/claude
  broken yaml here`
	_, err := Load(writeConfig(t, content))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "YAML") {
		t.Errorf("error should mention YAML, got: %v", err)
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("TELEBRIDGE_OBSIDIAN_TOKEN", "override_token")
	cfg, err := Load(writeConfig(t, validMinimalYAML()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Bots["obsidian"].Token != "override_token" {
		t.Errorf("token = %q, want override_token", cfg.Bots["obsidian"].Token)
	}
}

func TestRelativePathResolution(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
bots:
  test:
    token: "tok"
    users:
      1:
        working_dir: "/home/user"
        voice_dir: "inbox"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Bots["test"].Users[1].VoiceDir != "/home/user/inbox" {
		t.Errorf("voice_dir = %q", cfg.Bots["test"].Users[1].VoiceDir)
	}
}

func TestAbsolutePathPreserved(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
bots:
  test:
    token: "tok"
    users:
      1:
        working_dir: "/home/user"
        voice_dir: "/tmp/voice"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Bots["test"].Users[1].VoiceDir != "/tmp/voice" {
		t.Errorf("voice_dir = %q", cfg.Bots["test"].Users[1].VoiceDir)
	}
}

func TestDefaultSessionsFile(t *testing.T) {
	cfg, err := Load(writeConfig(t, validMinimalYAML()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Bots["obsidian"].Sessions != "obsidian_sessions.json" {
		t.Errorf("sessions = %q", cfg.Bots["obsidian"].Sessions)
	}
}
