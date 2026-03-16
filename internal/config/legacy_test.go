package config

import "testing"


func TestConvertLegacyBackend(t *testing.T) {
	cfg := &Config{
		Claude: ClaudeConfig{
			Binary:         "/usr/bin/claude",
			TimeoutMinutes: 15,
		},
		TelegramBots: map[string]BotConfig{},
	}
	convertLegacy(cfg)

	bc, ok := cfg.Backends["default"]
	if !ok {
		t.Fatal("expected Backends[default]")
	}
	if bc.Type != "claude" {
		t.Errorf("Type = %q, want claude", bc.Type)
	}
	if bc.Binary != "/usr/bin/claude" {
		t.Errorf("Binary = %q", bc.Binary)
	}
	if bc.TimeoutMinutes != 15 {
		t.Errorf("TimeoutMinutes = %d", bc.TimeoutMinutes)
	}
}

func TestConvertLegacyFrontends(t *testing.T) {
	cfg := &Config{
		Claude: ClaudeConfig{Binary: "/usr/bin/claude"},
		TelegramBots: map[string]BotConfig{
			"mybot": {
				Token:              "tok123",
				Model:              "opus",
				PermissionMode:     "plan",
				AppendSystemPrompt: "Be helpful.",
				Agent:              &AgentConfig{Name: "a", Description: "d", Prompt: "p"},
				Sessions:           "mybot_sessions.json",
				Users:              map[int64]*UserConfig{1: {WorkingDir: "/work"}},
			},
		},
	}
	convertLegacy(cfg)

	fc, ok := cfg.Frontends["mybot"]
	if !ok {
		t.Fatal("expected Frontends[mybot]")
	}
	if fc.Type != "telegram" {
		t.Errorf("Type = %q", fc.Type)
	}
	if fc.Backend != "default" {
		t.Errorf("Backend = %q", fc.Backend)
	}
	if fc.Token != "tok123" {
		t.Errorf("Token = %q", fc.Token)
	}
	if fc.Model != "opus" {
		t.Errorf("Model = %q", fc.Model)
	}
	if fc.PermissionMode != "plan" {
		t.Errorf("PermissionMode = %q", fc.PermissionMode)
	}
	if fc.AppendSystemPrompt != "Be helpful." {
		t.Errorf("AppendSystemPrompt = %q", fc.AppendSystemPrompt)
	}
	if fc.Sessions != "mybot_sessions.json" {
		t.Errorf("Sessions = %q", fc.Sessions)
	}
	if fc.Agent == nil || fc.Agent.Name != "a" {
		t.Errorf("Agent = %v", fc.Agent)
	}
	if fc.Users[1] == nil || fc.Users[1].WorkingDir != "/work" {
		t.Error("Users not preserved")
	}
}

func TestConvertLegacyPluginsEmpty(t *testing.T) {
	cfg := &Config{
		Claude:       ClaudeConfig{Binary: "/usr/bin/claude"},
		TelegramBots: map[string]BotConfig{},
	}
	convertLegacy(cfg)

	if cfg.Plugins == nil {
		t.Error("Plugins should not be nil")
	}
	if len(cfg.Plugins) != 0 {
		t.Errorf("Plugins len = %d, want 0", len(cfg.Plugins))
	}
}

func TestConvertLegacyMultipleBots(t *testing.T) {
	cfg := &Config{
		Claude: ClaudeConfig{Binary: "/usr/bin/claude"},
		TelegramBots: map[string]BotConfig{
			"bot_a": {Token: "a", Users: map[int64]*UserConfig{1: {WorkingDir: "/a"}}},
			"bot_b": {Token: "b", Users: map[int64]*UserConfig{2: {WorkingDir: "/b"}}},
		},
	}
	convertLegacy(cfg)

	if len(cfg.Frontends) != 2 {
		t.Fatalf("Frontends count = %d, want 2", len(cfg.Frontends))
	}
	if _, ok := cfg.Frontends["bot_a"]; !ok {
		t.Error("missing bot_a frontend")
	}
	if _, ok := cfg.Frontends["bot_b"]; !ok {
		t.Error("missing bot_b frontend")
	}
}
