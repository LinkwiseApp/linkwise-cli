package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func tempCache(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "version-check.json")
}

func redirectTo(t *testing.T, version string, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		http.Redirect(w, r, "/releases/tag/v"+version, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestReadCacheOfAMissingFileIsNotAnError(t *testing.T) {
	c := readCache(tempCache(t))
	if c.Latest != "" {
		t.Fatalf("want an empty cache, got %+v", c)
	}
	if !c.due(time.Now()) {
		t.Fatal("a cache that was never written is due")
	}
}

func TestReadCacheOfACorruptFileIsNotAnError(t *testing.T) {
	path := tempCache(t)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := readCache(path)
	if c.Latest != "" {
		t.Fatalf("want an empty cache, got %+v", c)
	}
	if !c.due(time.Now()) {
		t.Fatal("an unreadable cache is due")
	}
}

func TestCacheRoundTrips(t *testing.T) {
	path := tempCache(t)
	now := time.Now().Truncate(time.Second)

	if err := writeCache(path, cache{CheckedAt: now, Latest: "9.9.9"}); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	got := readCache(path)
	if got.Latest != "9.9.9" {
		t.Fatalf("Latest = %q, want 9.9.9", got.Latest)
	}
	if !got.CheckedAt.Equal(now) {
		t.Fatalf("CheckedAt = %v, want %v", got.CheckedAt, now)
	}
}

func TestCacheIsNotDueWithinTheInterval(t *testing.T) {
	now := time.Now()
	c := cache{CheckedAt: now.Add(-23 * time.Hour), Latest: "0.1.1"}
	if c.due(now) {
		t.Fatal("a check from 23 hours ago is not due again")
	}
}

func TestCacheIsDueAfterTheInterval(t *testing.T) {
	now := time.Now()
	c := cache{CheckedAt: now.Add(-25 * time.Hour), Latest: "0.1.1"}
	if !c.due(now) {
		t.Fatal("a check from 25 hours ago is due")
	}
}

// A clock that went backwards must not park the check forever.
func TestCacheWrittenInTheFutureIsDue(t *testing.T) {
	now := time.Now()
	c := cache{CheckedAt: now.Add(48 * time.Hour), Latest: "0.1.1"}
	if !c.due(now) {
		t.Fatal("a check stamped in the future is due")
	}
}

func TestBeginReportsANewerRelease(t *testing.T) {
	srv := redirectTo(t, "0.2.0", nil)

	check := Begin(Options{
		Current:   "0.1.0",
		CachePath: tempCache(t),
		Client:    srv.Client(),
		URL:       srv.URL,
		Method:    Homebrew,
		Now:       time.Now(),
	})

	notice := check.Notice(2 * time.Second)
	if !strings.Contains(notice, "0.2.0") {
		t.Fatalf("notice does not name the new version: %q", notice)
	}
	if !strings.Contains(notice, "0.1.0") {
		t.Fatalf("notice does not name the running version: %q", notice)
	}
	if !strings.Contains(notice, Homebrew.Command()) {
		t.Fatalf("notice does not name the command to run: %q", notice)
	}
}

func TestBeginSaysNothingWhenCurrent(t *testing.T) {
	srv := redirectTo(t, "0.1.1", nil)

	check := Begin(Options{
		Current:   "0.1.1",
		CachePath: tempCache(t),
		Client:    srv.Client(),
		URL:       srv.URL,
		Method:    Shell,
		Now:       time.Now(),
	})

	if notice := check.Notice(2 * time.Second); notice != "" {
		t.Fatalf("want silence on the newest version, got %q", notice)
	}
}

// The check must not cost a network round trip on every command.
func TestBeginSkipsTheNetworkWhenTheCacheIsFresh(t *testing.T) {
	var hits int32
	srv := redirectTo(t, "0.2.0", &hits)

	path := tempCache(t)
	now := time.Now()
	if err := writeCache(path, cache{CheckedAt: now, Latest: "0.2.0"}); err != nil {
		t.Fatal(err)
	}

	check := Begin(Options{
		Current:   "0.1.0",
		CachePath: path,
		Client:    srv.Client(),
		URL:       srv.URL,
		Method:    NPM,
		Now:       now,
	})

	// Still reported, from what the last check learned.
	if notice := check.Notice(2 * time.Second); !strings.Contains(notice, "0.2.0") {
		t.Fatalf("want the cached version reported, got %q", notice)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("asked the network %d times with a fresh cache", n)
	}
}

func TestBeginStoresWhatItLearned(t *testing.T) {
	srv := redirectTo(t, "0.2.0", nil)
	path := tempCache(t)

	check := Begin(Options{
		Current:   "0.1.0",
		CachePath: path,
		Client:    srv.Client(),
		URL:       srv.URL,
		Method:    Shell,
		Now:       time.Now(),
	})
	check.Notice(2 * time.Second)

	if got := readCache(path); got.Latest != "0.2.0" {
		t.Fatalf("cache holds %q, want 0.2.0", got.Latest)
	}
}

// A dev build is someone working on the CLI. Telling them to go install a
// release of it is noise, and the lookup is wasted.
func TestBeginIsSilentForADevBuild(t *testing.T) {
	var hits int32
	srv := redirectTo(t, "0.2.0", &hits)

	check := Begin(Options{
		Current:   "dev",
		CachePath: tempCache(t),
		Client:    srv.Client(),
		URL:       srv.URL,
		Method:    Shell,
		Now:       time.Now(),
	})

	if notice := check.Notice(2 * time.Second); notice != "" {
		t.Fatalf("want silence on a dev build, got %q", notice)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("a dev build asked the network %d times", n)
	}
}

func TestBeginIsSilentWhenDisabled(t *testing.T) {
	var hits int32
	srv := redirectTo(t, "0.2.0", &hits)

	check := Begin(Options{
		Current:   "0.1.0",
		CachePath: tempCache(t),
		Client:    srv.Client(),
		URL:       srv.URL,
		Method:    Shell,
		Now:       time.Now(),
		Disabled:  true,
	})

	if notice := check.Notice(2 * time.Second); notice != "" {
		t.Fatalf("want silence when disabled, got %q", notice)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("a disabled check asked the network %d times", n)
	}
}

// A check still in flight must never hold a command open.
func TestNoticeGivesUpOnASlowCheck(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		http.Redirect(w, r, "/releases/tag/v0.2.0", http.StatusFound)
	}))
	defer srv.Close()
	defer close(release)

	check := Begin(Options{
		Current:   "0.1.0",
		CachePath: tempCache(t),
		Client:    srv.Client(),
		URL:       srv.URL,
		Method:    Shell,
		Now:       time.Now(),
	})

	start := time.Now()
	notice := check.Notice(50 * time.Millisecond)
	elapsed := time.Since(start)

	if notice != "" {
		t.Fatalf("want silence from an unfinished check, got %q", notice)
	}
	if elapsed > time.Second {
		t.Fatalf("Notice waited %v for a check that had not finished", elapsed)
	}
}

// Every command path calls this, including ones that never started a check.
func TestNoticeOnANilCheckIsEmpty(t *testing.T) {
	var check *Check
	if notice := check.Notice(time.Second); notice != "" {
		t.Fatalf("want empty, got %q", notice)
	}
}

// An unreachable network is the normal state of a laptop on a plane, and it
// is not this command's business to report it.
func TestBeginSwallowsALookupFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	check := Begin(Options{
		Current:   "0.1.0",
		CachePath: tempCache(t),
		Client:    srv.Client(),
		URL:       srv.URL,
		Method:    Shell,
		Now:       time.Now(),
	})

	if notice := check.Notice(2 * time.Second); notice != "" {
		t.Fatalf("want silence when the lookup fails, got %q", notice)
	}
}
