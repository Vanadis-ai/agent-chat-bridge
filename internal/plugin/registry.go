package plugin

import (
	"fmt"

	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

// Constructor creates a new plugin instance.
type Constructor func() core.Plugin

var registry = map[string]Constructor{}

// Register adds a plugin constructor to the registry.
// Called from init() in plugin implementation files.
// Panics on duplicate name (programming error, caught at init time).
func Register(name string, ctor Constructor) {
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("plugin %q already registered", name))
	}
	registry[name] = ctor
}

// Create instantiates a plugin by name. Returns error if unknown.
func Create(name string) (core.Plugin, error) {
	ctor, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown plugin: %q", name)
	}
	return ctor(), nil
}

// LoadPlugins creates and initializes enabled plugins from config.
// Returns error immediately on unknown plugin or Init failure (fail-fast).
// Preserves config order in the returned slice.
func LoadPlugins(configs []config.PluginConfig) ([]core.Plugin, error) {
	var plugins []core.Plugin
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		p, err := Create(cfg.Name)
		if err != nil {
			return nil, fmt.Errorf("create plugin %q: %w", cfg.Name, err)
		}
		if err := p.Init(cfg.Config); err != nil {
			return nil, fmt.Errorf("init plugin %q: %w", cfg.Name, err)
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}
