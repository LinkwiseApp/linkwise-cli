package cli

import (
	"strings"
	"testing"
)

func noEnv(string) (string, bool) { return "", false }

// envWith reports the named variables as present, mirroring os.LookupEnv so
// that "set but empty" is distinguishable from unset.
func envWith(set map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := set[k]
		return v, ok
	}
}

func TestShouldCheckOnAnOrdinaryInteractiveCommand(t *testing.T) {
	if !shouldCheck([]string{"ls"}, true, noEnv) {
		t.Fatal("want a check on a plain interactive command")
	}
}

// The notice goes to stderr, and a stderr that is not a terminal is a log
// file or a capture. Neither wants advertising in it.
func TestShouldNotCheckWhenStderrIsNotATerminal(t *testing.T) {
	if shouldCheck([]string{"ls"}, false, noEnv) {
		t.Fatal("want no check when stderr is redirected")
	}
}

func TestShouldNotCheckWhenTheUserOptedOut(t *testing.T) {
	env := envWith(map[string]string{"LINKWISE_NO_UPDATE_CHECK": "1"})
	if shouldCheck([]string{"ls"}, true, env) {
		t.Fatal("want no check when LINKWISE_NO_UPDATE_CHECK is set")
	}
}

// Following no-color.org's convention: the variable being present is the
// signal, whatever it is set to. Requiring a truthy value would mean
// LINKWISE_NO_UPDATE_CHECK= quietly did nothing.
func TestOptingOutWorksWithAnEmptyValue(t *testing.T) {
	env := envWith(map[string]string{"LINKWISE_NO_UPDATE_CHECK": ""})
	if shouldCheck([]string{"ls"}, true, env) {
		t.Fatal("want no check when the opt-out is set to an empty value")
	}
}

func TestShouldNotCheckInCI(t *testing.T) {
	env := envWith(map[string]string{"CI": "true"})
	if shouldCheck([]string{"ls"}, true, env) {
		t.Fatal("want no check in CI")
	}
}

// These either do the check themselves, print something a shell evaluates,
// or answer instantly and would be dominated by the wait.
func TestShouldNotCheckOnCommandsThatShouldNotBeDelayed(t *testing.T) {
	for _, args := range [][]string{
		{"update"},
		{"update", "--run"},
		{"completion", "zsh"},
		{"help"},
		{"--version"},
		{"-v"},
		{"--help"},
		{"-h"},
		{"ls", "--help"},
	} {
		if shouldCheck(args, true, noEnv) {
			t.Errorf("want no check for %v", args)
		}
	}
}

// A JSON run is a script's run, even from a terminal.
func TestShouldNotCheckOnAJSONRun(t *testing.T) {
	if shouldCheck([]string{"ls", "--json"}, true, noEnv) {
		t.Fatal("want no check on a --json run")
	}
}

func TestShouldCheckWithNoArgumentsAtAll(t *testing.T) {
	if !shouldCheck(nil, true, noEnv) {
		t.Fatal("want a check for bare linkwise")
	}
}

// End to end through the exported entry point, so the wiring main uses is
// covered rather than only the predicate under it.
func TestBeginVersionCheckIsSilentWhenSuppressed(t *testing.T) {
	check := BeginVersionCheck("0.1.0", []string{"update"}, true, noEnv)
	if notice := check.Notice(0); notice != "" {
		t.Fatalf("want silence, got %q", notice)
	}
}

// Whatever happens looking up a cache path or reaching the network, a version
// check must never be the thing that stops a command from running.
func TestBeginVersionCheckNeverReportsItsOwnFailures(t *testing.T) {
	check := BeginVersionCheck("0.1.0", []string{"ls"}, true, noEnv)
	if notice := check.Notice(0); strings.Contains(strings.ToLower(notice), "error") {
		t.Fatalf("reported an error as a notice: %q", notice)
	}
}
