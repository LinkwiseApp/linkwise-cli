package config

import (
	"os"
	"path/filepath"
	"strings"
)

// TUI is the `[tui]` table in config.toml.
type TUI struct {
	Theme string `toml:"theme"` // dark | light
}

// Dark reports which palette to start in, defaulting to dark because a
// terminal that has never been configured is far more often a dark one.
func (t TUI) Dark() bool { return !strings.EqualFold(t.Theme, "light") }

// SaveTheme rewrites only the `[tui]` table, leaving the rest of the file
// byte for byte as it was.
//
// Marshalling the whole Config back out would be shorter and would silently
// delete every comment and every key this version does not know about. A
// config file is the user's, not ours, so the edit is surgical.
func SaveTheme(dark bool) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	path := filepath.Join(dir, "config.toml")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	theme := "dark"
	if !dark {
		theme = "light"
	}

	out := replaceTable(string(raw), "tui", "[tui]\ntheme = \""+theme+"\"\n")
	return os.WriteFile(path, []byte(out), 0o600)
}

// replaceTable swaps one top-level TOML table for new text, appending it if
// the table is not there yet.
//
// A table runs from its own header to the next line that starts one, which
// is enough structure to edit a section without parsing the language. It is
// not enough for a table nested under another, so this is deliberately only
// used for a top-level one.
func replaceTable(doc, name, replacement string) string {
	header := "[" + name + "]"
	lines := strings.Split(doc, "\n")

	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			start = i
			break
		}
	}
	if start == -1 {
		trimmed := strings.TrimRight(doc, "\n")
		if trimmed == "" {
			return replacement
		}
		return trimmed + "\n\n" + replacement
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			end = i
			break
		}
	}

	rest := append([]string{}, lines[:start]...)
	rest = append(rest, strings.Split(strings.TrimRight(replacement, "\n"), "\n")...)
	if end < len(lines) {
		rest = append(rest, "")
		rest = append(rest, lines[end:]...)
	}
	return strings.TrimRight(strings.Join(rest, "\n"), "\n") + "\n"
}
