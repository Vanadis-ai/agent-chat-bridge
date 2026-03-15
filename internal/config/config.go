package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level application configuration.
type Config struct {
	Claude ClaudeConfig         `yaml:"claude"`
	TelegramBots map[string]BotConfig `yaml:"telegram_bots"`
}

// ClaudeConfig holds settings for the Claude CLI binary.
type ClaudeConfig struct {
	Binary         string `yaml:"binary"`
	TimeoutMinutes int    `yaml:"timeout_minutes"`
	MaxConcurrent  int    `yaml:"max_concurrent"`
}

// AgentConfig defines a custom Claude agent for the bot.
type AgentConfig struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Prompt      string   `yaml:"prompt"`
	Tools       []string `yaml:"tools"`
}

// BotConfig defines a single Telegram bot.
type BotConfig struct {
	Token              string                `yaml:"token"`
	Model              string                `yaml:"model"`
	PermissionMode     string                `yaml:"permission_mode"`
	AppendSystemPrompt string                `yaml:"append_system_prompt"`
	Agent              *AgentConfig          `yaml:"agent"`
	Sessions           string                `yaml:"sessions"`
	Users              map[int64]*UserConfig `yaml:"users"`
}

// UserConfig holds per-user settings within a bot.
type UserConfig struct {
	WorkingDir string `yaml:"working_dir"`
	VoiceDir   string `yaml:"voice_dir"`
	FilesDir   string `yaml:"files_dir"`
}

// Load reads and validates a config file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	applyDefaults(&cfg)
	resolvePaths(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

