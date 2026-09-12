// Package cli holds every command, and the wiring that gives one a
// configured API client.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/config"
	"github.com/LinkwiseApp/linkwise-cli/internal/keyring"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

// UsageError marks a failure that happened before anything was sent: a bad
// flag, a missing argument, an unknown command. It exists so that main can
// tell "you typed it wrong" from "the server said no", which the published
// exit-code table treats as different things.
type UsageError struct{ Err error }

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

// App holds the persistent flags and builds what a command needs from them.
// One place resolves configuration, so no command reimplements precedence.
type App struct {
	version string

	flagToken   string
	flagProfile string
	flagAPI     string
	flagJSON    bool

	cmd *cobra.Command
}

// Env is everything a command needs that is not its own arguments.
type Env struct {
	Client   *api.Client
	Config   *config.Config
	Store    *keyring.Store
	Resolved config.Resolved
	Mode     render.Mode
	Out      io.Writer
	Err      io.Writer
	In       io.Reader
}

func NewRoot(version string) *cobra.Command {
	app := &App{version: version}

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
	// command test saw err == nil with no RunE set.
	//
	// With a RunE present, the command becomes runnable, and cobra.NoArgs
	// does reach the args check, but the error it returns comes straight
	// back from execute() unwrapped, ahead of RunE, so ExitCode saw a plain
	// error and returned 1.
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

	f := root.PersistentFlags()
	f.StringVar(&app.flagToken, "token", "", "Personal access token. `-` reads it from stdin.")
	f.StringVar(&app.flagProfile, "profile", "", "Which configured profile to use")
	f.StringVar(&app.flagAPI, "api", "", "Base URL, for pointing at another deploy")
	f.BoolVar(&app.flagJSON, "json", false, "Force JSON output")

	app.cmd = root
	root.AddCommand(app.authCmd(), app.lsCmd())

	return root
}

// Env resolves configuration and builds a client. Called by each command's
// RunE rather than in a persistent hook, so that a command which needs no
// credential is not blocked by the absence of one.
func (a *App) Env() (*Env, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	store := keyring.New(dir)

	resolved, err := config.Resolve(config.Sources{
		FlagToken:   a.flagToken,
		FlagProfile: a.flagProfile,
		FlagAPI:     a.flagAPI,
		Getenv:      os.Getenv,
		Keychain:    store.Get,
		File:        cfg,
	})
	if err != nil {
		return nil, &UsageError{Err: err}
	}

	// The command's own streams, not os.Stdout, so tests can capture them.
	out := a.cmd.OutOrStdout()
	errOut := a.cmd.ErrOrStderr()

	isTTY := false
	if f, ok := out.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	return &Env{
		Client:   api.New(resolved.BaseURL, resolved.Token, a.version),
		Config:   cfg,
		Store:    store,
		Resolved: resolved,
		Mode:     render.Detect(isTTY, a.flagJSON, cfg.Output.Format),
		Out:      out,
		Err:      errOut,
		In:       a.cmd.InOrStdin(),
	}, nil
}

// requireToken is the check every command that calls the API makes first.
// Reporting "not signed in" locally beats sending a request with an empty
// bearer and reporting whatever the server says about it.
func (e *Env) requireToken() error {
	if e.Resolved.Token == "" {
		return &api.Error{
			Code: "unauthorized",
			Message: "Not signed in. Run `linkwise auth login`, " +
				"or set LINKWISE_TOKEN.",
		}
	}
	return nil
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
