package cli

import (
	"time"

	"github.com/LinkwiseApp/linkwise-cli/internal/config"
	"github.com/LinkwiseApp/linkwise-cli/internal/selfupdate"
)

// NoticeGrace is how long the run is willing to wait at exit for a version
// check that has not finished.
//
// Short on purpose. The lookup was started before the command ran, so a
// command that talked to the API has usually already paid for it; this only
// covers the case of something that finished faster than one round trip, and
// in that case the answer waits for the next run, which reads it from the
// cache for free.
const NoticeGrace = 400 * time.Millisecond

// quietCommands neither want an update notice nor should be made to wait for
// one: update does the lookup itself, completion prints a script a shell
// evaluates, and the rest answer instantly.
var quietCommands = map[string]bool{
	"update":     true,
	"completion": true,
	"help":       true,
}

// BeginVersionCheck starts a lookup alongside the command, when one is wanted.
// It never blocks and never fails.
func BeginVersionCheck(version string, args []string, stderrIsTTY bool, lookupEnv func(string) (string, bool)) *selfupdate.Check {
	if !shouldCheck(args, stderrIsTTY, lookupEnv) {
		return nil
	}

	dir, err := config.Dir()
	if err != nil {
		// No config directory is no cache, and a check with nowhere to
		// remember its answer would ask on every single command.
		return nil
	}

	return selfupdate.Begin(selfupdate.Options{
		Current:   version,
		CachePath: selfupdate.CachePath(dir),
		Method:    selfupdate.Current(),
		Now:       time.Now(),
	})
}

// shouldCheck decides whether this invocation is one a notice belongs in.
//
// Separate from BeginVersionCheck, and given its environment rather than
// reading it, because every one of these rules is a decision someone will
// want to change and each is a line of a test.
func shouldCheck(args []string, stderrIsTTY bool, lookupEnv func(string) (string, bool)) bool {
	// The notice is written to stderr, so a stderr that is not a terminal is
	// a log file or a capture, and neither wants advertising in it.
	if !stderrIsTTY {
		return false
	}

	// Presence is the signal, following no-color.org, so that setting it to
	// an empty string is not a confusing no-op.
	if _, ok := lookupEnv("LINKWISE_NO_UPDATE_CHECK"); ok {
		return false
	}
	// A pipeline did not choose its version and cannot act on the news.
	if _, ok := lookupEnv("CI"); ok {
		return false
	}

	for i, arg := range args {
		switch arg {
		case "--version", "-v", "--help", "-h", "--json":
			return false
		}
		// Only the first bare word is the command; a later one is an
		// argument, and a link titled "help" is not a reason to go quiet.
		if i == 0 && quietCommands[arg] {
			return false
		}
	}
	return true
}
