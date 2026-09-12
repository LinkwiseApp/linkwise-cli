package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name         string
		isTTY        bool
		forceJSON    bool
		configFormat string
		want         Mode
	}{
		{"a terminal gets a table", true, false, "table", Table},
		{"a pipe gets json", false, false, "table", JSON},
		{"--json wins on a terminal", true, true, "table", JSON},
		{"config format json wins on a terminal", true, false, "json", JSON},
		// A config that says table must not turn a pipe back into a table:
		// piping is a stronger statement of intent than a file written once.
		{"config table does not override a pipe", false, false, "table", JSON},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Detect(c.isTTY, c.forceJSON, c.configFormat); got != c.want {
				t.Fatalf("Detect = %v, want %v", got, c.want)
			}
		})
	}
}

func TestUseColor(t *testing.T) {
	cases := []struct {
		setting    string
		noColorSet bool
		isTTY      bool
		want       bool
	}{
		{"auto", false, true, true},
		{"auto", false, false, false},
		// no-color.org: any value at all, including an empty one, turns it off.
		{"auto", true, true, false},
		{"always", true, false, true},
		{"never", false, true, false},
	}
	for _, c := range cases {
		if got := UseColor(c.setting, c.noColorSet, c.isTTY); got != c.want {
			t.Errorf("UseColor(%q, %v, %v) = %v", c.setting, c.noColorSet, c.isTTY, got)
		}
	}
}

func TestWriteNDJSONIsOneObjectPerLine(t *testing.T) {
	type row struct {
		ID string `json:"id"`
	}
	var buf bytes.Buffer
	if err := WriteNDJSON(&buf, []row{{"a"}, {"b"}}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 || lines[1] != `{"id":"b"}` {
		t.Fatalf("output = %q", buf.String())
	}
}

func TestWriteTableAlignsColumns(t *testing.T) {
	var buf bytes.Buffer
	err := WriteTable(&buf, []string{"ID", "TITLE"}, [][]string{
		{"a", "Short"},
		{"bbbbbbbb", "Longer title"},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want a header and two rows, got %d lines: %q", len(lines), buf.String())
	}
	// The title column must start at the same offset on every line.
	at := strings.Index(lines[0], "TITLE")
	if at != strings.Index(lines[1], "Short") || at != strings.Index(lines[2], "Longer") {
		t.Fatalf("columns are not aligned:\n%s", buf.String())
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("abcdefghij", 8); got != "abcde..." {
		t.Fatalf("Truncate = %q", got)
	}
	if got := Truncate("short", 8); got != "short" {
		t.Fatalf("Truncate = %q", got)
	}
	// Counted in runes, not bytes, or a title with an accent in it misaligns
	// the column it sits in.
	if got := Truncate("héllo wörld", 8); len([]rune(got)) != 8 {
		t.Fatalf("Truncate = %q, %d runes", got, len([]rune(got)))
	}
}
