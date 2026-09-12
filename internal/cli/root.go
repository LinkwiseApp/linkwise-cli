// Package cli holds every command, and the wiring that gives one a
// configured API client.
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

// UsageError marks a failure that happened before anything was sent: a bad
// flag, a missing argument, an unknown command. It exists so that main can
// tell "you typed it wrong" from "the server said no", which the published
// exit-code table treats as different things.
type UsageError struct{ Err error }

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

func NewRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "linkwise",
		Short:   "Your Linkwise library, from the terminal",
		Version: version,
		// Cobra prints usage on any returned error by default, which buries a
		// one line server message under sixty lines of help. Errors are
		// printed by main, in one place, on stderr.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetVersionTemplate("{{.Version}}\n")

	// Every error cobra itself produces is a usage error: it only fails before
	// a RunE is reached. Wrapping them here means no command has to remember.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &UsageError{Err: err}
	})

	// A command with neither Run nor RunE is not "Runnable" by cobra's own
	// definition, and an unrunnable command never reaches its Args
	// validator at all: cobra's execute() returns flag.ErrHelp first, and
	// ExecuteC treats that as "print help, return nil", so the unknown
	// command test saw err == nil with no RunE set. Confirmed by running
	// exactly that (Args set, no RunE): the test failed on the "want an
	// error" line, before ExitCode was even reached.
	//
	// With a RunE present, the command becomes runnable, and cobra.NoArgs
	// (the brief's first sketch) does reach the args check, but the error
	// it returns comes straight back from execute() unwrapped, ahead of
	// RunE, so ExitCode saw a plain error and returned 1. Confirmed by
	// running root.Args = cobra.NoArgs with the RunE below: the unknown
	// command test failed with ExitCode returning 1, not 2.
	//
	// So RunE has to stay, to keep the command runnable, but the Args
	// validator has to be the one that produces the UsageError, since
	// nothing downstream of it ever runs for a rejected arg list.
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}
	root.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return &UsageError{Err: fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())}
		}
		return nil
	}

	return root
}

// ExitCode maps an error onto the codes published at
// linkwise.app/developers/cli#exit-codes. A usage error is caught here because
// nothing was sent; everything else is the API's to classify.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		return 2
	}
	return api.ExitCode(err)
}
