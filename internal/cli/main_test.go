package cli

import (
	"os"
	"testing"

	"github.com/LinkwiseApp/linkwise-cli/internal/keyring"
)

// TestMain isolates the whole package from the machine it runs on.
//
// These tests drive real commands, and a command stores and deletes
// credentials. Pointed at the real config directory and the real keychain
// entry, `auth logout` in a test would sign the developer out for real, and a
// config file with output.format = "json" would fail the table tests. Both are
// redirected once here rather than in every test that happens to need it.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "linkwise-cli-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv("XDG_CONFIG_HOME", dir)
	keyring.Service = "app.linkwise.cli.test"

	// Inherited environment would otherwise reach into the precedence chain
	// and decide what these tests see.
	for _, k := range []string{"LINKWISE_TOKEN", "LINKWISE_API", "LINKWISE_PROFILE", "NO_COLOR"} {
		os.Unsetenv(k)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
