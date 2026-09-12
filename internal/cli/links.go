package cli

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (a *App) lsCmd() *cobra.Command {
	var (
		collection string
		tag        string
		archived   bool
		limit      int
	)

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List links, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}
			ctx := cmd.Context()

			query := url.Values{}
			if collection != "" {
				id, err := resolveCollection(ctx, env, collection)
				if err != nil {
					return err
				}
				query.Set("collection_id", id)
			}
			if tag != "" {
				id, err := resolveTag(ctx, env, tag)
				if err != nil {
					return err
				}
				query.Set("tag_id", id)
			}
			// Tri-state on the server: omitting it is not the same as false.
			// Only send it when asked for, so the default stays the server's.
			if archived {
				query.Set("archived", "true")
			}
			if limit > 0 {
				query.Set("limit", strconv.Itoa(min(limit, 100)))
			}

			rows := [][]string{}
			var collected []api.Link

			err = api.Each(ctx, env.Client, api.Request{Method: "GET", Path: "/links", Query: query},
				func(page []api.Link) error {
					for _, l := range page {
						if limit > 0 && len(collected) >= limit {
							return api.ErrStop
						}
						collected = append(collected, l)
						rows = append(rows, []string{
							api.ShortID(l.ID),
							render.Truncate(api.Str(l.Title, l.URL), 48),
							render.Truncate(l.URL, 40),
							api.Day(l.CreatedAt),
						})
					}
					if limit > 0 && len(collected) >= limit {
						return api.ErrStop
					}
					return nil
				})
			if err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteNDJSON(env.Out, collected)
			}
			return render.WriteTable(env.Out, []string{"ID", "TITLE", "URL", "SAVED"}, rows)
		},
	}

	f := cmd.Flags()
	f.StringVar(&collection, "collection", "", "Filter by collection name or id")
	f.StringVar(&tag, "tag", "", "Filter by tag name or id")
	f.BoolVar(&archived, "archived", false, "Show archived links instead of hiding them")
	f.IntVar(&limit, "limit", 0, "Stop after this many links")

	return cmd
}

// resolveCollection accepts a name or an id, because the published examples
// use names and nobody types a UUID at a prompt. An id is passed through
// untouched, so a script that already has one pays nothing for this.
func resolveCollection(ctx context.Context, env *Env, nameOrID string) (string, error) {
	if uuidPattern.MatchString(nameOrID) {
		return nameOrID, nil
	}

	var found string
	err := api.Each(ctx, env.Client, api.Request{Method: "GET", Path: "/collections"},
		func(page []api.Collection) error {
			for _, c := range page {
				if c.Name() == nameOrID {
					found = c.ID()
					return api.ErrStop
				}
			}
			return nil
		})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", &api.Error{
			Code:    "not_found",
			Message: fmt.Sprintf("No collection named %q. Run `linkwise collections ls` to see them.", nameOrID),
		}
	}
	return found, nil
}

func resolveTag(ctx context.Context, env *Env, nameOrID string) (string, error) {
	if uuidPattern.MatchString(nameOrID) {
		return nameOrID, nil
	}

	var found string
	err := api.Each(ctx, env.Client, api.Request{Method: "GET", Path: "/tags"},
		func(page []api.Tag) error {
			for _, t := range page {
				if t.Name() == nameOrID {
					found = t.ID()
					return api.ErrStop
				}
			}
			return nil
		})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", &api.Error{
			Code:    "not_found",
			Message: fmt.Sprintf("No tag named %q. Run `linkwise tags ls` to see them.", nameOrID),
		}
	}
	return found, nil
}
