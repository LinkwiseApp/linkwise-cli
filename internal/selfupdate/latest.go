package selfupdate

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LatestURL is the release page that always redirects to the newest tag.
//
// Not api.github.com: that endpoint is rate limited to 60 requests an hour
// per IP unauthenticated, so on an office or CI network it starts answering
// 403 and a version check that fails for everyone behind one NAT is worse
// than no version check. The redirect is served by the website, unmetered,
// and the tag is in the Location header.
const LatestURL = "https://github.com/LinkwiseApp/linkwise-cli/releases/latest"

// checkTimeout bounds the lookup. This runs alongside a command the user
// actually asked for, and a slow network must not hold that command open.
const checkTimeout = 3 * time.Second

// Latest returns the newest published version, without its leading v.
//
// url is the release page to ask; empty means LatestURL, and a test passes
// its own server.
func Latest(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	if rawURL == "" {
		rawURL = LatestURL
	}
	if client == nil {
		client = &http.Client{}
	}

	// The caller's client is copied rather than mutated: it may be the one
	// the rest of the process uses, and redirects are wanted everywhere else.
	noFollow := *client
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if noFollow.Timeout == 0 || noFollow.Timeout > checkTimeout {
		noFollow.Timeout = checkTimeout
	}

	// HEAD, because the body of a release page is several hundred kilobytes
	// of HTML and the header is the entire answer.
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return "", err
	}

	res, err := noFollow.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	loc := res.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("no release redirect from %s (HTTP %d)", rawURL, res.StatusCode)
	}

	// Parsed rather than split, because the Location may be relative.
	u, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	tag := u.Path[strings.LastIndex(u.Path, "/")+1:]

	version := strings.TrimPrefix(tag, "v")
	if _, ok := parseVersion(version); !ok {
		return "", fmt.Errorf("unreadable release tag %q", tag)
	}
	return version, nil
}

// Newer reports whether latest is a higher version than current.
//
// Anything it cannot read on either side is false: a dev build, an empty
// string, a tag shaped like nothing we publish. Staying quiet when unsure is
// the right failure for something that only ever prints a suggestion.
func Newer(current, latest string) bool {
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}

	for i := range l {
		switch {
		case l[i] > c[i]:
			return true
		case l[i] < c[i]:
			return false
		}
	}
	return false
}

// parseVersion reads major, minor and patch out of a tag.
//
// A missing component is zero, so "0.1" and "0.1.0" are the same version
// rather than one being mysteriously smaller. Any -suffix is dropped: we do
// not publish prereleases, and the useful behaviour if one ever appears is
// to compare the release part rather than refuse.
func parseVersion(s string) ([3]int, bool) {
	var v [3]int

	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "v"))
	if s == "" {
		return v, false
	}
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}

	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}
