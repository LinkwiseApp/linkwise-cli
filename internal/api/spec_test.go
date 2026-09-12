package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Every route the CLI can call must exist in the spec.
//
// This is what replaces generating a client. An endpoint renamed or removed in
// the API fails here, in CI, instead of surfacing as a 404 in somebody's
// terminal a release later.
func TestEveryCalledRouteExistsInTheSpec(t *testing.T) {
	raw, err := os.ReadFile("../../spec/openapi.json")
	if err != nil {
		t.Fatalf("read spec: %v. Run scripts/sync-openapi.sh", err)
	}

	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	for _, route := range CalledRoutes {
		methods, ok := doc.Paths[route.Path]
		if !ok {
			t.Errorf("%s %s: the spec has no such path", route.Method, route.Path)
			continue
		}
		if _, ok := methods[strings.ToLower(route.Method)]; !ok {
			t.Errorf("%s %s: the path exists but not that method", route.Method, route.Path)
		}
	}
}

// The two the CLI must never call. routes/ai.ts refuses a token on both, and a
// command that reached them would fail for every user who ever ran it.
func TestTheCliNeverCallsTheSessionOnlyRoutes(t *testing.T) {
	for _, route := range CalledRoutes {
		if route.Path == "/v1/chat" || route.Path == "/v1/tts" {
			t.Errorf("%s %s is session only and cannot be reached with a token", route.Method, route.Path)
		}
	}
}
