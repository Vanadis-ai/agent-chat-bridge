package config

import (
	"strings"
	"testing"
)

// New format validation tests

func TestNewFormatBackendMissingType(t *testing.T) {
	yaml := `
backends:
  b1:
    binary: "/usr/bin/claude"
frontends:
  f1:
    type: telegram
    backend: b1
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing backend type")
	}
	if !strings.Contains(err.Error(), "type is required") {
		t.Errorf("got: %v", err)
	}
}

func TestNewFormatFrontendBadBackendRef(t *testing.T) {
	yaml := `
backends:
  b1:
    type: claude
    binary: "/usr/bin/claude"
frontends:
  f1:
    type: telegram
    backend: nonexistent
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for bad backend ref")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("got: %v", err)
	}
}

func TestNewFormatPluginDuplicateNames(t *testing.T) {
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
        working_dir: "/tmp"
plugins:
  - name: logger
    enabled: true
  - name: logger
    enabled: true
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for duplicate plugin names")
	}
	if !strings.Contains(err.Error(), "duplicate plugin") {
		t.Errorf("got: %v", err)
	}
}

func TestNewFormatPluginMissingName(t *testing.T) {
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
        working_dir: "/tmp"
plugins:
  - enabled: true
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing plugin name")
	}
	if !strings.Contains(err.Error(), "plugin name") {
		t.Errorf("got: %v", err)
	}
}

func TestNewFormatUnknownBackendType(t *testing.T) {
	yaml := `
backends:
  b1:
    type: openai
    binary: "/usr/bin/openai"
frontends:
  f1:
    type: telegram
    backend: b1
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for unknown backend type")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("got: %v", err)
	}
}

func TestNewFormatUnknownFrontendType(t *testing.T) {
	yaml := `
backends:
  b1:
    type: claude
    binary: "/usr/bin/claude"
frontends:
  f1:
    type: discord
    backend: b1
    token: "tok"
    users:
      1:
        working_dir: "/tmp"
`
	_, err := Load(writeConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for unknown frontend type")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("got: %v", err)
	}
}
