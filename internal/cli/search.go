package cli

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

func (a *App) searchCmd() *cobra.Command {
	var (
		mode       string
		highlights bool
		limit      int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the library",
		Long: "fast is keyword only and returns everything at once.\n" +
			"hybrid blends keyword with embeddings and is the default.",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Checked before anything is built, so a bad mode is a usage error
			// and nothing is sent. The API accepts only these two.
			if mode != "fast" && mode != "hybrid" {
				return &UsageError{Err: fmt.Errorf(
					"--mode must be fast or hybrid, not %q. hybrid blends keyword and embeddings", mode)}
			}

			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			path := "/search"
			query := url.Values{"q": []string{args[0]}}
			if highlights {
				// The highlights endpoint takes no mode: there is only one way
				// to search them.
				path = "/search/highlights"
			} else {
				query.Set("mode", mode)
			}
			if limit > 0 {
				query.Set("limit", strconv.Itoa(min(limit, 100)))
			}

			var hits []api.SearchHit
			if _, err := env.Client.Do(cmd.Context(),
				api.Request{Method: "GET", Path: path, Query: query}, &hits); err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteNDJSON(env.Out, hits)
			}

			rows := make([][]string, 0, len(hits))
			for _, h := range hits {
				rows = append(rows, []string{
					api.ShortID(h.ID),
					render.Truncate(api.Str(h.Title, api.Str(h.URL, "")), 56),
				})
			}
			return render.WriteTable(env.Out, []string{"ID", "TITLE"}, rows)
		},
	}

	f := cmd.Flags()
	f.StringVar(&mode, "mode", "hybrid", "fast or hybrid")
	f.BoolVar(&highlights, "highlights", false, "Search inside highlights instead of links")
	f.IntVar(&limit, "limit", 0, "Stop after this many results")

	return cmd
}
