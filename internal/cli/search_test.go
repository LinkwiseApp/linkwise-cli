package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchRejectsSemanticLocally(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	var errOut bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&errOut)
	root.SetArgs([]string{"search", "go", "--mode", "semantic", "--api", srv.URL, "--token", "lw_pat_x"})

	err := root.Execute()
	if err == nil {
		t.Fatal("want an error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d, want 2: nothing was sent", got)
	}
	if called {
		t.Fatal("a mode the API does not accept must not reach the API")
	}
	if !strings.Contains(err.Error(), "hybrid") {
		t.Fatalf("the error should point at hybrid, got %q", err.Error())
	}
}

func TestSearchSendsQueryAndMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "postgres" || r.URL.Query().Get("mode") != "fast" {
			t.Errorf("query = %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		// Buckets, not a flat list. The spec declares this route's data as an
		// array of SearchResult and the live route returns an object keyed by
		// what matched, which is what the CLI has to read.
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"links":       []map[string]any{{"id": "l1", "title": "Postgres tips", "url": "https://a.test"}},
				"collections": []map[string]any{},
				"highlights":  []map[string]any{},
			},
			"meta": map[string]any{"mode": "fast"},
		})
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"search", "postgres", "--mode", "fast", "--api", srv.URL, "--token", "lw_pat_x", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "Postgres tips") {
		t.Fatalf("output = %q", out.String())
	}
}

// The highlights route is the one that does return a flat array, and its rows
// carry the highlighted text rather than a link title.
func TestSearchHighlightsUsesTheOtherEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/highlights" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("mode") != "" {
			t.Errorf("the highlights route takes no mode, got %q", r.URL.Query().Get("mode"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "h1", "link_id": "l1", "selected_text": "a sentence worth keeping"},
		}})
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"search", "anything", "--highlights", "--api", srv.URL, "--token", "lw_pat_x", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "a sentence worth keeping") {
		t.Fatalf("output = %q", out.String())
	}
}

// mode=fast returns everything it found in one response and ignores limit, so
// the cap has to be applied here or `search --limit 5` prints fifteen rows.
func TestSearchAppliesLimitItselfBecauseFastIgnoresIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		links := []map[string]any{}
		for i := range 15 {
			links = append(links, map[string]any{"id": fmt.Sprintf("l%d", i), "title": "A hit"})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"links": links},
			"meta": map[string]any{"mode": "fast"},
		})
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"search", "anything", "--mode", "fast", "--limit", "3",
		"--api", srv.URL, "--token", "lw_pat_x", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.Count(strings.TrimRight(out.String(), "\n"), "\n") + 1; got != 3 {
		t.Fatalf("want 3 rows, got %d: %q", got, out.String())
	}
}

// What else matched is worth knowing about, and stdout has to stay pipeable,
// so it goes to stderr.
func TestSearchMentionsTheOtherBucketsOnStderr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"links":       []map[string]any{{"id": "l1", "title": "A hit"}},
				"collections": []map[string]any{{"id": "c1"}, {"id": "c2"}},
				"highlights":  []map[string]any{{"id": "h1"}},
			},
		})
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"search", "anything", "--api", srv.URL, "--token", "lw_pat_x", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(errOut.String(), "2 collections") || !strings.Contains(errOut.String(), "1 highlight") {
		t.Fatalf("stderr = %q", errOut.String())
	}
	if strings.Contains(out.String(), "collections") {
		t.Fatal("stdout must stay pipeable")
	}
}

// The offsets a highlight needs are character positions into the article text,
// which nobody types by hand. Locating the text is what makes `highlights add`
// usable from a terminal at all.
func TestHighlightsAddFindsTheOffsets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/content"):
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"link_id":      "l1",
					"html_content": "<p>Hello there. <b>The quick fox</b> jumped.</p>",
				},
			})
		case r.URL.Path == "/highlights" && r.Method == "POST":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			// "Hello there. " is 13 characters once the markup is gone, and
			// "The quick fox" is another 13.
			if body["start_offset"] != float64(13) || body["end_offset"] != float64(26) {
				t.Errorf("offsets = %v to %v", body["start_offset"], body["end_offset"])
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "h1", "link_id": "l1"}})
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"highlights", "add", "l1", "The quick fox", "--api", srv.URL, "--token", "lw_pat_x"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

// The reader counts offsets the way JavaScript does, in UTF-16 code units, so
// an article with an accent in it before the highlight must not shift by the
// extra bytes that character takes up in Go.
func TestHighlightsAddCountsOffsetsTheWayTheReaderDoes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/content"):
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"link_id": "l1",
					// "café " is five UTF-16 code units and six bytes.
					"html_content": "<p>café <b>target</b></p>",
				},
			})
		case r.URL.Path == "/highlights" && r.Method == "POST":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["start_offset"] != float64(5) {
				t.Errorf("start = %v, want 5 code units rather than 6 bytes", body["start_offset"])
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "h1", "link_id": "l1"}})
		}
	}))
	defer srv.Close()

	root := NewRoot("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"highlights", "add", "l1", "target", "--api", srv.URL, "--token", "lw_pat_x"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

// Text that appears twice would land the highlight in the wrong place, and
// guessing which one was meant is worse than saying so.
func TestHighlightsAddRefusesAmbiguousText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/highlights" && r.Method == "POST" {
			t.Error("an ambiguous highlight must not be sent")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"link_id": "l1", "html_content": "<p>same words and then same words</p>"},
		})
	}))
	defer srv.Close()

	root := NewRoot("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"highlights", "add", "l1", "same words", "--api", srv.URL, "--token", "lw_pat_x"})

	if err := root.Execute(); err == nil {
		t.Fatal("want an error for text that appears more than once")
	}
}
