package config

import (
	"fmt"
	"os"
	"regexp"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func validate(cfg *Config) error {
	if err := validateBackends(cfg.Backends); err != nil {
		return err
	}
	if err := validateFrontends(cfg.Frontends, cfg.Backends); err != nil {
		return err
	}
	return validatePlugins(cfg.Plugins)
}

func validateBackends(backends map[string]BackendConfig) error {
	if len(backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}
	for name, bc := range backends {
		if bc.Type == "" {
			return fmt.Errorf("backend %q: type is required", name)
		}
		switch bc.Type {
		case BackendTypeClaude:
			if bc.Binary == "" {
				return fmt.Errorf("backend %q: binary is required for claude type", name)
			}
		default:
			return fmt.Errorf("backend %q: unknown type %q (supported: claude)", name, bc.Type)
		}
	}
	return nil
}

func validateFrontends(frontends map[string]FrontendConfig, backends map[string]BackendConfig) error {
	if len(frontends) == 0 {
		return fmt.Errorf("at least one frontend is required")
	}
	if err := validateFrontendNames(frontends); err != nil {
		return err
	}
	if err := validateFrontendTokens(frontends); err != nil {
		return err
	}
	for name, fc := range frontends {
		if err := validateFrontend(name, &fc, backends); err != nil {
			return err
		}
	}
	return nil
}

func validateFrontendNames(frontends map[string]FrontendConfig) error {
	for name := range frontends {
		if !namePattern.MatchString(name) {
			return fmt.Errorf("invalid frontend name %q: must match [a-z][a-z0-9_]*", name)
		}
	}
	return nil
}

func validateFrontendTokens(frontends map[string]FrontendConfig) error {
	seen := make(map[string]string)
	for name, fc := range frontends {
		if fc.Token == "" {
			continue
		}
		if prev, exists := seen[fc.Token]; exists {
			return fmt.Errorf("duplicate token: frontends %q and %q share the same token", prev, name)
		}
		seen[fc.Token] = name
	}
	return nil
}

func validateFrontend(name string, fc *FrontendConfig, backends map[string]BackendConfig) error {
	if fc.Type == "" {
		return fmt.Errorf("frontend %q: type is required", name)
	}
	if fc.Backend == "" {
		return fmt.Errorf("frontend %q: backend is required", name)
	}
	if _, ok := backends[fc.Backend]; !ok {
		return fmt.Errorf("frontend %q: backend %q not found", name, fc.Backend)
	}
	switch fc.Type {
	case FrontendTypeTelegram:
		return validateTelegramFrontend(name, fc)
	default:
		return fmt.Errorf("frontend %q: unknown type %q (supported: telegram)", name, fc.Type)
	}
}

func validateTelegramFrontend(name string, fc *FrontendConfig) error {
	if fc.Token == "" {
		return fmt.Errorf("frontend %q: token is required", name)
	}
	if fc.Agent != nil && fc.AppendSystemPrompt != "" {
		return fmt.Errorf("frontend %q: agent and append_system_prompt are mutually exclusive", name)
	}
	if fc.Agent != nil {
		if err := validateAgent(name, fc.Agent); err != nil {
			return err
		}
	}
	if len(fc.Users) == 0 {
		return fmt.Errorf("frontend %q: at least one user is required", name)
	}
	for uid, u := range fc.Users {
		if u.WorkingDir == "" {
			return fmt.Errorf("frontend %q, user %d: working_dir is required", name, uid)
		}
	}
	return nil
}

func validateAgent(name string, agent *AgentConfig) error {
	if agent.Name == "" {
		return fmt.Errorf("frontend %q: agent.name is required", name)
	}
	if agent.Description == "" {
		return fmt.Errorf("frontend %q: agent.description is required", name)
	}
	if agent.Prompt == "" {
		return fmt.Errorf("frontend %q: agent.prompt is required", name)
	}
	return nil
}

func validatePlugins(plugins []PluginConfig) error {
	seen := make(map[string]bool)
	for _, p := range plugins {
		if p.Name == "" {
			return fmt.Errorf("plugin name is required")
		}
		if seen[p.Name] {
			return fmt.Errorf("duplicate plugin name: %q", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// ValidatePaths checks that filesystem paths actually exist.
func ValidatePaths(cfg *Config) error {
	for name, bc := range cfg.Backends {
		if bc.Type == BackendTypeClaude {
			label := fmt.Sprintf("backend %q: binary", name)
			if err := checkFileExists(bc.Binary, label); err != nil {
				return err
			}
		}
	}
	for name, fc := range cfg.Frontends {
		for uid, u := range fc.Users {
			label := fmt.Sprintf("frontend %q, user %d: working_dir", name, uid)
			if err := checkDirExists(u.WorkingDir, label); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkFileExists(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: path %q does not exist", label, path)
	}
	if info.IsDir() {
		return fmt.Errorf("%s: path %q is a directory, not a file", label, path)
	}
	return nil
}

func checkDirExists(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: path %q does not exist", label, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: path %q is not a directory", label, path)
	}
	return nil
}
