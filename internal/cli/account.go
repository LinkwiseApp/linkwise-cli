package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

func (a *App) usageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "usage",
		Short: "Plan, credits and what is left",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			var usage api.Usage
			if _, err := env.Client.Do(cmd.Context(),
				api.Request{Method: "GET", Path: "/me/usage"}, &usage); err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteJSON(env.Out, usage)
			}

			num := func(p *int) string {
				if p == nil {
					return "-"
				}
				return fmt.Sprintf("%d", *p)
			}

			return render.WriteTable(env.Out, []string{"", "ASSIGNED", "USED", "LEFT"}, [][]string{
				{"Chat", num(usage.Chat.Assigned), num(usage.Chat.Used), num(usage.Chat.Remaining)},
				{"Embedding", num(usage.Embedding.Assigned), num(usage.Embedding.Used), num(usage.Embedding.Remaining)},
			})
		},
	}
}
