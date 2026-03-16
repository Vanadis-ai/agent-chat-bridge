package config

const (
	BackendTypeClaude    = "claude"
	FrontendTypeTelegram = "telegram"
)

// convertLegacy transforms legacy Config fields (Claude + TelegramBots) into
// the new format (Backends + Frontends + Plugins).
// Called by Load() after initial unmarshal when legacy format is detected.
func convertLegacy(cfg *Config) {
	cfg.Backends = map[string]BackendConfig{
		"default": {
			Type:           BackendTypeClaude,
			Binary:         cfg.Claude.Binary,
			TimeoutMinutes: cfg.Claude.TimeoutMinutes,
		},
	}

	cfg.Frontends = make(map[string]FrontendConfig, len(cfg.TelegramBots))
	for name, bot := range cfg.TelegramBots {
		cfg.Frontends[name] = FrontendConfig{
			Type:               FrontendTypeTelegram,
			Backend:            "default",
			Token:              bot.Token,
			Model:              bot.Model,
			PermissionMode:     bot.PermissionMode,
			AppendSystemPrompt: bot.AppendSystemPrompt,
			Agent:              bot.Agent,
			Sessions:           bot.Sessions,
			Users:              bot.Users,
		}
	}

	if cfg.Plugins == nil {
		cfg.Plugins = []PluginConfig{}
	}
}
