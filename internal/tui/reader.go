package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

// railWidth is the metadata column on the left of the reader. Wide enough
// for a two-line title and a byline, narrow enough to leave the article a
// comfortable measure at 80 columns.
const railWidth = 22

type reader struct {
	link     api.Link
	content  api.ReaderContent
	markdown string
	offset   int
	loading  bool
	// wrapped is the article at the width it was last laid out for, cached
	// so that scrolling does not re-wrap the whole piece on every keystroke.
	wrapped     []string
	wrappedAt   int
	missingText string
}

func (m Model) openReader(link api.Link) (tea.Model, tea.Cmd) {
	m.reader = reader{link: link, loading: true}
	m.failure = ""
	m.push(screenReader)
	return m, fetchContent(m.client, link.ID)
}

func (m Model) applyContent(msg contentMsg) (tea.Model, tea.Cmd) {
	if msg.linkID != m.reader.link.ID {
		return m, nil
	}
	m.reader.loading = false
	if msg.err != nil {
		// Extraction runs after a link is saved, so asking too soon is a
		// normal thing to do. It belongs in the pane as an explanation, not
		// on the error line as a failure.
		var apiErr *api.Error
		if errors.As(msg.err, &apiErr) && apiErr.Code == "not_found" {
			m.reader.missingText = apiErr.Message
			return m, nil
		}
		m.fail(msg.err)
		return m, nil
	}

	m.reader.content = msg.content
	m.reader.markdown = msg.markdown
	m.reader.offset = 0
	m.reader.rewrap(m.width)
	if strings.TrimSpace(msg.markdown) == "" {
		m.reader.missingText = "No reader content for this link. Press o to open it in the browser."
	}
	return m, nil
}

func (m Model) updateReader(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	page := m.height - 6
	if page < 1 {
		page = 1
	}
	total := len(m.reader.wrapped)
	maxOffset := total - page
	if maxOffset < 0 {
		maxOffset = 0
	}

	switch msg.String() {
	case "j", "down":
		m.reader.offset = clampTo(m.reader.offset+1, maxOffset)
	case "k", "up":
		m.reader.offset = clampTo(m.reader.offset-1, maxOffset)
	case " ", "pgdown", "f":
		m.reader.offset = clampTo(m.reader.offset+page, maxOffset)
	case "b", "pgup":
		m.reader.offset = clampTo(m.reader.offset-page, maxOffset)
	case "g", "home":
		m.reader.offset = 0
	case "G", "end":
		m.reader.offset = maxOffset
	case "o":
		openInBrowser(m.reader.link.URL)
		m, cmd := m.note("Opened " + hostOf(m.reader.link.URL))
		return m, cmd
	case "y":
		m, cmd := m.copyURL(m.reader.link.URL)
		return m, cmd
	case "e", "u", "p":
		return m, mutationFor(m.client, m.reader.link, msg.String())
	}
	return m, nil
}

func clampTo(i, max int) int {
	if i < 0 {
		return 0
	}
	if i > max {
		return max
	}
	return i
}

func (m Model) viewReader(st styles, height int) []string {
	actions := []hint{
		{"e", archiveLabel(m.reader.link)},
		{"u", readLabel(m.reader.link)},
		{"p", pinLabel(m.reader.link)},
		{"o", "Browser"},
		{"y", "Copy URL"},
	}
	out := []string{indent(actionBar(st, actions, m.width-gutter)), ""}

	// Below this the rail and the article would each be too narrow to read,
	// so the rail is dropped and the article gets the whole window.
	rail := railWidth
	if m.width < 72 {
		rail = 0
	}
	body := m.articleLines(readerTextWidth(m.width))
	visible := height - 2

	var railLines []string
	if rail > 0 {
		railLines = m.railLines(st, rail)
	}

	for i := 0; i < visible; i++ {
		left := ""
		if rail > 0 {
			if i < len(railLines) {
				left = railLines[i]
			}
			left = pad(left, rail) + "   "
		}

		right := ""
		if j := m.reader.offset + i; j >= 0 && j < len(body) {
			right = body[j]
		}
		out = append(out, indent(left+right))
	}
	return out
}

// readerTextWidth is the measure the article is wrapped to. Shared by the
// view and by rewrap, because a cache keyed on a width computed two
// different ways is a cache that never hits.
func readerTextWidth(windowWidth int) int {
	rail := railWidth + 3
	if windowWidth < 72 {
		rail = 0
	}
	w := windowWidth - gutter*2 - rail
	if w < 20 {
		return 20
	}
	return w
}

// rewrap lays the article out for a window width and caches the result.
// Wrapping a long piece is the only thing here expensive enough to be felt
// while scrolling, so it happens on arrival and on resize, never per frame.
func (r *reader) rewrap(windowWidth int) {
	width := readerTextWidth(windowWidth)
	r.wrapped = wrap(r.markdown, width)
	r.wrappedAt = width
}

func (m Model) articleLines(width int) []string {
	if m.reader.loading {
		return []string{"Loading…"}
	}
	if m.reader.missingText != "" {
		return wrap(m.reader.missingText, width)
	}
	if m.reader.wrappedAt == width {
		return m.reader.wrapped
	}
	return wrap(m.reader.markdown, width)
}

func (m Model) railLines(st styles, width int) []string {
	c := m.reader.content
	title := api.Str(c.Title, api.Str(m.reader.link.Title, m.reader.link.URL))

	out := wrap(title, width)
	for i := range out {
		out[i] = st.accent.Render(out[i])
	}
	out = append(out, "")

	if by := api.Str(c.Byline, ""); by != "" {
		out = append(out, st.text.Render(truncate(by, width)))
	}
	site := api.Str(c.SiteName, hostOf(m.reader.link.URL))
	out = append(out, st.faint.Render(truncate(site, width)), "")

	out = append(out, st.faint.Render("Status"))
	out = append(out, statusStyle(st, m.reader.link).Render(statusOf(m.reader.link)), "")

	if c.WordCount != nil {
		out = append(out, st.faint.Render("Words"), st.text.Render(thousands(*c.WordCount)))
	}
	return out
}

// actionBar is the row of bracketed keys along the top of the reader, as in
// [e] Archive  [o] Browser. Labels reflect the link's current state, so the
// key never offers to archive something already archived.
func actionBar(st styles, hints []hint, width int) string {
	var parts []string
	for _, h := range hints {
		parts = append(parts, st.accent.Render("["+h.key+"]")+" "+st.text.Render(h.label))
	}
	bar := strings.Join(parts, "   ")
	if lipgloss.Width(bar) <= width {
		return bar
	}
	// Dropping whole actions from the right, so the bar never ends mid-label.
	for n := len(parts) - 1; n > 0; n-- {
		bar = strings.Join(parts[:n], "   ")
		if lipgloss.Width(bar) <= width {
			return bar
		}
	}
	return bar
}

func archiveLabel(l api.Link) string {
	if l.ArchivedAt != nil {
		return "Restore"
	}
	return "Archive"
}

func readLabel(l api.Link) string {
	if l.IsVisited != nil && *l.IsVisited {
		return "Unread"
	}
	return "Read"
}

func pinLabel(l api.Link) string {
	if l.IsPinned != nil && *l.IsPinned {
		return "Unpin"
	}
	return "Pin"
}
