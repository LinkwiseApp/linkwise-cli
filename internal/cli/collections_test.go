package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollectionsLsListsThem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"collection_id": "c1", "collection_name": "reading"}, {"collection_id": "c2", "collection_name": "later"}},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"collections", "ls", "--api", srv.URL, "--token", "lw_pat_x", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "reading") || !strings.Contains(out.String(), "later") {
		t.Fatalf("output = %q", out.String())
	}
}

// PUT replaces the whole set. The command is named `tag` and the help has to
// say "replace", because a user who expects it to add will lose tags.
func TestTagReplacesTheWholeSet(t *testing.T) {
	var sawPut bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/tags") && r.Method == "PUT":
			sawPut = true
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			names, _ := body["tags"].([]any)
			if len(names) != 2 || names[0] != "go" || names[1] != "cli" {
				t.Errorf("tags = %v, the endpoint takes names and creates missing ones", names)
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		case r.URL.Path == "/tags":
			// Reaching this would mean the command resolved names to ids,
			// which is a lookup the endpoint makes unnecessary and which
			// would fail for a tag that does not exist yet.
			t.Error("tag must not look tags up: it sends names")
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "meta": map[string]any{"has_more": false}})
		}
	}))
	defer srv.Close()

	root := NewRoot("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"tag", "11111111-1111-1111-1111-111111111111", "go", "cli",
		"--api", srv.URL, "--token", "lw_pat_x"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !sawPut {
		t.Fatal("the tags were never sent")
	}
}
