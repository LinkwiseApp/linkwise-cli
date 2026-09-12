package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type link struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func TestDoDecodesDataAndMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer lw_pat_test" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []link{{ID: "a", Title: "One"}},
			"meta": map[string]any{"has_more": true, "next_cursor": "eyJwIjoyfQ"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "lw_pat_test", "test")
	var out []link
	meta, err := c.Do(context.Background(), Request{Method: "GET", Path: "/links"}, &out)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(out) != 1 || out[0].Title != "One" {
		t.Fatalf("data = %+v", out)
	}
	if meta == nil || !meta.HasMore || meta.NextCursor == nil || *meta.NextCursor != "eyJwIjoyfQ" {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestDoReturnsApiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req-123")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "not_found", "message": "Link not found"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	_, err := c.Do(context.Background(), Request{Method: "GET", Path: "/links/x"}, nil)

	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	if apiErr.Code != "not_found" || apiErr.Status != 404 || apiErr.RequestID != "req-123" {
		t.Fatalf("error = %+v", apiErr)
	}
	if apiErr.Error() != "Link not found" {
		t.Fatalf("message = %q, a 404 should not carry a request id", apiErr.Error())
	}
}

// 204 is what DELETE returns. There is no envelope to decode, and treating an
// empty body as malformed JSON would turn every successful delete into an
// error.
func TestDoAcceptsNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	if _, err := c.Do(context.Background(), Request{Method: "DELETE", Path: "/links/x"}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
}

// Something in front of the gateway answering with HTML must not surface as a
// JSON parse error, which would send the reader after the wrong problem.
func TestDoReportsNonEnvelopeBodyAsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<html>upstream is down</html>"))
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	_, err := c.Do(context.Background(), Request{Method: "GET", Path: "/links"}, nil)
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("want *Error, got %T", err)
	}
	if apiErr.Code != "internal" || apiErr.Status != 502 {
		t.Fatalf("error = %+v", apiErr)
	}
}

// The docs promise the CLI has already waited and retried once before it
// reports a rate limit, so exit code 6 means "I tried twice", not "I gave up".
func TestDoRetriesRateLimitOnce(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "rate_limited", "message": "Slow down"},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []link{{ID: "a"}}})
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	var out []link
	if _, err := c.Do(context.Background(), Request{Method: "GET", Path: "/links"}, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if calls != 2 {
		t.Fatalf("want 2 calls, got %d", calls)
	}
}

// A daily quota can reset hours away. Sleeping until then looks like a hang,
// and a hang is worse than an honest exit 6.
func TestDoDoesNotRetryLongWaits(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "rate_limited", "message": "Daily quota reached"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	start := time.Now()
	_, err := c.Do(context.Background(), Request{Method: "GET", Path: "/links"}, nil)
	if err == nil {
		t.Fatal("want an error")
	}
	if calls != 1 {
		t.Fatalf("want 1 call, got %d", calls)
	}
	if time.Since(start) > time.Second {
		t.Fatal("it slept")
	}
}

func TestDoSendsQueryAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Errorf("limit = %q", got)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["url"] != "https://example.com" {
			t.Errorf("body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": link{ID: "a"}})
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	var out link
	_, err := c.Do(context.Background(), Request{
		Method: "POST",
		Path:   "/links",
		Query:  url.Values{"limit": []string{"5"}},
		Body:   map[string]any{"url": "https://example.com"},
	}, &out)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
}

func TestDoRawReturnsTheBodyUntouched(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-opml+xml; charset=utf-8")
		w.Write([]byte(`<?xml version="1.0"?><opml version="2.0"></opml>`))
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	body, ct, err := c.DoRaw(context.Background(), Request{Method: "GET", Path: "/feeds/export"})
	if err != nil {
		t.Fatalf("DoRaw: %v", err)
	}
	if !strings.HasPrefix(string(body), "<?xml") {
		t.Fatalf("body = %q", body)
	}
	if !strings.Contains(ct, "opml") {
		t.Fatalf("content type = %q", ct)
	}
}

// An error on a raw route still arrives as the JSON envelope, and must still
// map to an exit code rather than being handed back as XML.
func TestDoRawStillDecodesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "unauthorized", "message": "Missing or invalid credentials"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "t", "test")
	_, _, err := c.DoRaw(context.Background(), Request{Method: "GET", Path: "/feeds/export"})
	if ExitCode(err) != 3 {
		t.Fatalf("exit code = %d, want 3", ExitCode(err))
	}
}
