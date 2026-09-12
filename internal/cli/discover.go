package cli

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

func (a *App) discoverCmd() *cobra.Command {
	var save string
	var limit int

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Print the recommendation feed",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}
			ctx := cmd.Context()

			// --save takes an item id from a previous run, so it does not need
			// the feed and should not spend a request fetching it.
			if save != "" {
				var link api.Link
				if _, err := env.Client.Do(ctx, api.Request{
					Method: "POST", Path: "/discover/" + save + "/save",
				}, &link); err != nil {
					return err
				}
				if env.Mode == render.JSON {
					return render.WriteJSON(env.Out, link)
				}
				fmt.Fprintf(env.Out, "%s  %s\n", api.ShortID(link.ID), api.Str(link.Title, link.URL))
				return nil
			}

			query := url.Values{}
			if limit > 0 {
				query.Set("limit", strconv.Itoa(min(limit, 100)))
			}

			var items []api.DiscoverItem
			if _, err := env.Client.Do(ctx,
				api.Request{Method: "GET", Path: "/discover", Query: query}, &items); err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteNDJSON(env.Out, items)
			}

			rows := make([][]string, 0, len(items))
			for _, it := range items {
				rows = append(rows, []string{
					api.ShortID(it.ID),
					render.Truncate(api.Str(it.Title, api.Str(it.SourceURL, "")), 56),
					api.Str(it.Category, ""),
				})
			}
			return render.WriteTable(env.Out, []string{"ID", "TITLE", "CATEGORY"}, rows)
		},
	}

	f := cmd.Flags()
	f.StringVar(&save, "save", "", "Save one item into the library, by id")
	f.IntVar(&limit, "limit", 0, "How many items to print")

	return cmd
}
