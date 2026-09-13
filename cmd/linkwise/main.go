// Command linkwise is the Linkwise command line interface.
//
// It does one thing beyond calling into internal/cli: it turns an error into
// an exit code. Scripts distinguish an expired key from a missing link by that
// number alone, so it is the one piece of behaviour that cannot live anywhere
// a library might swallow it.
package main

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/LinkwiseApp/linkwise-cli/internal/cli"
)

// Set by goreleaser with -ldflags. "dev" when built by hand.
var version = "dev"

func main() {
	// Started before the command rather than after it, so the lookup overlaps
	// the work the user actually asked for instead of being added to it.
	check := cli.BeginVersionCheck(version, os.Args[1:],
		term.IsTerminal(int(os.Stderr.Fd())), os.LookupEnv)

	root := cli.NewRoot(version)
	// ExecuteContext rather than Execute so cmd.Context() is a real context
	// every command can hang a cancellation off, rather than nil.
	err := root.ExecuteContext(context.Background())

	// Ahead of the error, so that the last line on the screen is the thing
	// that went wrong rather than an advertisement.
	if notice := check.Notice(cli.NoticeGrace); notice != "" {
		fmt.Fprintln(os.Stderr, notice)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "linkwise:", err)
		os.Exit(cli.ExitCode(err))
	}
}
