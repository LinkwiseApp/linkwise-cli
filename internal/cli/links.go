package cli

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
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

func (a *App) saveCmd() *cobra.Command {
	var title, description, collection string
	var tags []string

	cmd := &cobra.Command{
		Use:   "save <url>",
		Short: "Save a link",
		Long: "Saving a URL that is already in the library returns the existing link " +
			"rather than creating a duplicate.",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}
			ctx := cmd.Context()

			body := map[string]any{"url": args[0]}
			if title != "" {
				body["title"] = title
			}
			if description != "" {
				body["description"] = description
			}
			if collection != "" {
				id, err := resolveCollection(ctx, env, collection)
				if err != nil {
					return err
				}
				body["collection_id"] = id
			}

			var link api.Link
			if _, err := env.Client.Do(ctx, api.Request{Method: "POST", Path: "/links", Body: body}, &link); err != nil {
				return err
			}

			// Tags are a separate call: the create endpoint takes no tags, and
			// PUT replaces the set rather than adding to it.
			//
			// Names, not ids. The endpoint takes `tags` as an array of strings
			// and creates any that do not exist, so resolving them here would
			// be a lookup that buys nothing and a not_found for a tag the
			// server would happily have made.
			if len(tags) > 0 {
				if _, err := env.Client.Do(ctx, api.Request{
					Method: "PUT",
					Path:   "/links/" + link.ID + "/tags",
					Body:   map[string]any{"tags": tags},
				}, nil); err != nil {
					return err
				}
			}

			if env.Mode == render.JSON {
				return render.WriteJSON(env.Out, link)
			}
			fmt.Fprintf(env.Out, "%s  %s\n", api.ShortID(link.ID), api.Str(link.Title, link.URL))
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&title, "title", "", "Override the title instead of parsing one")
	f.StringVar(&description, "description", "", "A note to yourself about why you saved it")
	f.StringVar(&collection, "collection", "", "File it in a collection, by name or id")
	f.StringArrayVar(&tags, "tag", nil, "Tag it. Repeatable.")

	return cmd
}

func (a *App) openCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <id>",
		Short: "Open a saved link in the browser",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			var link api.Link
			if _, err := env.Client.Do(cmd.Context(),
				api.Request{Method: "GET", Path: "/links/" + args[0]}, &link); err != nil {
				return err
			}

			// Printed as well as opened, so this still does something useful
			// over ssh, where there is no browser to open.
			fmt.Fprintln(env.Err, link.URL)
			openBrowser(link.URL)
			return nil
		},
	}
}

func (a *App) readCmd() *cobra.Command {
	var raw bool

	cmd := &cobra.Command{
		Use:   "read <id>",
		Short: "Print the extracted reader content",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			var content api.ReaderContent
			if _, err := env.Client.Do(cmd.Context(),
				api.Request{Method: "GET", Path: "/links/" + args[0] + "/content"}, &content); err != nil {
				return err
			}

			if env.DocMode == render.JSON {
				return render.WriteJSON(env.Out, content)
			}

			html := api.Str(content.HTMLContent, "")
			if html == "" {
				// Extraction runs after a link is saved, so asking too soon is
				// a normal thing to do and deserves a real answer rather than
				// an empty page.
				return &api.Error{
					Code:    "not_found",
					Message: "No reader content yet. Extraction runs after a link is saved; try again shortly.",
				}
			}

			if raw {
				fmt.Fprintln(env.Out, html)
				return nil
			}

			markdown, err := htmltomarkdown.ConvertString(html)
			if err != nil {
				return err
			}
			fmt.Fprintln(env.Out, markdown)
			return nil
		},
	}

	cmd.Flags().BoolVar(&raw, "raw", false, "Print the extracted HTML instead of Markdown")
	return cmd
}

func (a *App) rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a link",
		Long:  "A soft delete. The link is recoverable in the app.",
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
				api.Request{Method: "DELETE", Path: "/links/" + args[0]}, nil); err != nil {
				return err
			}
			fmt.Fprintf(env.Err, "Deleted %s\n", args[0])
			return nil
		},
	}
}
