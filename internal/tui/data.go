package tui

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

// This file is the only one in the package that knows the API exists. Every
// other file is a function from state to a string, which is what lets the
// layout be tested without a server and the navigation without a terminal.

// requestTimeout bounds a single call. The shared client already has a
// sixty second timeout; this is shorter because a person is watching, and a
// stalled fetch should become a retryable status line rather than a minute
// of nothing.
const requestTimeout = 30 * time.Second

// pageSize is what one fetch asks for. The list endpoint caps at 100, and 50
// is comfortably more than a tall terminal shows, so scrolling rarely waits.
const pageSize = 50

// Filter is one way of looking at the library: a label for the header and
// the query that produces it.
type Filter struct {
	Key   string
	Label string
	Query url.Values
}

func filterAll() Filter {
	return Filter{Key: "all", Label: "all", Query: url.Values{}}
}

func filterUnread() Filter {
	return Filter{Key: "unread", Label: "unread", Query: url.Values{"is_visited": {"false"}}}
}

func filterArchived() Filter {
	return Filter{Key: "archived", Label: "archived", Query: url.Values{"archived": {"true"}}}
}

func filterCollection(c api.Collection) Filter {
	return Filter{
		Key:   "collection:" + c.ID(),
		Label: c.Name(),
		Query: url.Values{"collection_id": {c.ID()}},
	}
}

// linksMsg carries one page of the library back to the model.
//
// gen is a generation counter. Changing filter while a fetch is in flight
// would otherwise let the old filter's page land in the new filter's list,
// which reads as data corruption and is really just a race.
type linksMsg struct {
	gen       int
	appending bool
	items     []api.Link
	total     *int
	next      string
	err       error
}

func fetchLinks(c *api.Client, f Filter, cursor string, gen int, appending bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		q := url.Values{}
		for k, v := range f.Query {
			q[k] = append([]string(nil), v...)
		}
		q.Set("limit", strconv.Itoa(pageSize))
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		var page []api.Link
		meta, err := c.Do(ctx, api.Request{Method: "GET", Path: "/links", Query: q}, &page)
		msg := linksMsg{gen: gen, appending: appending, items: page, err: err}
		if meta != nil {
			msg.total = meta.Total
			if meta.NextCursor != nil {
				msg.next = *meta.NextCursor
			}
		}
		return msg
	}
}

type collectionsMsg struct {
	items []api.Collection
	err   error
}

func fetchCollections(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		var all []api.Collection
		err := api.Each(ctx, c, api.Request{Method: "GET", Path: "/collections"},
			func(page []api.Collection) error {
				all = append(all, page...)
				return nil
			})
		return collectionsMsg{items: all, err: err}
	}
}

// contentMsg is the reader payload, already converted.
//
// The conversion happens here rather than in the view because it is the one
// expensive thing the reader does, and doing it once per fetch instead of
// once per frame is the difference between smooth scrolling and none.
type contentMsg struct {
	linkID   string
	content  api.ReaderContent
	markdown string
	err      error
}

func fetchContent(c *api.Client, linkID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		var content api.ReaderContent
		_, err := c.Do(ctx, api.Request{Method: "GET", Path: "/links/" + linkID + "/content"}, &content)
		if err != nil {
			return contentMsg{linkID: linkID, err: err}
		}

		markdown := ""
		if html := api.Str(content.HTMLContent, ""); html != "" {
			markdown, err = htmltomarkdown.ConvertString(html)
			if err != nil {
				return contentMsg{linkID: linkID, content: content, err: err}
			}
			markdown = imageMarkdown.ReplaceAllString(markdown, "")
		}
		return contentMsg{linkID: linkID, content: content, markdown: markdown}
	}
}

// patchedMsg reports the result of a mutation, with the note to show on the
// status line when it worked.
type patchedMsg struct {
	linkID string
	note   string
	patch  map[string]any
	err    error
}

func patchLink(c *api.Client, linkID, note string, patch map[string]any) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		_, err := c.Do(ctx, api.Request{
			Method: "PATCH",
			Path:   "/links/" + linkID,
			Body:   patch,
		}, nil)
		return patchedMsg{linkID: linkID, note: note, patch: patch, err: err}
	}
}

type searchMsg struct {
	query string
	hits  []api.SearchHit
	err   error
}

func fetchSearch(c *api.Client, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		var results api.SearchResults
		_, err := c.Do(ctx, api.Request{
			Method: "GET",
			Path:   "/search",
			Query:  url.Values{"q": {query}, "limit": {strconv.Itoa(pageSize)}},
		}, &results)
		return searchMsg{query: query, hits: results.Links, err: err}
	}
}

type accountMsg struct {
	me    api.Me
	usage api.Usage
	err   error
}

func fetchAccount(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		var me api.Me
		if _, err := c.Do(ctx, api.Request{Method: "GET", Path: "/me"}, &me); err != nil {
			return accountMsg{err: err}
		}

		// Usage is fetched in the same command so the screen paints once with
		// everything, rather than twice with a gap. A usage failure is not
		// allowed to lose the account details that already arrived.
		var usage api.Usage
		if _, err := c.Do(ctx, api.Request{Method: "GET", Path: "/me/usage"}, &usage); err != nil {
			return accountMsg{me: me}
		}
		return accountMsg{me: me, usage: usage}
	}
}

// imageMarkdown matches an inline image. A terminal cannot draw one, so what
// survives conversion is a line of alt text and a URL in the middle of a
// paragraph, which is worse than nothing. Links are left alone: their text
// reads as part of the sentence, and the URL is the only way to follow one
// from here.
var imageMarkdown = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

// linkByID finds a link in a slice. The reader needs the row it came from to
// reflect a mutation without refetching the whole page.
func linkByID(items []api.Link, id string) (int, bool) {
	for i, l := range items {
		if l.ID == id {
			return i, true
		}
	}
	return 0, false
}
