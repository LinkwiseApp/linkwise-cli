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
