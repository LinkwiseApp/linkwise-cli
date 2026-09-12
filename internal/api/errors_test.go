package api

import (
	"errors"
	"testing"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{errors.New("dial tcp: connection refused"), 1},
		{&Error{Code: "unauthorized"}, 3},
		{&Error{Code: "forbidden"}, 4},
		{&Error{Code: "not_found"}, 5},
		{&Error{Code: "rate_limited"}, 6},
		{&Error{Code: "plan_limit"}, 7},
		{&Error{Code: "pro_required"}, 7},
		// The request was sent and the server rejected it. That is not the
		// same as the caller mistyping a flag, so it is not a 2.
		{&Error{Code: "invalid_request"}, 1},
		{&Error{Code: "conflict"}, 1},
		{&Error{Code: "internal"}, 1},
	}
	for _, c := range cases {
		if got := ExitCode(c.err); got != c.want {
			t.Errorf("ExitCode(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

func TestErrorMessageCarriesRequestIdOnlyForInternal(t *testing.T) {
	internal := &Error{Code: "internal", Message: "Something went wrong.", RequestID: "req-9"}
	if internal.Error() != "Something went wrong. (request req-9)" {
		t.Errorf("internal message = %q", internal.Error())
	}
	notFound := &Error{Code: "not_found", Message: "Link not found", RequestID: "req-9"}
	if notFound.Error() != "Link not found" {
		t.Errorf("not_found message = %q", notFound.Error())
	}
}
