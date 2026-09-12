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

	"github.com/LinkwiseApp/linkwise-cli/internal/cli"
)

// Set by goreleaser with -ldflags. "dev" when built by hand.
var version = "dev"

func main() {
	root := cli.NewRoot(version)
	// ExecuteContext rather than Execute so cmd.Context() is a real context
	// every command can hang a cancellation off, rather than nil.
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "linkwise:", err)
		os.Exit(cli.ExitCode(err))
	}
}
