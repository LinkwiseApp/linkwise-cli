package selfupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestNewerComparesVersions(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"0.1.0", "0.1.1", true},
		{"0.1.1", "0.1.1", false},
		{"0.1.1", "0.1.0", false},
		// Lexically "0.9.0" sorts after "0.10.0", which is the bug this
		// exists to not have.
		{"0.9.0", "0.10.0", true},
		{"1.0.0", "0.9.9", false},
		{"0.1.1", "1.0.0", true},
		// Tags carry a v; the version baked into the binary does not.
		{"0.1.0", "v0.1.1", true},
		{"v0.1.1", "0.1.1", false},
		// A `go build` with no ldflags reports "dev". Nagging someone who is
		// working on the CLI to go install a release of it is noise.
		{"dev", "0.1.1", false},
		{"", "0.1.1", false},
		{"0.1.0", "", false},
		{"0.1.0", "not-a-version", false},
		// A shorter version is not a smaller one.
		{"0.1", "0.1.0", false},
		{"0.1", "0.1.1", true},
	}

	for _, c := range cases {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestLatestReadsTheRedirectTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/LinkwiseApp/linkwise-cli/releases/tag/v0.2.0", http.StatusFound)
	}))
	defer srv.Close()

	got, err := Latest(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if want := "0.2.0"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The redirect is the answer, so following it would download a release page
// to learn something already in the header.
func TestLatestDoesNotFollowTheRedirect(t *testing.T) {
	var hits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tag/v0.3.0", http.StatusFound)
	})
	mux.HandleFunc("/tag/v0.3.0", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, err := Latest(context.Background(), srv.Client(), srv.URL+"/latest")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "0.3.0" {
		t.Fatalf("got %q, want 0.3.0", got)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("followed the redirect %d times", n)
	}
}

func TestLatestFailsWhenNothingRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if _, err := Latest(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("want an error when there is no redirect to read")
	}
}

// A client configured to follow redirects is the normal one to have lying
// around, and passing it must not change the answer.
func TestLatestOverridesAFollowingClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/releases/tag/v0.4.0", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	got, err := Latest(context.Background(), &http.Client{}, srv.URL)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "0.4.0" {
		t.Fatalf("got %q, want 0.4.0", got)
	}
}
