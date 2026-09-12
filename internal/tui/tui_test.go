package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// Every test here runs with NO_COLOR set, which collapses the styles to plain
// text. That is what makes a golden file a readable picture of the layout
// rather than a wall of escape sequences, and it exercises the no-colour path
// the README promises at the same time.
func newTestModel(t *testing.T, w, h int) Model {
	t.Helper()
	t.Setenv("NO_COLOR", "1")

	m := New(Options{Version: "0.2.0", Dark: true})
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model)
}

func key(t *testing.T, m Model, k string) Model {
	t.Helper()
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "space":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

func ptr[T any](v T) *T { return &v }

func sampleLinks() []api.Link {
	return []api.Link{
		{
			ID: "11111111-1111-1111-1111-111111111111", URL: "https://supabase.com/blog/rls",
			Title: ptr("Postgres row-level security, explained"), CreatedAt: "2026-09-04T10:00:00Z",
		},
		{
			ID: "22222222-2222-2222-2222-222222222222", URL: "https://danluu.com/butler-lampson-1999/",
			Title: ptr("The Unreasonable Effectiveness of Plain Text"), IsVisited: ptr(true),
			CreatedAt: "2026-09-02T10:00:00Z",
		},
		{
			ID: "33333333-3333-3333-3333-333333333333", URL: "https://www.martinkleppmann.com/ddia.html",
			Title: ptr("Designing Data-Intensive Applications"), IsPinned: ptr(true),
			CreatedAt: "2026-08-29T10:00:00Z",
		},
		{
			ID: "44444444-4444-4444-4444-444444444444", URL: "https://example.com/no-title-yet",
			CreatedAt: "2026-08-21T10:00:00Z", ArchivedAt: ptr("2026-08-22T10:00:00Z"),
		},
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".txt")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test ./internal/tui -update` to create it)", err)
	}
	if got != string(want) {
		t.Errorf("%s does not match the golden file.\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestHomeLayout(t *testing.T) {
	m := newTestModel(t, 90, 32)
	m.home.counts = map[string]int{"all": 1284, "unread": 312, "archived": 941, "collections": 14}
	golden(t, "home-90x32", m.View())
}

// The block wordmark is 47 columns and the smallest window the interface
// draws at all is 60, so it has to fit there without wrapping. This is the
// test that keeps those two numbers in step.
func TestHomeFitsTheSmallestWindow(t *testing.T) {
	m := newTestModel(t, minWidth, minHeight)
	golden(t, "home-60x16", m.View())

	for _, line := range strings.Split(m.View(), "\n") {
		if len([]rune(line)) > minWidth {
			t.Errorf("%q is wider than the window", line)
		}
	}
	if !strings.Contains(m.View(), "█") {
		t.Error("the wordmark should still be drawn at the minimum width")
	}
}

func TestBrowseLayout(t *testing.T) {
	m := newTestModel(t, 90, 32)
	next, _ := m.openBrowse(filterAll())
	m = next.(Model)

	next, _ = m.applyLinks(linksMsg{gen: m.browse.gen, items: sampleLinks(), total: ptr(1284)})
	m = next.(Model)

	golden(t, "browse-90x32", m.View())
}

func TestReaderLayout(t *testing.T) {
	m := newTestModel(t, 90, 32)
	next, _ := m.openReader(sampleLinks()[0])
	m = next.(Model)

	next, _ = m.applyContent(contentMsg{
		linkID: sampleLinks()[0].ID,
		content: api.ReaderContent{
			LinkID: sampleLinks()[0].ID,
			Title:  ptr("Postgres row-level security, explained"),
			Byline: ptr("Paul Copplestone"), SiteName: ptr("Supabase"), WordCount: ptr(1204),
		},
		markdown: "Row-level security is the part of Postgres that people reach for " +
			"last and should reach for first.\n\nA policy is a predicate the planner " +
			"folds into every query against the table, which means it cannot be " +
			"forgotten at a call site.",
	})
	m = next.(Model)

	golden(t, "reader-90x32", m.View())
}

func TestTerminalTooSmallSaysSo(t *testing.T) {
	m := newTestModel(t, 40, 10)
	if !strings.Contains(m.View(), "Terminal too small") {
		t.Errorf("expected a size message, got %q", m.View())
	}
}

func TestEveryFrameIsExactlyTheWindowHeight(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {90, 32}, {120, 40}, {60, 16}} {
		m := newTestModel(t, size[0], size[1])
		for _, s := range []screen{screenHome, screenBrowse, screenCollections, screenReader, screenSearch, screenAccount, screenHelp} {
			m.screen = s
			got := len(strings.Split(m.View(), "\n"))
			if got != size[1] {
				t.Errorf("%dx%d screen %d: %d lines, want %d", size[0], size[1], s, got, size[1])
			}
		}
	}
}

// A frame wider than the window wraps in the terminal and shifts everything
// below it, which is the failure that makes a TUI look broken.
func TestNoLineOverflowsTheWindow(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.home.counts = map[string]int{"all": 1284, "unread": 312, "archived": 941, "collections": 14}

	next, _ := m.openBrowse(filterAll())
	m = next.(Model)
	next, _ = m.applyLinks(linksMsg{gen: m.browse.gen, items: sampleLinks(), total: ptr(1284)})
	m = next.(Model)

	for _, s := range []screen{screenHome, screenBrowse, screenReader, screenSearch, screenAccount, screenHelp} {
		m.screen = s
		for i, line := range strings.Split(m.View(), "\n") {
			if n := len([]rune(line)); n > 80 {
				t.Errorf("screen %d line %d is %d columns: %q", s, i, n, line)
			}
		}
	}
}

func TestHomeNavigationOpensBrowse(t *testing.T) {
	m := newTestModel(t, 90, 32)

	m = key(t, m, "j")
	if m.home.cursor != 1 {
		t.Fatalf("cursor is %d, want 1", m.home.cursor)
	}

	m = key(t, m, "enter")
	if m.screen != screenBrowse {
		t.Fatalf("screen is %d, want browse", m.screen)
	}
	if m.browse.filter.Key != "unread" {
		t.Errorf("filter is %q, want unread", m.browse.filter.Key)
	}
}

func TestEscReturnsToWhereYouCameFrom(t *testing.T) {
	m := newTestModel(t, 90, 32)

	next, _ := m.openBrowse(filterArchived())
	m = next.(Model)
	next, _ = m.applyLinks(linksMsg{gen: m.browse.gen, items: sampleLinks()})
	m = next.(Model)

	m = key(t, m, "enter") // into the reader
	if m.screen != screenReader {
		t.Fatalf("screen is %d, want reader", m.screen)
	}

	m = key(t, m, "esc")
	if m.screen != screenBrowse {
		t.Fatalf("esc from the reader landed on %d, want browse", m.screen)
	}

	m = key(t, m, "esc")
	if m.screen != screenHome {
		t.Fatalf("esc from browse landed on %d, want home", m.screen)
	}
}

// While the search field has focus, q is a letter. A global binding that
// swallowed it would make the field unusable, which is worth a test because
// the bug only shows up when someone searches for something with a q in it.
func TestSearchFieldTakesKeysThatAreGlobalElsewhere(t *testing.T) {
	m := newTestModel(t, 90, 32)

	m = key(t, m, "/")
	if m.screen != screenSearch || !m.search.typing {
		t.Fatalf("/ did not open the search field")
	}

	for _, k := range []string{"q", "u", "e", "r", "y", "space", "d"} {
		m = key(t, m, k)
	}
	if m.search.query != "query d" {
		t.Errorf("query is %q, want %q", m.search.query, "query d")
	}
	if m.dark != true {
		t.Error("d typed into the field should not have toggled the theme")
	}

	m = key(t, m, "backspace")
	if m.search.query != "query " {
		t.Errorf("after backspace the query is %q, want %q", m.search.query, "query ")
	}
}

func TestThemeTogglePersists(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var saved []bool
	m := New(Options{Version: "0.2.0", Dark: true, SaveTheme: func(d bool) error {
		saved = append(saved, d)
		return nil
	}})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 32})
	m = next.(Model)

	m = key(t, m, "d")
	if m.dark {
		t.Error("d did not switch to the light palette")
	}
	if len(saved) != 1 || saved[0] != false {
		t.Errorf("SaveTheme calls were %v, want [false]", saved)
	}
}

// A theme that cannot be written is still a theme that changed on screen.
// Failing the keystroke would be worse than a note about the file.
func TestThemeToggleSurvivesAFailedWrite(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	m := New(Options{Version: "0.2.0", Dark: true, SaveTheme: func(bool) error {
		return os.ErrPermission
	}})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 32})
	m = next.(Model)

	m = key(t, m, "d")
	if m.dark {
		t.Error("the palette should have changed for this session")
	}
	if !strings.Contains(m.failure, "this session only") {
		t.Errorf("failure is %q, want a note about the config file", m.failure)
	}
}

func TestArchiveUpdatesTheRowWithoutRefetching(t *testing.T) {
	m := newTestModel(t, 90, 32)
	next, _ := m.openBrowse(filterAll())
	m = next.(Model)
	next, _ = m.applyLinks(linksMsg{gen: m.browse.gen, items: sampleLinks()})
	m = next.(Model)

	id := sampleLinks()[0].ID
	next, _ = m.applyPatch(patchedMsg{linkID: id, note: "Archived.", patch: map[string]any{"archived": true}})
	m = next.(Model)

	if got := statusOf(m.browse.items[0]); got != "archived" {
		t.Errorf("row status is %q, want archived", got)
	}
	if m.status != "Archived." {
		t.Errorf("status line is %q, want the note", m.status)
	}
	if m.browse.cursor != 0 {
		t.Errorf("the cursor moved to %d", m.browse.cursor)
	}
}

// A page fetched for the previous filter must not land in the new one.
func TestAStalePageIsDropped(t *testing.T) {
	m := newTestModel(t, 90, 32)

	next, _ := m.openBrowse(filterAll())
	m = next.(Model)
	stale := m.browse.gen

	next, _ = m.openBrowse(filterUnread())
	m = next.(Model)

	next, _ = m.applyLinks(linksMsg{gen: stale, items: sampleLinks()})
	m = next.(Model)

	if len(m.browse.items) != 0 {
		t.Errorf("the stale page landed: %d rows", len(m.browse.items))
	}
}

func TestAFailedFetchKeepsTheRowsAlreadyOnScreen(t *testing.T) {
	m := newTestModel(t, 90, 32)
	next, _ := m.openBrowse(filterAll())
	m = next.(Model)
	next, _ = m.applyLinks(linksMsg{gen: m.browse.gen, items: sampleLinks()})
	m = next.(Model)

	next, _ = m.applyLinks(linksMsg{gen: m.browse.gen, err: &api.Error{
		Code: "internal", Message: "The server is having a moment.",
	}})
	m = next.(Model)

	if len(m.browse.items) != 4 {
		t.Errorf("rows were lost on a failed fetch: %d left", len(m.browse.items))
	}
	if m.failure != "The server is having a moment." {
		t.Errorf("failure is %q, want the API's own message", m.failure)
	}
}

// Extraction runs after a link is saved, so an empty reader is a normal
// state with an explanation, not an error line.
func TestMissingReaderContentExplainsItself(t *testing.T) {
	m := newTestModel(t, 90, 32)
	next, _ := m.openReader(sampleLinks()[0])
	m = next.(Model)

	next, _ = m.applyContent(contentMsg{linkID: sampleLinks()[0].ID, err: &api.Error{
		Code: "not_found", Message: "No reader content yet. Extraction runs after a link is saved.",
	}})
	m = next.(Model)

	if m.failure != "" {
		t.Errorf("failure line is %q, want it empty", m.failure)
	}
	if !strings.Contains(m.View(), "Extraction runs") {
		t.Error("the explanation is not in the pane")
	}
}

func TestColumnsDropRatherThanSqueeze(t *testing.T) {
	wide := columnsFor(120)
	if wide.host == 0 || wide.status == 0 || wide.date == 0 {
		t.Errorf("at 120 columns every column should be present, got %+v", wide)
	}

	narrow := columnsFor(60)
	if narrow.host != 0 {
		t.Errorf("at 60 columns the host column should be gone, got %+v", narrow)
	}
	if narrow.title < 12 {
		t.Errorf("the title column collapsed to %d", narrow.title)
	}
}

func TestHelpBarTruncatesOnASeparator(t *testing.T) {
	hints := hintsFor(screenBrowse, true)

	full := helpBarText(hints, 200)
	if !strings.Contains(full, "enter:select") {
		t.Errorf("the full bar is missing an entry: %q", full)
	}

	short := helpBarText(hints, 30)
	if len(short) > 30 {
		t.Errorf("the bar is %d columns, want 30 or fewer: %q", len(short), short)
	}
	for _, part := range strings.Split(short, " | ") {
		if !strings.Contains(part, ":") {
			t.Errorf("%q is a half entry, the bar should truncate between entries", part)
		}
	}
}

func TestScrollWindowFollowsTheCursor(t *testing.T) {
	cases := []struct{ cursor, offset, height, n, want int }{
		{0, 0, 10, 100, 0},
		{9, 0, 10, 100, 0},    // still on the first screenful
		{10, 0, 10, 100, 1},   // scrolled by exactly one
		{50, 45, 10, 100, 45}, // already visible, nothing moves
		{2, 45, 10, 100, 2},   // jumped back above the window
		{0, 0, 10, 3, 0},      // fewer rows than the window
	}
	for _, c := range cases {
		if got := window(c.cursor, c.offset, c.height, c.n); got != c.want {
			t.Errorf("window(%d, %d, %d, %d) = %d, want %d", c.cursor, c.offset, c.height, c.n, got, c.want)
		}
	}
}

func TestShortDate(t *testing.T) {
	cases := map[string]string{
		"2026-09-04T10:00:00Z":      "Sep 04",
		"2026-01-31T23:59:59+05:30": "Jan 31",
		"2026-13-01T00:00:00Z":      "2026-13-01", // not a month, so the raw date
		"":                          "",
	}
	for in, want := range cases {
		if got := shortDate(in); got != want {
			t.Errorf("shortDate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestThousands(t *testing.T) {
	cases := map[int]string{0: "0", 999: "999", 1000: "1,000", 1284: "1,284", 1234567: "1,234,567"}
	for in, want := range cases {
		if got := thousands(in); got != want {
			t.Errorf("thousands(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncateMeasuresDisplayWidth(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("got %q, want it untouched", got)
	}
	if got := truncate("hello world", 8); len([]rune(got)) > 8 {
		t.Errorf("%q is wider than 8", got)
	}
	// A wide rune counts as two cells, so four of them do not fit in five.
	if got := truncate("日本語です", 5); len([]rune(got)) > 3 {
		t.Errorf("%q is too wide: wide runes should count double", got)
	}
}
