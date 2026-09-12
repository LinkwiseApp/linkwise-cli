package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

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

			query := url.Values{"q": []string{args[0]}}
			if limit > 0 {
				query.Set("limit", strconv.Itoa(min(limit, 100)))
			}

			// Two routes with two different response shapes, so they are
			// decoded separately rather than through one struct that fits
			// neither.
			if highlights {
				// This one takes no mode: there is only one way to search
				// highlights. It is also the one that returns a flat array.
				var found []api.Highlight
				if _, err := env.Client.Do(cmd.Context(),
					api.Request{Method: "GET", Path: "/search/highlights", Query: query}, &found); err != nil {
					return err
				}
				found = capped(found, limit)

				if env.Mode == render.JSON {
					return render.WriteNDJSON(env.Out, found)
				}
				rows := make([][]string, 0, len(found))
				for _, h := range found {
					rows = append(rows, []string{
						api.ShortID(h.ID),
						render.Truncate(h.SelectedText, 64),
					})
				}
				return render.WriteTable(env.Out, []string{"ID", "TEXT"}, rows)
			}

			query.Set("mode", mode)

			var results api.SearchResults
			if _, err := env.Client.Do(cmd.Context(),
				api.Request{Method: "GET", Path: "/search", Query: query}, &results); err != nil {
				return err
			}

			// mode=fast returns everything it found in one response and does
			// not apply limit, so the cap is applied here for both modes.
			hits := capped(results.Links, limit)

			// What else matched goes to stderr. It is worth knowing that the
			// word also named a collection, and stdout has to stay pipeable.
			if note := otherMatches(results); note != "" {
				fmt.Fprintln(env.Err, note)
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

func capped[T any](items []T, limit int) []T {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

// otherMatches describes the buckets the table does not show. Empty when the
// query only matched links, so the common case prints nothing.
func otherMatches(r api.SearchResults) string {
	var parts []string
	for _, b := range []struct {
		n    int
		noun string
	}{
		{len(r.Collections), "collection"},
		{len(r.Highlights), "highlight"},
		{len(r.Tags), "tag"},
		{len(r.Feeds), "feed"},
	} {
		if b.n == 0 {
			continue
		}
		noun := b.noun
		if b.n != 1 {
			noun += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", b.n, noun))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Also matched " + strings.Join(parts, ", ") + "."
}
