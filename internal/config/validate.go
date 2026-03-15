package config

import (
	"fmt"
	"os"
	"regexp"
)

var botNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func validate(cfg *Config) error {
	if err := validateClaude(&cfg.Claude); err != nil {
		return err
	}
	return validateBots(cfg.Bots)
}

func validateClaude(c *ClaudeConfig) error {
	if c.Binary == "" {
		return fmt.Errorf("claude.binary is required")
	}
	return nil
}

func validateBots(bots map[string]BotConfig) error {
	if len(bots) == 0 {
		return fmt.Errorf("at least one bot is required")
	}

	if err := validateBotNames(bots); err != nil {
		return err
	}
	if err := validateDuplicateTokens(bots); err != nil {
		return err
	}

	for name, bot := range bots {
		if err := validateBot(name, &bot); err != nil {
			return err
		}
	}
	return nil
}

func validateBotNames(bots map[string]BotConfig) error {
	for name := range bots {
		if !botNamePattern.MatchString(name) {
			return fmt.Errorf(
				"invalid bot name %q: must match [a-z][a-z0-9_]*", name,
			)
		}
	}
	return nil
}

func validateDuplicateTokens(bots map[string]BotConfig) error {
	seen := make(map[string]string)
	for name, bot := range bots {
		if prev, exists := seen[bot.Token]; exists {
			return fmt.Errorf(
				"duplicate token: bots %q and %q share the same token",
				prev, name,
			)
		}
		seen[bot.Token] = name
	}
	return nil
}

func validateBot(name string, bot *BotConfig) error {
	if bot.Token == "" {
		return fmt.Errorf("bot %q: token is required", name)
	}
	if len(bot.Users) == 0 {
		return fmt.Errorf("bot %q: at least one user is required", name)
	}
	for uid, u := range bot.Users {
		if u.WorkingDir == "" {
			return fmt.Errorf(
				"bot %q, user %d: working_dir is required", name, uid,
			)
		}
	}
	return nil
}

// ValidatePaths checks that filesystem paths actually exist.
func ValidatePaths(cfg *Config) error {
	if err := checkFileExists(cfg.Claude.Binary, "claude.binary"); err != nil {
		return err
	}
	for name, bot := range cfg.Bots {
		for uid, u := range bot.Users {
			label := fmt.Sprintf("bot %q, user %d: working_dir", name, uid)
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
		return fmt.Errorf(
			"%s: path %q is a directory, not a file", label, path,
		)
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
