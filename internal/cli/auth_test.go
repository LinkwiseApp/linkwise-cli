package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthStatusShowsAccountPlanAndScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"uid":          "u1",
				"email":        "you@example.com",
				"subscription": map[string]any{"plan": "pro", "is_trial": false},
				"auth":         map[string]any{"via": "pat", "scopes": []string{"links:read", "search:read"}},
			},
		})
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"auth", "status", "--api", srv.URL, "--token", "lw_pat_abcdefgh", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output was not json: %q", out.String())
	}
	if got["account"] != "you@example.com" || got["plan"] != "pro" {
		t.Fatalf("status = %+v", got)
	}
	if got["token_from"] != "flag" {
		t.Fatalf("token_from = %v, it should say where the token came from", got["token_from"])
	}
}

// The published page says logout revokes. It cannot: DELETE /v1/tokens/{id}
// needs a session JWT, and requireJwt exists so that a leaked token cannot
// manage tokens. Logout clears the local copy and says where to revoke.
func TestAuthLogoutDoesNotCallTheApi(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"auth", "logout", "--api", srv.URL, "--profile", "default"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("logout must not call the API: it cannot revoke, and pretending to is worse than saying so")
	}
	if !strings.Contains(errOut.String(), "developers/dashboard") {
		t.Fatalf("logout should say where to revoke, got %q", errOut.String())
	}
}

// A key that does not work must fail at the prompt, not on the next command.
func TestAuthLoginRejectsABadKeyBeforeStoringIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "unauthorized", "message": "Missing or invalid credentials"},
		})
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetIn(strings.NewReader("lw_pat_wrong\n"))
	root.SetArgs([]string{"auth", "login", "--api", srv.URL, "--token", "-"})

	err := root.Execute()
	if err == nil {
		t.Fatal("want an error for a rejected key")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("exit code = %d, want 3", got)
	}
}
