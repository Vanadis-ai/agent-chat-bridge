package config

import "path/filepath"

func applyDefaults(cfg *Config) {
	applyBackendDefaults(cfg.Backends)
	applyFrontendDefaults(cfg.Frontends)
}

func applyBackendDefaults(backends map[string]BackendConfig) {
	for name, bc := range backends {
		if bc.TimeoutMinutes == 0 {
			bc.TimeoutMinutes = 10
		}
		backends[name] = bc
	}
}

func applyFrontendDefaults(frontends map[string]FrontendConfig) {
	for name, fc := range frontends {
		if fc.PermissionMode == "" {
			fc.PermissionMode = "bypassPermissions"
		}
		if fc.Sessions == "" {
			fc.Sessions = name + "_sessions.json"
		}
		applyUserDefaults(fc.Users)
		frontends[name] = fc
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
	for _, fc := range cfg.Frontends {
		for _, u := range fc.Users {
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
