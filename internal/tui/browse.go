package tui

import (
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

type browse struct {
	filter  Filter
	items   []api.Link
	cursor  int
	offset  int
	total   *int
	next    string
	loading bool

	// gen rises on every filter change. A page that was already in flight
	// carries the old generation and is dropped on arrival, so switching
	// filters quickly cannot mix two libraries into one list.
	gen int
}

func (m Model) openBrowse(f Filter) (tea.Model, tea.Cmd) {
	m.browse.gen++
	m.browse.filter = f
	m.browse.items = nil
	m.browse.cursor = 0
	m.browse.offset = 0
	m.browse.next = ""
	m.browse.total = nil
	m.browse.loading = true
	m.push(screenBrowse)
	return m, fetchLinks(m.client, f, "", m.browse.gen, false)
}

func (m Model) applyLinks(msg linksMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.browse.gen {
		return m, nil
	}
	m.browse.loading = false
	if msg.err != nil {
		m.fail(msg.err)
		return m, nil
	}

	m.failure = ""
	if msg.appending {
		m.browse.items = append(m.browse.items, msg.items...)
	} else {
		m.browse.items = msg.items
	}
	m.browse.next = msg.next
	if msg.total != nil {
		m.browse.total = msg.total
	}
	m.browse.cursor = clamp(m.browse.cursor, len(m.browse.items))
	return m, nil
}

// rowsVisible is how many list rows fit, leaving one line for the screen
// heading, one blank after it, and one for the "more" footer.
func rowsVisible(height int) int {
	n := height - 3
	if n < 1 {
		return 1
	}
	return n
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.browse.items)
	rows := rowsVisible(m.height - 3)

	switch msg.String() {
	case "j", "down":
		m.browse.cursor = clamp(m.browse.cursor+1, n)
	case "k", "up":
		m.browse.cursor = clamp(m.browse.cursor-1, n)
	case "ctrl+d":
		m.browse.cursor = clamp(m.browse.cursor+rows/2, n)
	case "ctrl+u":
		m.browse.cursor = clamp(m.browse.cursor-rows/2, n)
	case "g", "home":
		m.browse.cursor = 0
	case "G", "end":
		m.browse.cursor = clamp(n-1, n)
	case "r":
		return m.openBrowse(m.browse.filter)
	case "enter", "l", "right":
		if link, ok := m.selectedLink(); ok {
			return m.openReader(link)
		}
		return m, nil
	case "o":
		if link, ok := m.selectedLink(); ok {
			openInBrowser(link.URL)
			m, cmd := m.note("Opened " + hostOf(link.URL))
			return m, cmd
		}
		return m, nil
	case "y":
		if link, ok := m.selectedLink(); ok {
			m, cmd := m.copyURL(link.URL)
			return m, cmd
		}
		return m, nil
	case "e", "u", "p":
		if link, ok := m.selectedLink(); ok {
			return m, mutationFor(m.client, link, msg.String())
		}
		return m, nil
	}

	// The scroll window is decided here rather than in the view, because the
	// view cannot remember anything: it is a pure function of the model, and
	// a window derived fresh each frame would snap rather than scroll.
	m.browse.offset = window(m.browse.cursor, m.browse.offset, rows, n)

	// Fetching one screenful before the end, so that scrolling into the next
	// page usually finds it already there.
	if m.browse.next != "" && !m.browse.loading && m.browse.cursor >= len(m.browse.items)-rows {
		m.browse.loading = true
		return m, fetchLinks(m.client, m.browse.filter, m.browse.next, m.browse.gen, true)
	}
	return m, nil
}

func (m Model) selectedLink() (api.Link, bool) {
	if len(m.browse.items) == 0 {
		return api.Link{}, false
	}
	return m.browse.items[clamp(m.browse.cursor, len(m.browse.items))], true
}

// mutationFor turns a key into the PATCH it stands for. One place decides
// what `e`, `u` and `p` mean, so the reader's action bar and the list agree
// by construction.
func mutationFor(c *api.Client, link api.Link, key string) tea.Cmd {
	switch key {
	case "e":
		archived := link.ArchivedAt == nil
		note := "Archived."
		if !archived {
			note = "Restored from the archive."
		}
		return patchLink(c, link.ID, note, map[string]any{"archived": archived})
	case "u":
		visited := !(link.IsVisited != nil && *link.IsVisited)
		note := "Marked read."
		if !visited {
			note = "Marked unread."
		}
		return patchLink(c, link.ID, note, map[string]any{"is_visited": visited})
	case "p":
		pinned := !(link.IsPinned != nil && *link.IsPinned)
		note := "Pinned."
		if !pinned {
			note = "Unpinned."
		}
		return patchLink(c, link.ID, note, map[string]any{"is_pinned": pinned})
	}
	return nil
}

// applyPatch folds a successful mutation back into the row in place.
//
// Refetching the page would be simpler and worse: it would lose the scroll
// position, and it would make archiving from the middle of a long list jump
// under the cursor.
func (m Model) applyPatch(msg patchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.fail(msg.err)
		return m, nil
	}
	m.failure = ""

	if i, ok := linkByID(m.browse.items, msg.linkID); ok {
		link := m.browse.items[i]
		if v, ok := msg.patch["archived"].(bool); ok {
			if v {
				now := "archived"
				link.ArchivedAt = &now
			} else {
				link.ArchivedAt = nil
			}
		}
		if v, ok := msg.patch["is_visited"].(bool); ok {
			link.IsVisited = &v
		}
		if v, ok := msg.patch["is_pinned"].(bool); ok {
			link.IsPinned = &v
		}
		m.browse.items[i] = link
	}
	if m.reader.link.ID == msg.linkID {
		if v, ok := msg.patch["archived"].(bool); ok {
			if v {
				now := "archived"
				m.reader.link.ArchivedAt = &now
			} else {
				m.reader.link.ArchivedAt = nil
			}
		}
		if v, ok := msg.patch["is_visited"].(bool); ok {
			m.reader.link.IsVisited = &v
		}
		if v, ok := msg.patch["is_pinned"].(bool); ok {
			m.reader.link.IsPinned = &v
		}
	}

	m, cmd := m.note(msg.note)
	return m, cmd
}

func (m Model) viewBrowse(st styles, height int) []string {
	heading := "Library: " + m.browse.filter.Label
	if m.browse.total != nil {
		heading += "  " + st.faint.Render(thousands(*m.browse.total))
	}
	out := []string{indent(st.accent.Render(heading)), ""}

	if len(m.browse.items) == 0 {
		if m.browse.loading {
			return append(out, indent(st.dim.Render("Loading…")))
		}
		return append(out, indent(st.dim.Render(emptyFor(m.browse.filter))))
	}

	rows := rowsVisible(height)
	offset := window(m.browse.cursor, m.browse.offset, rows, len(m.browse.items))
	cols := columnsFor(m.width)

	end := min(offset+rows, len(m.browse.items))
	for i := offset; i < end; i++ {
		out = append(out, m.row(st, m.browse.items[i], cols, i == m.browse.cursor))
	}

	if m.browse.next != "" {
		out = append(out, indent(st.faint.Render("Scroll down for more…")))
	}
	return out
}

// columns is the width of each fixed column, with the title taking whatever
// is left. Columns drop out entirely on a narrow window rather than being
// squeezed to an unreadable four characters.
type columns struct {
	title  int
	host   int
	status int
	date   int
}

func columnsFor(width int) columns {
	avail := width - gutter*2 - 2 // two for the cursor marker
	c := columns{host: 18, status: 8, date: 6}

	title := avail - c.host - c.status - c.date - 3
	if title < 24 {
		c.host = 0
		title = avail - c.status - c.date - 2
	}
	if title < 18 {
		c.status = 0
		title = avail - c.date - 1
	}
	if title < 12 {
		c.date = 0
		title = avail
	}
	c.title = title
	return c
}

func (m Model) row(st styles, l api.Link, c columns, selected bool) string {
	title := api.Str(l.Title, l.URL)

	parts := []string{cell(title, c.title)}
	if c.host > 0 {
		parts = append(parts, st.faint.Render(cell(hostOf(l.URL), c.host)))
	}
	if c.status > 0 {
		parts = append(parts, statusStyle(st, l).Render(cell(statusOf(l), c.status)))
	}
	if c.date > 0 {
		parts = append(parts, st.faint.Render(cell(shortDate(l.CreatedAt), c.date)))
	}
	body := strings.Join(parts, " ")

	marker := "  "
	if selected {
		marker = st.accent.Render("> ")
		// Only the title is highlighted. Carrying the highlight across the
		// whole row would paint a block the width of the terminal, which is
		// louder than a cursor needs to be.
		parts[0] = st.selected.Render(pad(truncate(title, c.title), c.title))
		body = strings.Join(parts, " ")
	}
	if l.IsPinned != nil && *l.IsPinned {
		body += st.accent.Render(" *")
	}
	return indent(marker + body)
}

func statusOf(l api.Link) string {
	switch {
	case l.ArchivedAt != nil:
		return "archived"
	case l.IsVisited != nil && *l.IsVisited:
		return "read"
	default:
		return "unread"
	}
}

func statusStyle(st styles, l api.Link) interface{ Render(...string) string } {
	switch {
	case l.ArchivedAt != nil:
		return st.faint
	case l.IsVisited != nil && *l.IsVisited:
		return st.good
	default:
		return st.accent
	}
}

func emptyFor(f Filter) string {
	switch f.Key {
	case "unread":
		return "Nothing unread. Everything here has been opened."
	case "archived":
		return "Nothing archived yet."
	case "all":
		return "Nothing saved yet. Try `linkwise save <url>`."
	default:
		return "Nothing in this collection yet."
	}
}

// hostOf is the column that tells you where something came from. www is
// dropped because it is never the part you read.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return strings.TrimPrefix(u.Host, "www.")
}

var months = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// shortDate turns an RFC 3339 timestamp into "Sep 04" without parsing it.
//
// The date part of that format is fixed width, so slicing it is safe and
// cannot fail on a timestamp with an unusual offset, which time.Parse would
// reject outright and leave the cell empty.
func shortDate(ts string) string {
	if len(ts) < 10 {
		return ts
	}
	mm := ts[5:7]
	i := int(mm[0]-'0')*10 + int(mm[1]-'0')
	if i < 1 || i > 12 {
		return ts[:10]
	}
	return months[i-1] + " " + ts[8:10]
}
