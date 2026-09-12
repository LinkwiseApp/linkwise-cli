// Package api is the only thing in the CLI that speaks HTTP.
//
// It owns the response envelope, the error shape, cursor pagination and the
// one automatic retry, so that no command has to know any of it. A command
// that hand-rolled pagination would be a command that breaks when the cursor's
// contents change, which is precisely what the cursor being opaque is for.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://jcbgrqawrvztwsvxawda.supabase.co/functions/v1/api/v1"

// maxRetryWait bounds the single automatic retry. A daily quota can reset
// hours away, and a CLI that appears to hang is worse than one that exits 6
// and says when to come back.
const maxRetryWait = 60 * time.Second

// unhintedRetryWait is what we pause for when a 429 arrives with no
// Retry-After and no reset header. Going straight back would earn the same
// rejection; a second's pause at least gives a per-second bucket time to
// refill.
const unhintedRetryWait = time.Second

type Client struct {
	BaseURL   string
	Token     string
	UserAgent string
	HTTP      *http.Client
}

func New(baseURL, token, version string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Token:   token,
		// Named so the gateway's request log can tell us which releases are
		// still in the wild, which is the only way to know when an old one
		// can stop being supported.
		UserAgent: "linkwise-cli/" + version,
		HTTP:      &http.Client{Timeout: 60 * time.Second},
	}
}

type Request struct {
	Method string
	Path   string
	Query  url.Values
	Body   any
}

type Meta struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Total      *int    `json:"total"`
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Meta  *Meta           `json:"meta"`
	Error *Error          `json:"error"`
}

// Do sends one request and decodes the envelope into out.
//
// It retries a rate limit exactly once, after waiting the interval the
// gateway names, because the published exit-code table promises that a 6 means
// the CLI already tried twice.
func (c *Client) Do(ctx context.Context, r Request, out any) (*Meta, error) {
	meta, err := c.do(ctx, r, out)

	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.Code == "rate_limited" && apiErr.RetryAfter <= maxRetryWait {
		wait := apiErr.RetryAfter
		if !apiErr.retryHinted {
			wait = unhintedRetryWait
		}
		select {
		case <-time.After(wait):
			return c.do(ctx, r, out)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return meta, err
}

func (c *Client) do(ctx context.Context, r Request, out any) (*Meta, error) {
	var body io.Reader
	if r.Body != nil {
		buf, err := json.Marshal(r.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(buf)
	}

	endpoint := c.BaseURL + r.Path
	if len(r.Query) > 0 {
		endpoint += "?" + r.Query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// DELETE answers 204 with no body at all. Reading that as malformed JSON
	// would turn every successful delete into a failure.
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		// A body that is not the envelope means something in front of the
		// gateway answered: a proxy, an outage page, a captive portal.
		// Reporting a JSON parse error would send the reader after the wrong
		// problem.
		return nil, &Error{
			Code:      "internal",
			Message:   fmt.Sprintf("unexpected %d response from %s", resp.StatusCode, endpoint),
			Status:    resp.StatusCode,
			RequestID: resp.Header.Get("x-request-id"),
		}
	}

	if env.Error != nil {
		env.Error.Status = resp.StatusCode
		env.Error.RequestID = resp.Header.Get("x-request-id")
		env.Error.RetryAfter, env.Error.retryHinted = retryAfter(resp.Header)
		return nil, env.Error
	}

	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return nil, err
		}
	}

	return env.Meta, nil
}
