// Package config reads the documented config.toml and works out which
// profile, base URL and token a command should use.
//
// It never touches the keychain itself. The lookup is passed in, so the
// precedence rules can be tested exhaustively without one, and so a CI machine
// with no keychain runs the same code a laptop does.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const DefaultAPI = "https://jcbgrqawrvztwsvxawda.supabase.co/functions/v1/api/v1"

type Config struct {
	DefaultProfile string             `toml:"default_profile"`
	Profiles       map[string]Profile `toml:"profiles"`
	Output         Output             `toml:"output"`
	TUI            TUI                `toml:"tui"`
}

type Profile struct {
	API string `toml:"api"`
}

type Output struct {
	Format string `toml:"format"` // table | json
	Color  string `toml:"color"`  // auto | always | never
}

// Dir is ~/.config/linkwise, honouring XDG_CONFIG_HOME.
//
// Deliberately not os.UserConfigDir: on macOS that returns Library/Application
// Support, and the published docs promise ~/.config/linkwise on every
// platform. A path in documentation is a contract.
func Dir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "linkwise"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "linkwise"), nil
}

func defaults() *Config {
	return &Config{
		Profiles: map[string]Profile{},
		Output:   Output{Format: "table", Color: "auto"},
	}
}

// Load reads the config file. A missing file is the normal first run, not a
// failure, and yields the defaults.
func Load() (*Config, error) {
	cfg := defaults()

	dir, err := Dir()
	if err != nil {
		return cfg, err
	}

	raw, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	if err := toml.Unmarshal(raw, cfg); err != nil {
		return defaults(), err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if cfg.Output.Format == "" {
		cfg.Output.Format = "table"
	}
	if cfg.Output.Color == "" {
		cfg.Output.Color = "auto"
	}
	return cfg, nil
}
