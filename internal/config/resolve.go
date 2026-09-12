package config

import "fmt"

// Sources is everything that can supply a setting, gathered so the precedence
// rules read as one function instead of four scattered fallbacks.
type Sources struct {
	FlagToken   string
	FlagProfile string
	FlagAPI     string

	// Getenv is injected so tests never mutate the process environment, which
	// would make them order dependent.
	Getenv func(string) string

	// Keychain returns the stored token for a profile and where it came from,
	// "keychain" or "file". An empty token with a nil error means nothing is
	// stored, which is not a failure: it is a first run.
	Keychain func(profile string) (token, from string, err error)

	File *Config
}

type Resolved struct {
	Profile string
	BaseURL string
	Token   string

	// TokenFrom names the winning source, so `auth status` can say
	// "environment" and someone can see why CI is not using their key.
	TokenFrom string
}

func Resolve(s Sources) (Resolved, error) {
	getenv := s.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	file := s.File
	if file == nil {
		file = defaults()
	}

	var out Resolved

	// Profile, because the keychain lookup and the base URL both need it.
	switch {
	case s.FlagProfile != "":
		out.Profile = s.FlagProfile
	case getenv("LINKWISE_PROFILE") != "":
		out.Profile = getenv("LINKWISE_PROFILE")
	case file.DefaultProfile != "":
		out.Profile = file.DefaultProfile
	default:
		out.Profile = "default"
	}

	// A named profile with no entry is a typo worth reporting. "default" is
	// exempt: it is what you get when there is no file at all.
	if _, ok := file.Profiles[out.Profile]; !ok && out.Profile != "default" {
		return out, fmt.Errorf("no profile named %q in the config file", out.Profile)
	}

	switch {
	case s.FlagAPI != "":
		out.BaseURL = s.FlagAPI
	case getenv("LINKWISE_API") != "":
		out.BaseURL = getenv("LINKWISE_API")
	case file.Profiles[out.Profile].API != "":
		out.BaseURL = file.Profiles[out.Profile].API
	default:
		out.BaseURL = DefaultAPI
	}

	switch {
	case s.FlagToken != "":
		out.Token, out.TokenFrom = s.FlagToken, "flag"
	case getenv("LINKWISE_TOKEN") != "":
		// Above the keychain on purpose. A pipeline must never pick up a
		// developer's stored credential because the runner happened to have
		// one.
		out.Token, out.TokenFrom = getenv("LINKWISE_TOKEN"), "environment"
	case s.Keychain != nil:
		token, from, err := s.Keychain(out.Profile)
		if err != nil {
			return out, err
		}
		out.Token, out.TokenFrom = token, from
	}

	return out, nil
}
