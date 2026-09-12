package cli

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/spf13/cobra"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
	"github.com/LinkwiseApp/linkwise-cli/internal/render"
)

func (a *App) highlightsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "highlights [ls|add]", Short: "Read and create highlights"}

	var linkFilter string
	var limit int
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List highlights, newest first",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}

			query := url.Values{}
			if linkFilter != "" {
				query.Set("link_id", linkFilter)
			}
			if limit > 0 {
				query.Set("limit", strconv.Itoa(min(limit, 100)))
			}

			var all []api.Highlight
			rows := [][]string{}
			err = api.Each(cmd.Context(), env.Client,
				api.Request{Method: "GET", Path: "/highlights", Query: query},
				func(page []api.Highlight) error {
					for _, h := range page {
						if limit > 0 && len(all) >= limit {
							return api.ErrStop
						}
						all = append(all, h)
						rows = append(rows, []string{
							api.ShortID(h.ID),
							render.Truncate(h.SelectedText, 64),
							api.Day(h.CreatedAt),
						})
					}
					if limit > 0 && len(all) >= limit {
						return api.ErrStop
					}
					return nil
				})
			if err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteNDJSON(env.Out, all)
			}
			return render.WriteTable(env.Out, []string{"ID", "TEXT", "MADE"}, rows)
		},
	}
	ls.Flags().StringVar(&linkFilter, "link", "", "Only highlights on this link")
	ls.Flags().IntVar(&limit, "limit", 0, "Stop after this many highlights")

	var annotation string
	var start, end int
	add := &cobra.Command{
		Use:   "add <link-id> <text>",
		Short: "Create a highlight",
		Long: "The offsets a highlight needs are found by locating the text in the\n" +
			"article, so you only pass the text you highlighted. Pass --start and\n" +
			"--end yourself when the text appears more than once.",
		Args: usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Env()
			if err != nil {
				return err
			}
			if err := env.requireToken(); err != nil {
				return err
			}
			ctx := cmd.Context()

			linkID, text := args[0], args[1]

			// start_offset and end_offset are required, and a person at a
			// terminal has no way to know them. They are character positions
			// into the article's concatenated text content, which is what the
			// reader walks when it restores a highlight, so locating the text
			// in the stripped reader content reproduces the same number.
			//
			// Getting it wrong is not fatal: the reader checks the text at the
			// offset and falls back to searching for it. But a right answer
			// puts the highlight in the right place on the first try.
			if start == 0 && end == 0 {
				var content api.ReaderContent
				if _, err := env.Client.Do(ctx,
					api.Request{Method: "GET", Path: "/links/" + linkID + "/content"}, &content); err != nil {
					return err
				}

				plain := textContent(api.Str(content.HTMLContent, ""))
				at := strings.Index(plain, text)
				if at < 0 {
					return &api.Error{
						Code: "invalid_request",
						Message: "That text is not in the article. Check the wording, " +
							"or pass --start and --end yourself.",
					}
				}
				if strings.Contains(plain[at+len(text):], text) {
					return &api.Error{
						Code: "invalid_request",
						Message: "That text appears more than once, so the highlight would be " +
							"ambiguous. Pass --start and --end to say which one.",
					}
				}
				// Counted in UTF-16 code units, not bytes. The reader is
				// JavaScript, and a JavaScript string offset counts code
				// units, so an accent anywhere earlier in the article would
				// otherwise push every offset past it out by a byte.
				start = utf16Len(plain[:at])
				end = start + utf16Len(text)
			}

			body := map[string]any{
				"link_id":       linkID,
				"selected_text": text,
				"start_offset":  start,
				"end_offset":    end,
			}
			if annotation != "" {
				body["annotation"] = annotation
			}

			var created api.Highlight
			if _, err := env.Client.Do(ctx,
				api.Request{Method: "POST", Path: "/highlights", Body: body}, &created); err != nil {
				return err
			}

			if env.Mode == render.JSON {
				return render.WriteJSON(env.Out, created)
			}
			fmt.Fprintf(env.Out, "%s\n", created.ID)
			return nil
		},
	}
	// The flag is --note and the field is annotation, because that is what the
	// column is called. A highlight has one; a link does not, which is why
	// save takes --description and this takes --note.
	add.Flags().StringVar(&annotation, "note", "", "A note attached to the highlight")
	add.Flags().IntVar(&start, "start", 0, "Character offset where the highlight begins")
	add.Flags().IntVar(&end, "end", 0, "Character offset where it ends")

	cmd.AddCommand(ls, add)
	return cmd
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

// utf16Len is how long JavaScript thinks a string is: the number of UTF-16
// code units, which is one per character up to U+FFFF and two above it.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// textContent is the article's text with its markup removed, which is what the
// reader walks when it restores a highlight. The reader HTML is sanitised
// before it ever reaches us, with scripts and event handlers already stripped,
// so removing the remaining tags leaves exactly the text nodes it would visit.
func textContent(htmlSource string) string {
	return html.UnescapeString(tagPattern.ReplaceAllString(htmlSource, ""))
}
