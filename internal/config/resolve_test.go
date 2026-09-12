package config

import "testing"

func env(pairs map[string]string) func(string) string {
	return func(k string) string { return pairs[k] }
}

func noKeychain(string) (string, string, error) { return "", "", nil }

func TestResolveTokenPrecedence(t *testing.T) {
	keychain := func(string) (string, string, error) { return "from-keychain", "keychain", nil }

	cases := []struct {
		name     string
		sources  Sources
		want     string
		wantFrom string
	}{
		{
			name: "flag beats everything",
			sources: Sources{
				FlagToken: "from-flag",
				Getenv:    env(map[string]string{"LINKWISE_TOKEN": "from-env"}),
				Keychain:  keychain,
			},
			want: "from-flag", wantFrom: "flag",
		},
		{
			// A pipeline must never pick up a developer's stored credential.
			name: "environment beats the keychain",
			sources: Sources{
				Getenv:   env(map[string]string{"LINKWISE_TOKEN": "from-env"}),
				Keychain: keychain,
			},
			want: "from-env", wantFrom: "environment",
		},
		{
			name:    "keychain when nothing else is set",
			sources: Sources{Getenv: env(nil), Keychain: keychain},
			want:    "from-keychain", wantFrom: "keychain",
		},
		{
			name:    "nothing at all is not an error",
			sources: Sources{Getenv: env(nil), Keychain: noKeychain},
			want:    "", wantFrom: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Resolve(c.sources)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.Token != c.want || got.TokenFrom != c.wantFrom {
				t.Fatalf("token = %q from %q, want %q from %q",
					got.Token, got.TokenFrom, c.want, c.wantFrom)
			}
		})
	}
}

func TestResolveProfileAndBaseURL(t *testing.T) {
	file := &Config{
		DefaultProfile: "work",
		Profiles: map[string]Profile{
			"work": {API: "https://staging.example/v1"},
		},
	}

	got, err := Resolve(Sources{Getenv: env(nil), Keychain: noKeychain, File: file})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Profile != "work" || got.BaseURL != "https://staging.example/v1" {
		t.Fatalf("resolved = %+v", got)
	}

	// LINKWISE_API points at a staging deploy without editing the file.
	got, err = Resolve(Sources{
		Getenv:   env(map[string]string{"LINKWISE_API": "https://other.example/v1"}),
		Keychain: noKeychain,
		File:     file,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.BaseURL != "https://other.example/v1" {
		t.Fatalf("base url = %q", got.BaseURL)
	}
}

func TestResolveFallsBackToDefaults(t *testing.T) {
	got, err := Resolve(Sources{Getenv: env(nil), Keychain: noKeychain})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Profile != "default" {
		t.Fatalf("profile = %q", got.Profile)
	}
	if got.BaseURL != DefaultAPI {
		t.Fatalf("base url = %q", got.BaseURL)
	}
}

// A profile named on the command line that does not exist is a mistake worth
// reporting, not something to silently treat as the default.
func TestResolveRejectsUnknownProfile(t *testing.T) {
	file := &Config{Profiles: map[string]Profile{"work": {}}}
	_, err := Resolve(Sources{FlagProfile: "hom", Getenv: env(nil), Keychain: noKeychain, File: file})
	if err == nil {
		t.Fatal("want an error for an unknown profile")
	}
}
