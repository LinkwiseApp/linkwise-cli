package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarkIsTheDefault(t *testing.T) {
	cases := map[string]bool{"": true, "dark": true, "light": false, "LIGHT": false, "nonsense": true}
	for in, want := range cases {
		if got := (TUI{Theme: in}).Dark(); got != want {
			t.Errorf("theme %q: Dark() = %v, want %v", in, got, want)
		}
	}
}

func TestSaveThemeCreatesTheFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := SaveTheme(false); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "linkwise", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `theme = "light"`) {
		t.Errorf("file is %q", raw)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TUI.Dark() {
		t.Error("the saved theme did not load back as light")
	}
}

// The config file is the user's. Rewriting it from the parsed struct would be
// shorter and would silently drop every comment and every key this version
// does not know about, so this is the test that keeps the edit surgical.
func TestSaveThemeKeepsTheRestOfTheFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "linkwise")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	original := `# my own notes, please keep them
default_profile = "work"

[output]
format = "table"
color = "never"

[profiles.work]
api = "https://example.test/v1"

[something_a_later_version_added]
key = "value"
`
	if err := os.WriteFile(filepath.Join(path, "config.toml"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SaveTheme(true); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(path, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)

	for _, keep := range []string{
		"# my own notes, please keep them",
		`default_profile = "work"`,
		`color = "never"`,
		`api = "https://example.test/v1"`,
		"[something_a_later_version_added]",
		`key = "value"`,
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("the edit dropped %q\n--- file ---\n%s", keep, got)
		}
	}
	if !strings.Contains(got, `theme = "dark"`) {
		t.Errorf("the theme was not written\n--- file ---\n%s", got)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("the file no longer parses: %v\n--- file ---\n%s", err, got)
	}
	if cfg.DefaultProfile != "work" || cfg.Output.Color != "never" {
		t.Errorf("the reloaded config lost settings: %+v", cfg)
	}
}

func TestSaveThemeReplacesRatherThanRepeats(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	for _, dark := range []bool{true, false, true} {
		if err := SaveTheme(dark); err != nil {
			t.Fatal(err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, "linkwise", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)

	if n := strings.Count(got, "[tui]"); n != 1 {
		t.Errorf("the table appears %d times, want once\n--- file ---\n%s", n, got)
	}
	if !strings.Contains(got, `theme = "dark"`) || strings.Contains(got, `theme = "light"`) {
		t.Errorf("the last write did not win\n--- file ---\n%s", got)
	}
}

// A table in the middle of the file has a table after it, which is the case
// where an edit that ran to the end of the file would eat the rest.
func TestReplaceTableInTheMiddle(t *testing.T) {
	doc := "a = 1\n\n[tui]\ntheme = \"light\"\n\n[output]\nformat = \"json\"\n"
	got := replaceTable(doc, "tui", "[tui]\ntheme = \"dark\"\n")

	for _, keep := range []string{"a = 1", "[output]", `format = "json"`, `theme = "dark"`} {
		if !strings.Contains(got, keep) {
			t.Errorf("missing %q in:\n%s", keep, got)
		}
	}
	if strings.Contains(got, `theme = "light"`) {
		t.Errorf("the old value survived:\n%s", got)
	}
}
