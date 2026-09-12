package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsTheDocumentedShape(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := os.MkdirAll(filepath.Join(dir, "linkwise"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `default_profile = "default"

[profiles.default]
api = "https://example.test/v1"

[output]
format = "json"
color = "never"
`
	if err := os.WriteFile(filepath.Join(dir, "linkwise", "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Profiles["default"].API != "https://example.test/v1" {
		t.Fatalf("api = %q", cfg.Profiles["default"].API)
	}
	if cfg.Output.Format != "json" || cfg.Output.Color != "never" {
		t.Fatalf("output = %+v", cfg.Output)
	}
}

// A first run has no file. That is the normal case, not a failure.
func TestLoadWithNoFileReturnsDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output.Format != "table" || cfg.Output.Color != "auto" {
		t.Fatalf("defaults = %+v", cfg.Output)
	}
}
