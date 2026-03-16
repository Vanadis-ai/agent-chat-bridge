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

func validMinimalLegacyYAML() string {
	return `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
  obsidian:
    token: "tok123"
    users:
      123456789:
        working_dir: "/home/user/vault"
`
}

func TestLegacyFullConfig(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
  timeout_minutes: 15
telegram_bots:
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

	bc := cfg.Backends["default"]
	if bc.Type != "claude" {
		t.Errorf("backend type = %q, want claude", bc.Type)
	}
	if bc.Binary != "/usr/local/bin/claude" {
		t.Errorf("binary = %q", bc.Binary)
	}
	if bc.TimeoutMinutes != 15 {
		t.Errorf("timeout = %d, want 15", bc.TimeoutMinutes)
	}

	fc := cfg.Frontends["obsidian"]
	if fc.Type != "telegram" {
		t.Errorf("frontend type = %q", fc.Type)
	}
	if fc.Token != "tok_obs" {
		t.Errorf("token = %q", fc.Token)
	}
	if fc.Model != "opus" {
		t.Errorf("model = %q", fc.Model)
	}
	if fc.Sessions != "custom_sessions.json" {
		t.Errorf("sessions = %q", fc.Sessions)
	}

	u := fc.Users[111]
	if u.VoiceDir != "/tmp/voice" {
		t.Errorf("voice_dir = %q", u.VoiceDir)
	}
}

func TestLegacyMinimalConfig(t *testing.T) {
	cfg, err := Load(writeConfig(t, validMinimalLegacyYAML()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bc := cfg.Backends["default"]
	if bc.TimeoutMinutes != 10 {
		t.Errorf("timeout = %d, want 10", bc.TimeoutMinutes)
	}

	fc := cfg.Frontends["obsidian"]
	if fc.PermissionMode != "bypassPermissions" {
		t.Errorf("permission_mode = %q", fc.PermissionMode)
	}
	if fc.Sessions != "obsidian_sessions.json" {
		t.Errorf("sessions = %q", fc.Sessions)
	}

	u := fc.Users[123456789]
	if u.VoiceDir != "/home/user/vault/voice_inbox" {
		t.Errorf("voice_dir = %q", u.VoiceDir)
	}
	if u.FilesDir != "/home/user/vault/files_inbox" {
		t.Errorf("files_dir = %q", u.FilesDir)
	}
}

func TestNewFormatConfig(t *testing.T) {
	yaml := `
backends:
  claude_main:
    type: claude
    binary: "/usr/bin/claude"
    timeout_minutes: 20
frontends:
  mybot:
    type: telegram
    backend: claude_main
    token: "tok_new"
    model: "sonnet"
    users:
      999:
        working_dir: "/work"
plugins:
  - name: request_logger
    enabled: true
    config:
      log_file: "/tmp/log"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Backends) != 1 {
		t.Fatalf("backends count = %d, want 1", len(cfg.Backends))
	}
	bc := cfg.Backends["claude_main"]
	if bc.Type != "claude" {
		t.Errorf("backend type = %q", bc.Type)
	}
	if bc.TimeoutMinutes != 20 {
		t.Errorf("timeout = %d", bc.TimeoutMinutes)
	}

	fc := cfg.Frontends["mybot"]
	if fc.Backend != "claude_main" {
		t.Errorf("backend ref = %q", fc.Backend)
	}
	if fc.Token != "tok_new" {
		t.Errorf("token = %q", fc.Token)
	}

	if len(cfg.Plugins) != 1 {
		t.Fatalf("plugins count = %d, want 1", len(cfg.Plugins))
	}
	if cfg.Plugins[0].Name != "request_logger" {
		t.Errorf("plugin name = %q", cfg.Plugins[0].Name)
	}
}

func TestMissingConfigFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
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

func TestRelativePathResolution(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
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
	if cfg.Frontends["test"].Users[1].VoiceDir != "/home/user/inbox" {
		t.Errorf("voice_dir = %q", cfg.Frontends["test"].Users[1].VoiceDir)
	}
}

func TestAbsolutePathPreserved(t *testing.T) {
	yaml := `
claude:
  binary: "/usr/local/bin/claude"
telegram_bots:
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
	if cfg.Frontends["test"].Users[1].VoiceDir != "/tmp/voice" {
		t.Errorf("voice_dir = %q", cfg.Frontends["test"].Users[1].VoiceDir)
	}
}

func TestNewFormatDefaults(t *testing.T) {
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
        working_dir: "/work"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bc := cfg.Backends["b1"]
	if bc.TimeoutMinutes != 10 {
		t.Errorf("backend timeout = %d, want 10", bc.TimeoutMinutes)
	}

	fc := cfg.Frontends["f1"]
	if fc.PermissionMode != "bypassPermissions" {
		t.Errorf("permission_mode = %q", fc.PermissionMode)
	}
	if fc.Sessions != "f1_sessions.json" {
		t.Errorf("sessions = %q", fc.Sessions)
	}
}
