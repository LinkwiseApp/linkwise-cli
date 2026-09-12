package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLsPaginatesUntilTheServerStops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "1", "url": "https://a.test", "created_at": "2026-09-01T00:00:00Z"}},
				"meta": map[string]any{"has_more": true, "next_cursor": "p2"},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "2", "url": "https://b.test", "created_at": "2026-09-02T00:00:00Z"}},
				"meta": map[string]any{"has_more": false, "next_cursor": nil},
			})
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"ls", "--api", srv.URL, "--token", "lw_pat_x", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 NDJSON lines, got %d: %q", len(lines), out.String())
	}
}

// --limit 1 must stop after one record even though the server offers more.
func TestLsHonoursLimitAcrossPages(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "1", "url": "https://a.test", "created_at": "2026-09-01T00:00:00Z"},
				{"id": "2", "url": "https://b.test", "created_at": "2026-09-02T00:00:00Z"},
			},
			"meta": map[string]any{"has_more": true, "next_cursor": "more"},
		})
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"ls", "--api", srv.URL, "--token", "lw_pat_x", "--json", "--limit", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.Count(strings.TrimRight(out.String(), "\n"), "\n") + 1; got != 1 {
		t.Fatalf("want 1 record, got %d: %q", got, out.String())
	}
	if requests != 1 {
		t.Fatalf("want 1 request, got %d", requests)
	}
}

// The published example filters by a collection name, so names have to
// resolve. A name that matches nothing is a not_found, exit code 5.
func TestLsResolvesACollectionName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/collections":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"collection_id": "col-1", "collection_name": "reading"}},
				"meta": map[string]any{"has_more": false, "next_cursor": nil},
			})
		case "/links":
			if got := r.URL.Query().Get("collection_id"); got != "col-1" {
				t.Errorf("collection_id = %q, want col-1", got)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{},
				"meta": map[string]any{"has_more": false, "next_cursor": nil},
			})
		}
	}))
	defer srv.Close()

	root := NewRoot("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"ls", "--api", srv.URL, "--token", "lw_pat_x", "--json", "--collection", "reading"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

// A collection the account does not have is a not_found, which the published
// exit-code table says is 5.
func TestLsUnknownCollectionIsExitCodeFive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
			"meta": map[string]any{"has_more": false, "next_cursor": nil},
		})
	}))
	defer srv.Close()

	root := NewRoot("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"ls", "--api", srv.URL, "--token", "lw_pat_x", "--collection", "nosuchthing"})

	err := root.Execute()
	if err == nil {
		t.Fatal("want an error for a collection that does not exist")
	}
	if got := ExitCode(err); got != 5 {
		t.Fatalf("exit code = %d, want 5", got)
	}
}

func TestSaveSendsUrlAndFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/collections":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"collection_id": "col-1", "collection_name": "reading"}},
				"meta": map[string]any{"has_more": false},
			})
		case "/links":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["url"] != "https://example.com" {
				t.Errorf("url = %v", body["url"])
			}
			if body["description"] != "why I saved it" {
				t.Errorf("description = %v, the field is description, not note", body["description"])
			}
			if body["collection_id"] != "col-1" {
				t.Errorf("collection_id = %v", body["collection_id"])
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"id": "link-1", "url": "https://example.com", "created_at": "2026-09-10T00:00:00Z"},
			})
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"save", "https://example.com", "--api", srv.URL, "--token", "lw_pat_x",
		"--json", "--description", "why I saved it", "--collection", "reading"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "link-1") {
		t.Fatalf("output = %q", out.String())
	}
}

// A URL is required. Catching that here means nothing is sent, which is what
// makes it a usage error and an exit code of 2.
func TestSaveWithoutAUrlIsAUsageError(t *testing.T) {
	root := NewRoot("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"save"})

	err := root.Execute()
	if err == nil {
		t.Fatal("want an error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
}

func TestReadPrintsMarkdownByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"link_id":      "link-1",
				"title":        "A title",
				"html_content": "<h1>A title</h1><p>Body text.</p>",
			},
		})
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"read", "11111111-1111-1111-1111-111111111111", "--api", srv.URL, "--token", "lw_pat_x"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Converted, not passed through: the heading arrives as Markdown and no
	// tag survives.
	if !strings.Contains(out.String(), "Body text.") || strings.Contains(out.String(), "<h1>") {
		t.Fatalf("output = %q", out.String())
	}
	if !strings.Contains(out.String(), "# A title") {
		t.Fatalf("want a Markdown heading, got %q", out.String())
	}
}

// --raw is the escape hatch: it prints what the API sent, so a script can do
// its own parsing instead of reading ours back out of Markdown.
func TestReadRawPrintsTheHtml(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"link_id": "link-1", "html_content": "<h1>A title</h1>"},
		})
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"read", "11111111-1111-1111-1111-111111111111", "--raw",
		"--api", srv.URL, "--token", "lw_pat_x"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "<h1>") {
		t.Fatalf("output = %q", out.String())
	}
}
