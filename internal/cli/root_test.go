package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	root := NewRoot("1.2.3")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "1.2.3") {
		t.Fatalf("want version in output, got %q", out.String())
	}
}

// An unknown command is a usage error, which the published exit-code table
// says is 2. Getting this wrong is invisible until a script depends on it.
func TestUnknownCommandIsExitCodeTwo(t *testing.T) {
	root := NewRoot("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"nosuchcommand"})

	err := root.Execute()
	if err == nil {
		t.Fatal("want an error for an unknown command")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("want exit code 2, got %d", got)
	}
}
