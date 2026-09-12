package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// Error is the gateway's error object, plus what the transport told us.
//
// Code, not message, decides everything. The gateway derives HTTP status from
// the code and refuses to let message text influence it, after an earlier
// generation of functions got that wrong by substring matching on the message.
// The CLI holds the same line: prose changes, codes do not.
type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`

	// Read off the response rather than the body.
	Status     int           `json:"-"`
	RequestID  string        `json:"-"`
	RetryAfter time.Duration `json:"-"`

	// Whether RetryAfter came from a header at all. A gateway that says
	// "retry after 0 seconds" and a gateway that says nothing both leave
	// RetryAfter at zero, and only the first one is asking to be called
	// back immediately.
	retryHinted bool
}

func (e *Error) Error() string {
	// A request id is worth showing only for a failure the reader cannot act
	// on, where the next step is to report it. On a 404 it is noise.
	if e.Code == "internal" && e.RequestID != "" {
		return e.Message + " (request " + e.RequestID + ")"
	}
	return e.Message
}

// ExitCode maps a failure onto the codes published at
// linkwise.app/developers/cli#exit-codes. They are distinct per failure class
// so that a script can tell an expired key from a missing link without
// parsing stderr.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		// Every network failure lands here: the request got no answer, so
		// there is no code to map.
		return 1
	}

	switch apiErr.Code {
	case "unauthorized":
		return 3
	case "forbidden":
		return 4
	case "not_found":
		return 5
	case "rate_limited":
		return 6
	case "plan_limit", "pro_required":
		return 7
	default:
		return 1
	}
}

// retryAfter reads how long the gateway wants us to wait. It sets Retry-After
// in seconds when it throttles, and x-ratelimit-reset on every response as a
// unix second rather than a duration.
func retryAfter(h http.Header) (time.Duration, bool) {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second, true
		}
	}
	if v := h.Get("X-RateLimit-Reset"); v != "" {
		if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
			if d := time.Until(time.Unix(unix, 0)); d > 0 {
				return d, true
			}
		}
	}
	return 0, false
}
