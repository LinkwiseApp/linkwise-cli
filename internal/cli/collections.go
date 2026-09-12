package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

// namedThing is the shape collections and tags share: an id and a name, listed,
// created and deleted the same way. One implementation, two commands, because
// two copies of this would drift the moment either endpoint changed.
type namedThing struct {
	noun   string // "collection"
	plural string // "collections"
	path   string // "/collections"
}

// A function rather than a method on App, because a method cannot take a type
// parameter and the two resources decode into different structs.
func namedCmd[T api.Named](a *App, n namedThing) *cobra.Command {
	cmd := &cobra.Command{
		Use:   n.plural + " [ls|new|rm]",
		Short: "Manage " + n.plural,
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List " + n.plural,
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			var all []T
			rows := [][]string{}
			err = api.Each(cmd.Context(), env.Client, api.Request{Method: "GET", Path: n.path},
				func(page []T) error {
					all = append(all, page...)
					for _, item := range page {
						rows = append(rows, []string{api.ShortID(item.ID()), item.Name()})
					}
					return nil
				})
			if err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteNDJSON(env.Out, all)
			}
			return render.WriteTable(env.Out, []string{"ID", "NAME"}, rows)
		},
	}

	newCmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a " + n.noun,
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			// The request body says `name` even though the response says
			// `collection_name`. That asymmetry is the API's, not ours.
			var created T
			if _, err := env.Client.Do(cmd.Context(), api.Request{
				Method: "POST", Path: n.path, Body: map[string]any{"name": args[0]},
			}, &created); err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteJSON(env.Out, created)
			}
			fmt.Fprintf(env.Out, "%s  %s\n", api.ShortID(created.ID()), created.Name())
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <name-or-id>",
		Short: "Delete a " + n.noun,
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			var id string
			if n.noun == "collection" {
				id, err = resolveCollection(cmd.Context(), env, args[0])
			} else {
				id, err = resolveTag(cmd.Context(), env, args[0])
			}
			if err != nil {
				return err
			}

			if _, err := env.Client.Do(cmd.Context(),
				api.Request{Method: "DELETE", Path: n.path + "/" + id}, nil); err != nil {
				return err
			}
			fmt.Fprintf(env.Err, "Deleted %s %s\n", n.noun, args[0])
			return nil
		},
	}

	cmd.AddCommand(ls, newCmd, rm)
	return cmd
}
