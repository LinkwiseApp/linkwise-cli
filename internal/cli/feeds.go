package cli

import (
	"cmp"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

func (a *App) feedsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "feeds [ls|add|rm|export]", Short: "Manage RSS subscriptions"}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List subscriptions",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			var all []api.Feed
			rows := [][]string{}
			err = api.Each(cmd.Context(), env.Client, api.Request{Method: "GET", Path: "/feeds"},
				func(page []api.Feed) error {
					all = append(all, page...)
					for _, f := range page {
						rows = append(rows, []string{
							api.ShortID(f.ID),
							render.Truncate(cmp.Or(f.Title, f.FeedURL), 40),
							render.Truncate(f.FeedURL, 48),
						})
					}
					return nil
				})
			if err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteNDJSON(env.Out, all)
			}
			return render.WriteTable(env.Out, []string{"ID", "TITLE", "FEED"}, rows)
		},
	}

	add := &cobra.Command{
		Use:   "add <url>",
		Short: "Subscribe to a feed",
		// Discovery is the server's job: it takes a site address, a handle or
		// a feed URL and works out the rest, trying the platform-specific path
		// first and then the page's own link tags. So this passes through what
		// was typed rather than trying to find a feed itself.
		Long: "Takes a site address, a handle or a feed URL. Linkwise finds the feed.",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			var feed api.Feed
			if _, err := env.Client.Do(cmd.Context(), api.Request{
				Method: "POST", Path: "/feeds", Body: map[string]any{"url": args[0]},
			}, &feed); err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteJSON(env.Out, feed)
			}
			fmt.Fprintf(env.Out, "%s  %s\n", api.ShortID(feed.ID), cmp.Or(feed.Title, feed.FeedURL))
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <id>",
		Short: "Unsubscribe",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			if _, err := env.Client.Do(cmd.Context(),
				api.Request{Method: "DELETE", Path: "/feeds/" + args[0]}, nil); err != nil {
				return err
			}
			fmt.Fprintf(env.Err, "Unsubscribed from %s\n", args[0])
			return nil
		},
	}

	export := &cobra.Command{
		Use:   "export",
		Short: "Print every subscription as OPML",
		Long:  "Importable by any reader. Redirect it: linkwise feeds export > linkwise.opml",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			body, _, err := env.Client.DoRaw(cmd.Context(),
				api.Request{Method: "GET", Path: "/feeds/export"})
			if err != nil {
				return err
			}

			// The document and nothing else. Every progress line this command
			// might want to print would end up inside the reader's OPML file.
			_, err = env.Out.Write(body)
			return err
		},
	}

	cmd.AddCommand(ls, add, rm, export)
	return cmd
}
