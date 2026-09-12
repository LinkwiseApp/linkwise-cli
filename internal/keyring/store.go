// Package keyring stores one token per profile.
//
// The OS keychain when there is one, a file at 0600 when there is not. Which
// was used is returned rather than hidden: a tool that quietly writes a
// credential to disk is a tool that surprises someone during a screen share.
package keyring

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	gokeyring "github.com/zalando/go-keyring"
)

// Service is the keychain entry name. Reverse DNS because that is what every
// macOS keychain viewer sorts by, and a user should be able to find this.
//
// A variable rather than a constant so a test suite can point itself at an
// entry nobody is signed in with. Without that seam, running the tests on a
// machine where someone had logged in would read, and on logout erase, their
// real key.
var Service = "app.linkwise.cli"

var ErrNotFound = gokeyring.ErrNotFound

type Store struct {
	Dir string

	// Injected so the fallback path is testable on a machine whose keychain
	// works perfectly well.
	SetFn    func(service, user, password string) error
	GetFn    func(service, user string) (string, error)
	DeleteFn func(service, user string) error
}

func New(dir string) *Store {
	return &Store{
		Dir:      dir,
		SetFn:    gokeyring.Set,
		GetFn:    gokeyring.Get,
		DeleteFn: gokeyring.Delete,
	}
}

type credentials struct {
	Profiles map[string]credential `toml:"profiles"`
}

type credential struct {
	Token string `toml:"token"`
}

func (s *Store) path() string { return filepath.Join(s.Dir, "credentials.toml") }

// Set stores a token and reports where it landed, so the caller can say so.
func (s *Store) Set(profile, token string) (string, error) {
	if err := s.SetFn(Service, profile, token); err == nil {
		return "keychain", nil
	}
	return "file", s.writeFile(profile, token)
}

func (s *Store) Get(profile string) (string, string, error) {
	token, err := s.GetFn(Service, profile)
	if err == nil && token != "" {
		return token, "keychain", nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		// The keychain exists but refused, usually because the user declined
		// the unlock prompt. Fall through to the file rather than failing: the
		// file may well hold the answer on this machine.
		_ = err
	}

	creds, err := s.readFile()
	if err != nil {
		return "", "", err
	}
	if c, ok := creds.Profiles[profile]; ok && c.Token != "" {
		return c.Token, "file", nil
	}
	return "", "", nil
}

// Delete clears both stores. Clearing only one would leave a keychain entry
// that keeps signing the user in after they logged out.
func (s *Store) Delete(profile string) (string, error) {
	var from string

	if err := s.DeleteFn(Service, profile); err == nil {
		from = "keychain"
	}

	creds, err := s.readFile()
	if err != nil {
		return from, err
	}
	if _, ok := creds.Profiles[profile]; ok {
		delete(creds.Profiles, profile)
		if err := s.writeCredentials(creds); err != nil {
			return from, err
		}
		if from == "" {
			from = "file"
		}
	}
	return from, nil
}

func (s *Store) readFile() (credentials, error) {
	creds := credentials{Profiles: map[string]credential{}}

	raw, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return creds, nil
	}
	if err != nil {
		return creds, err
	}
	if err := toml.Unmarshal(raw, &creds); err != nil {
		return creds, err
	}
	if creds.Profiles == nil {
		creds.Profiles = map[string]credential{}
	}
	return creds, nil
}

func (s *Store) writeFile(profile, token string) error {
	creds, err := s.readFile()
	if err != nil {
		return err
	}
	creds.Profiles[profile] = credential{Token: token}
	return s.writeCredentials(creds)
}

func (s *Store) writeCredentials(creds credentials) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}

	// Created 0600 from the start rather than written and then chmodded.
	// Between those two calls the token would be world readable, which is a
	// window worth not having.
	f, err := os.OpenFile(s.path(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(creds)
}
