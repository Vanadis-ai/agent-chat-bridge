package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

// testPlugin is a minimal plugin for registry tests.
type testPlugin struct {
	initErr error
	inited  bool
}

func (p *testPlugin) Init(cfg map[string]any) error {
	p.inited = true
	return p.initErr
}

func (p *testPlugin) HandleMessage(_ context.Context, _ core.ChatMessage, _ chan<- core.StreamDelta) (*core.Response, error) {
	return nil, nil
}

func TestRegisterAndCreate(t *testing.T) {
	Register("test_basic", func() core.Plugin { return &testPlugin{} })

	p, err := Create("test_basic")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if p == nil {
		t.Fatal("plugin should not be nil")
	}
	if _, ok := p.(*testPlugin); !ok {
		t.Errorf("plugin type = %T, want *testPlugin", p)
	}
}

func TestCreateUnknownName(t *testing.T) {
	_, err := Create("nonexistent_plugin_xyz")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
	if !containsString(err.Error(), "nonexistent_plugin_xyz") {
		t.Errorf("error = %q, want to contain plugin name", err.Error())
	}
}

func TestLoadPluginsOrder(t *testing.T) {
	Register("test_order_a", func() core.Plugin { return &testPlugin{} })
	Register("test_order_b", func() core.Plugin { return &testPlugin{} })

	plugins, err := LoadPlugins([]config.PluginConfig{
		{Name: "test_order_a", Enabled: true},
		{Name: "test_order_b", Enabled: true},
	})
	if err != nil {
		t.Fatalf("LoadPlugins error: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("len = %d, want 2", len(plugins))
	}
	for _, p := range plugins {
		tp := p.(*testPlugin)
		if !tp.inited {
			t.Error("plugin should be initialized")
		}
	}
}

func TestLoadPluginsSkipsDisabled(t *testing.T) {
	Register("test_disabled", func() core.Plugin { return &testPlugin{} })

	plugins, err := LoadPlugins([]config.PluginConfig{
		{Name: "test_disabled", Enabled: false},
	})
	if err != nil {
		t.Fatalf("LoadPlugins error: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("len = %d, want 0 for disabled plugin", len(plugins))
	}
}

func TestLoadPluginsEmptyConfig(t *testing.T) {
	plugins, err := LoadPlugins(nil)
	if err != nil {
		t.Fatalf("LoadPlugins nil error: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("len = %d, want 0 for nil config", len(plugins))
	}

	plugins, err = LoadPlugins([]config.PluginConfig{})
	if err != nil {
		t.Fatalf("LoadPlugins empty error: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("len = %d, want 0 for empty config", len(plugins))
	}
}

func TestLoadPluginsUnknownPlugin(t *testing.T) {
	_, err := LoadPlugins([]config.PluginConfig{
		{Name: "totally_unknown_plugin", Enabled: true},
	})
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}

func TestLoadPluginsInitError(t *testing.T) {
	Register("test_init_fail", func() core.Plugin {
		return &testPlugin{initErr: errors.New("init failed")}
	})

	_, err := LoadPlugins([]config.PluginConfig{
		{Name: "test_init_fail", Enabled: true},
	})
	if err == nil {
		t.Fatal("expected error from Init failure")
	}
	if !containsString(err.Error(), "init failed") {
		t.Errorf("error = %q, want to contain 'init failed'", err.Error())
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
