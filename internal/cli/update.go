package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/render"
	"github.com/LinkwiseApp/linkwise-cli/internal/selfupdate"
)

// The three seams this command is tested through. Each one reaches outside
// the process: the release page, the path the binary was installed to, and a
// shell. A test that used the real ones would upgrade the developer's own
// install, so they are variables rather than direct calls.
var (
	releasesURL  = ""
	detectMethod = selfupdate.Current
	runCommand   = shellOut
)

// updateReport is what `update --json` prints. The field names are a promise
// to whatever script reads them, so they live in one struct rather than being
// assembled inline.
type updateReport struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	InstalledVia    string `json:"installed_via"`
	Command         string `json:"command"`
}

// lines is the human rendering, kept apart from the command so it can be read
// without a terminal: a test writes to a buffer, the CLI treats a pipe as a
// request for JSON, and the prose would otherwise never be reachable.
func (r updateReport) lines(method selfupdate.Method, run bool) []string {
	if !r.UpdateAvailable {
		return []string{fmt.Sprintf("linkwise %s is the newest release.", r.Current)}
	}

	out := []string{
		fmt.Sprintf("A new release is out: %s -> %s", r.Current, r.Latest),
		fmt.Sprintf("Installed via %s.", method),
		"",
	}
	if run {
		return append(out, "Running: "+r.Command, "")
	}
	// Printed rather than run, because the three channels each own the file
	// they put on disk. Overwriting a Homebrew symlink from here would leave
	// brew certain it has installed something it no longer has.
	return append(out,
		"  "+r.Command,
		"",
		"Run it, or pass --run to have linkwise run it for you.",
	)
}

func (a *App) updateCmd() *cobra.Command {
	var run bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for a newer release, and say how to get it",
		Long: "Reports the newest published release and how to install it.\n\n" +
			"Which command that is depends on how this binary was installed, so " +
			"update works it out from where the binary actually lives rather " +
			"than guessing. Pass --run to execute it.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}

			// Deliberately no requireToken. Being unable to update because you
			// are signed out would be a bad way to find out your key expired.

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			latest, err := selfupdate.Latest(ctx, nil, releasesURL)
			if err != nil {
				return fmt.Errorf("could not reach the release page: %w", err)
			}

			method := detectMethod()
			report := updateReport{
				Current:         a.version,
				Latest:          latest,
				UpdateAvailable: selfupdate.Newer(a.version, latest),
				InstalledVia:    method.Slug(),
				Command:         method.Command(),
			}

			// Where the update command's own output goes. In JSON mode stdout
			// is a data channel, so brew's progress bars belong on stderr or
			// the document being parsed is ruined.
			child := io.Writer(os.Stdout)

			if env.Mode == render.JSON {
				child = os.Stderr
				if err := render.WriteJSON(env.Out, report); err != nil {
					return err
				}
			} else {
				for _, line := range report.lines(method, run && report.UpdateAvailable) {
					fmt.Fprintln(env.Out, line)
				}
			}

			// Outside the rendering branch on purpose: --run is an
			// instruction, not a preference about output, so a piped run has
			// to honour it too.
			if run && report.UpdateAvailable {
				return runCommand(report.Command, child)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&run, "run", false, "Run the update command instead of printing it")
	return cmd
}

// shellOut runs one command line with the user's streams attached, so that a
// password prompt or a progress bar behaves the way it would if they had
// typed it.
func shellOut(line string, stdout io.Writer) error {
	shell, flag := "/bin/sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/c"
	}

	cmd := exec.Command(shell, flag, line)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
