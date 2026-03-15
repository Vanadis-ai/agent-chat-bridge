package config

import "path/filepath"

func applyDefaults(cfg *Config) {
	if cfg.Claude.TimeoutMinutes == 0 {
		cfg.Claude.TimeoutMinutes = 10
	}
	if cfg.Claude.MaxConcurrent == 0 {
		cfg.Claude.MaxConcurrent = 5
	}

	for name, bot := range cfg.TelegramBots {
		if bot.PermissionMode == "" {
			bot.PermissionMode = "bypassPermissions"
		}
		if bot.Sessions == "" {
			bot.Sessions = name + "_sessions.json"
		}
		applyUserDefaults(bot.Users)
		cfg.TelegramBots[name] = bot
	}
}

func applyUserDefaults(users map[int64]*UserConfig) {
	for _, u := range users {
		if u.VoiceDir == "" {
			u.VoiceDir = "voice_inbox"
		}
		if u.FilesDir == "" {
			u.FilesDir = "files_inbox"
		}
	}
}

func resolvePaths(cfg *Config) {
	for _, bot := range cfg.TelegramBots {
		for _, u := range bot.Users {
			u.VoiceDir = resolveDir(u.WorkingDir, u.VoiceDir)
			u.FilesDir = resolveDir(u.WorkingDir, u.FilesDir)
		}
	}
}

func resolveDir(base, dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(base, dir)
}
