package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

type screen int

const (
	screenHome screen = iota
	screenBrowse
	screenCollections
	screenReader
	screenSearch
	screenAccount
	screenHelp
)

// Options is everything the interface needs from the outside. Passed in
// rather than resolved here, so the package stays free of configuration
// precedence and can be driven by a test with a fake server.
type Options struct {
	Client  *api.Client
	Version string
	Dark    bool

	// SaveTheme persists a theme change. Optional: a failure to write the
	// config file must not interrupt someone who only pressed a key, so the
	// error is shown on the status line and nothing else happens.
	SaveTheme func(dark bool) error
}

// Model is the whole interface. One model with a screen field, rather than a
// stack of independent tea.Models, because every screen shares the same
// header, status line and theme, and because a message can arrive for a
// screen that is not the one in front.
type Model struct {
	client    *api.Client
	version   string
	saveTheme func(bool) error

	dark          bool
	width, height int
	ready         bool

	screen screen
	// back is where esc goes. A stack rather than a fixed parent, because the
	// reader can be reached from browse, from a collection, or from search,
	// and esc should return to whichever one it was.
	back []screen

	home        home
	browse      browse
	collections collections
	reader      reader
	search      search
	account     account

	// status is a transient note, failure is a sticky error. Kept apart so
	// that a successful action does not wipe an error the user has not read,
	// and so an error cannot be mistaken for a confirmation.
	status    string
	failure   string
	statusGen int
}

func New(opts Options) Model {
	return Model{
		client:    opts.Client,
		version:   opts.Version,
		saveTheme: opts.SaveTheme,
		dark:      opts.Dark,
		screen:    screenHome,
		home:      newHome(),
		search:    newSearch(),
	}
}

func (m Model) theme() Theme { return themeFor(m.dark) }

// Init fetches only the collections, which the home menu counts and the
// collections screen needs anyway.
//
// There is deliberately no count of the library here. The list endpoint
// reports meta.total "only when the underlying query reports one", and for
// /links it never does, so asking would be a round trip per menu row that
// could only ever come back empty.
func (m Model) Init() tea.Cmd {
	return fetchCollections(m.client)
}

// statusExpiryMsg clears a note that has been on screen long enough to read.
type statusExpiryMsg int

// note returns the model as well as the command, so that no caller has to
// write `return m, m.note(...)`, where whether the returned m carries the
// note depends on an evaluation order Go does not promise.
func (m Model) note(text string) (Model, tea.Cmd) {
	m.status = text
	m.statusGen++
	gen := m.statusGen
	return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg { return statusExpiryMsg(gen) })
}

// fail records an error for the status line. The screen behind it keeps its
// last good data, so a failed refresh never empties a list that was fine.
func (m *Model) fail(err error) {
	if err == nil {
		return
	}
	// The API's own message is written for a person to read, so it is used
	// as-is. Anything else is a transport failure and only has a Go string.
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		m.failure = apiErr.Message
		return
	}
	m.failure = err.Error()
}

func (m *Model) push(s screen) {
	m.back = append(m.back, m.screen)
	m.screen = s
}

func (m *Model) pop() {
	if len(m.back) == 0 {
		m.screen = screenHome
		return
	}
	m.screen = m.back[len(m.back)-1]
	m.back = m.back[:len(m.back)-1]
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		// The article is wrapped to the window, so a resize invalidates it.
		// Done here rather than in the view, which has nowhere to put it.
		if m.reader.markdown != "" && m.reader.wrappedAt != readerTextWidth(m.width) {
			m.reader.rewrap(m.width)
		}
		return m, nil

	case statusExpiryMsg:
		if int(msg) == m.statusGen {
			m.status = ""
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m.handleData(msg)
}

// handleKey runs the bindings that work anywhere, then hands the rest to
// whichever screen is in front.
//
// The search field is checked first and unconditionally: while it has focus,
// `q` is a letter and `d` is a letter, and a global binding that swallowed
// them would make the field unusable.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.screen == screenSearch && m.search.typing {
		return m.updateSearch(msg)
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "d":
		m.dark = !m.dark
		if m.saveTheme != nil {
			if err := m.saveTheme(m.dark); err != nil {
				m.failure = "Theme changed for this session only: " + err.Error()
				return m, nil
			}
		}
		return m, nil

	case "?":
		if m.screen == screenHelp {
			m.pop()
			return m, nil
		}
		m.push(screenHelp)
		return m, nil

	case "esc":
		if m.screen == screenHome {
			return m, nil
		}
		m.pop()
		return m, nil

	case "/":
		if m.screen != screenSearch {
			m.push(screenSearch)
		}
		m.search.typing = true
		m.failure = ""
		return m, nil
	}

	switch m.screen {
	case screenHome:
		return m.updateHome(msg)
	case screenBrowse:
		return m.updateBrowse(msg)
	case screenCollections:
		return m.updateCollections(msg)
	case screenReader:
		return m.updateReader(msg)
	case screenSearch:
		return m.updateSearch(msg)
	case screenAccount:
		return m.updateAccount(msg)
	case screenHelp:
		return m, nil
	}
	return m, nil
}

// handleData routes every non-key message to the state it belongs to,
// regardless of which screen is in front. A page that arrives after the user
// has moved on still lands in the list it was fetched for.
func (m Model) handleData(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case collectionsMsg:
		m.collections.loading = false
		if msg.err != nil {
			m.fail(msg.err)
			return m, nil
		}
		m.collections.items = msg.items
		m.home.counts["collections"] = len(msg.items)
		return m, nil

	case linksMsg:
		return m.applyLinks(msg)

	case contentMsg:
		return m.applyContent(msg)

	case patchedMsg:
		return m.applyPatch(msg)

	case searchMsg:
		return m.applySearch(msg)

	case accountMsg:
		m.account.loading = false
		if msg.err != nil {
			m.fail(msg.err)
			return m, nil
		}
		m.account.me = msg.me
		m.account.usage = msg.usage
		m.account.loaded = true
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("Terminal too small. linkwise needs %d by %d.\n", minWidth, minHeight)
	}

	st := m.theme().styles()
	bodyHeight := m.height - 3

	var body []string
	switch m.screen {
	case screenHome:
		body = m.viewHome(st, bodyHeight)
	case screenBrowse:
		body = m.viewBrowse(st, bodyHeight)
	case screenCollections:
		body = m.viewCollections(st, bodyHeight)
	case screenReader:
		body = m.viewReader(st, bodyHeight)
	case screenSearch:
		body = m.viewSearch(st, bodyHeight)
	case screenAccount:
		body = m.viewAccount(st, bodyHeight)
	case screenHelp:
		body = m.viewHelp(st, bodyHeight)
	}

	for len(body) < bodyHeight {
		body = append(body, "")
	}
	body = body[:bodyHeight]
	// Padding that runs to the right margin is invisible on screen and only
	// makes a frame harder to read in a test. Styled padding, such as the
	// highlight behind a selected row, ends in an escape sequence and is left
	// alone by this.
	for i := range body {
		body[i] = strings.TrimRight(body[i], " ")
	}

	out := make([]string, 0, m.height)
	out = append(out, m.header(st), "")
	out = append(out, body...)
	out = append(out, m.statusLine(st))
	return strings.Join(out, "\n")
}

// header is the tinted bar across the top: the name and version on the left,
// the keys that work here on the right.
func (m Model) header(st styles) string {
	t := m.theme()
	left := " linkwise v" + m.version
	bar := lipgloss.NewStyle()
	if !noColor() {
		bar = bar.Background(t.Selected)
	}

	hints := helpBarText(hintsFor(m.screen, m.dark), m.width-lipgloss.Width(left)-2)
	right := hints + " "

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	name := bar.Bold(true)
	if !noColor() {
		name = name.Foreground(t.Accent)
	}
	rest := bar
	if !noColor() {
		rest = rest.Foreground(t.Dim)
	}

	return name.Render(left) + bar.Render(strings.Repeat(" ", gap)) + rest.Render(right)
}

func (m Model) statusLine(st styles) string {
	switch {
	case m.failure != "":
		return indent(st.bad.Render(truncate(m.failure, m.width-gutter)))
	case m.status != "":
		return indent(st.dim.Render(truncate(m.status, m.width-gutter)))
	default:
		return ""
	}
}
