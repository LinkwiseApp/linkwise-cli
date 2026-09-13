package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// interval is how often the newest release is looked up. A release lands
// every few weeks at most, so asking once a day is already generous, and it
// keeps a busy shell from making the same request forty times an hour.
const interval = 24 * time.Hour

// cache is what the last check learned, so that a command can report a new
// release without waiting for a network round trip of its own.
type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// due reports whether it is time to ask again.
//
// A stamp in the future counts as due: a clock that moved backwards would
// otherwise park the check until the machine caught up, which on a laptop
// that was set wrong once could be months.
func (c cache) due(now time.Time) bool {
	if c.CheckedAt.IsZero() || c.CheckedAt.After(now) {
		return true
	}
	return now.Sub(c.CheckedAt) >= interval
}

// readCache never fails. A missing file is the first run and a corrupt one is
// a file we wrote and can write again; either way the answer is "check now",
// and neither is worth failing somebody's `ls` over.
func readCache(path string) cache {
	var c cache
	raw, err := os.ReadFile(path)
	if err != nil {
		return cache{}
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return cache{}
	}
	return c
}

func writeCache(path string, c cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Options is everything a check needs. Each field is injected rather than
// looked up so the whole thing runs against a test server and a temp file.
type Options struct {
	Current   string
	CachePath string
	Client    *http.Client
	URL       string
	Method    Method
	Now       time.Time

	// Disabled is the escape hatch, wired to LINKWISE_NO_UPDATE_CHECK. A
	// build pipeline that has pinned a version does not want to be told
	// about a newer one on every step.
	Disabled bool
}

// Check is a lookup running alongside the command the user actually asked
// for. Overlapping the two is what keeps the check free: by the time a
// command that talked to the API is done, this has usually finished too.
type Check struct {
	opts   Options
	cached string
	done   chan string
}

// Begin starts a check if one is due, and returns immediately either way.
func Begin(opts Options) *Check {
	c := &Check{opts: opts}

	// Nothing to compare against, so nothing to say and no reason to ask.
	if opts.Disabled {
		return c
	}
	if _, ok := parseVersion(opts.Current); !ok {
		return c
	}
	if opts.CachePath == "" {
		return c
	}
	if opts.Now.IsZero() {
		c.opts.Now = time.Now()
	}

	stored := readCache(opts.CachePath)
	c.cached = stored.Latest

	if !stored.due(c.opts.Now) {
		return c
	}

	c.done = make(chan string, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
		defer cancel()

		latest, err := Latest(ctx, c.opts.Client, c.opts.URL)
		if err != nil {
			// Silence. An offline laptop is not a broken CLI, and the stamp
			// is deliberately not written, so the next run tries again
			// rather than waiting out a day of no network.
			c.done <- ""
			return
		}

		_ = writeCache(c.opts.CachePath, cache{CheckedAt: c.opts.Now, Latest: latest})
		c.done <- latest
	}()

	return c
}

// Notice returns the single line to print on stderr, or "".
//
// It waits at most grace for a lookup still in flight. A command must never
// be held open by an errand it did not ask for, so an unfinished check just
// reports nothing and leaves the news for the next run, which reads it from
// the cache for free.
func (c *Check) Notice(grace time.Duration) string {
	if c == nil {
		return ""
	}

	latest := c.cached
	if c.done != nil {
		select {
		case found := <-c.done:
			if found != "" {
				latest = found
			}
		case <-time.After(grace):
		}
	}

	if !Newer(c.opts.Current, latest) {
		return ""
	}
	return fmt.Sprintf("A new release of linkwise is out: %s -> %s. Update with:\n  %s",
		c.opts.Current, latest, c.opts.Method.Command())
}

// CachePath is where the last check is remembered, next to the config file
// rather than in a cache directory, so that one documented path holds
// everything the CLI writes.
func CachePath(configDir string) string {
	return filepath.Join(configDir, "version-check.json")
}
