package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

const dashboardURL = "https://linkwise.app/developers/dashboard"

func (a *App) authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Sign in, check who you are, sign out",
	}
	cmd.AddCommand(a.authLoginCmd(), a.authStatusCmd(), a.authLogoutCmd())
	return cmd
}

func (a *App) authLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Store a personal access token",
		Long: "Create a key at " + dashboardURL + " and paste it here.\n" +
			"The key is validated before it is stored, and kept in the system keychain.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}

			token := a.flagToken
			switch token {
			case "-":
				// Piped, for scripts. Not a prompt, so no browser and no echo
				// handling.
				line, err := bufio.NewReader(env.In).ReadString('\n')
				if err != nil && line == "" {
					return err
				}
				token = strings.TrimSpace(line)
			case "":
				fmt.Fprintf(env.Err, "Create a key at %s\n", dashboardURL)
				openBrowser(dashboardURL)
				token, err = promptForToken(env)
				if err != nil {
					return err
				}
			}

			if token == "" {
				return &UsageError{Err: fmt.Errorf("no token given")}
			}

			// Validated before it is stored, so a mistyped key fails here
			// rather than on whatever command the user runs next.
			client := api.New(env.Resolved.BaseURL, token, a.version)
			var me api.Me
			if _, err := client.Do(context.Background(), api.Request{Method: "GET", Path: "/me"}, &me); err != nil {
				return err
			}

			from, err := env.Store.Set(env.Resolved.Profile, token)
			if err != nil {
				return err
			}

			fmt.Fprintf(env.Err, "Signed in as %s\n", me.Email)
			if from == "file" {
				// Said out loud. A tool that quietly writes a credential to
				// disk is a tool that surprises someone later.
				fmt.Fprintln(env.Err,
					"No system keychain here, so the key was written to credentials.toml with 0600 permissions.")
			} else {
				fmt.Fprintln(env.Err, "Key stored in the system keychain.")
			}
			return nil
		},
	}
}

func (a *App) authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the active profile, its scopes and where its key came from",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			var me api.Me
			if _, err := env.Client.Do(context.Background(), api.Request{Method: "GET", Path: "/me"}, &me); err != nil {
				return err
			}

			plan := "free"
			if me.Subscription != nil && me.Subscription.Plan != "" {
				plan = me.Subscription.Plan
			}

			status := map[string]any{
				"profile":    env.Resolved.Profile,
				"account":    me.Email,
				"plan":       plan,
				"api":        env.Resolved.BaseURL,
				"token":      maskToken(env.Resolved.Token),
				"token_from": env.Resolved.TokenFrom,
				"scopes":     me.Auth.ScopeList(),
			}

			if env.Mode == render.JSON {
				return render.WriteJSON(env.Out, status)
			}

			// Expiry is missing on purpose: GET /v1/me does not return it, and
			// inventing a value would be worse than leaving the row out.
			return render.WriteTable(env.Out, nil, [][]string{
				{"Profile", env.Resolved.Profile},
				{"Account", fmt.Sprintf("%s  (%s)", me.Email, plan)},
				{"Key", fmt.Sprintf("%s  (from the %s)", maskToken(env.Resolved.Token), env.Resolved.TokenFrom)},
				{"Scopes", strings.Join(me.Auth.ScopeList(), " ")},
			})
		},
	}
}

func (a *App) authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored key from this machine",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}

			from, err := env.Store.Delete(env.Resolved.Profile)
			if err != nil {
				return err
			}

			if from == "" {
				fmt.Fprintln(env.Err, "Nothing stored for this profile.")
				// Still worth pointing at, because a key that was never on
				// this machine, or was removed from it earlier, is very much
				// still live on the account.
				fmt.Fprintf(env.Err, "Keys are listed and revoked at %s.\n", dashboardURL)
				return nil
			}

			fmt.Fprintf(env.Err, "Removed the key from the %s.\n", from)
			// Deliberately not a revocation. DELETE /v1/tokens/{id} requires a
			// session, and requireJwt is there so that a leaked token cannot
			// manage tokens. Saying the key still works is more useful than
			// implying it does not.
			fmt.Fprintf(env.Err,
				"The key itself still works. Revoke it at %s.\n", dashboardURL)
			return nil
		},
	}
}

func promptForToken(env *Env) (string, error) {
	fmt.Fprint(env.Err, "Paste key: ")

	if f, ok := env.In.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		raw, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(env.Err)
		return strings.TrimSpace(string(raw)), err
	}

	line, err := bufio.NewReader(env.In).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// maskToken shows enough to tell two keys apart and not enough to use one.
// The prefix is already public: it is what the dashboard lists keys by.
func maskToken(token string) string {
	if len(token) <= 11 {
		return token
	}
	return token[:11] + "..."
}

// openBrowser is best effort. The URL is printed first, so a machine with no
// browser loses nothing by this failing.
func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "explorer"
	default:
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, url).Start()
}
