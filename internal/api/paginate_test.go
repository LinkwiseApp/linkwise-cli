package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEachFollowsCursors(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("cursor") {
		case "":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []link{{ID: "1"}, {ID: "2"}},
				"meta": map[string]any{"has_more": true, "next_cursor": "p2"},
			})
		case "p2":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []link{{ID: "3"}},
				"meta": map[string]any{"has_more": false, "next_cursor": nil},
			})
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	var ids []string
	err := Each(context.Background(), c, Request{Method: "GET", Path: "/links"}, func(page []link) error {
		for _, l := range page {
			ids = append(ids, l.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Each: %v", err)
	}
	if len(ids) != 3 || ids[2] != "3" {
		t.Fatalf("ids = %v", ids)
	}
	if len(seen) != 2 || seen[1] != "p2" {
		t.Fatalf("cursors = %v", seen)
	}
}

// `ls --limit 5` against a server that pages in 25s finishes mid-page. Stopping
// must not look like a failure.
func TestEachStopsWithoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []link{{ID: "1"}},
			"meta": map[string]any{"has_more": true, "next_cursor": "always-more"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	pages := 0
	err := Each(context.Background(), c, Request{Method: "GET", Path: "/links"}, func(page []link) error {
		pages++
		return ErrStop
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if pages != 1 {
		t.Fatalf("pages = %d", pages)
	}
}
