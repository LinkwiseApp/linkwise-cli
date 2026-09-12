package keyring

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A machine with no usable keychain must still work, and must say so rather
// than writing a credential to disk in silence.
func TestSetFallsBackToAFileWhenTheKeychainFails(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.SetFn = func(string, string, string) error { return errors.New("no keychain here") }
	// Stubbed rather than left as the real lookup: this machine's keychain may
	// hold a token for the same profile from an actual login, and a test that
	// reads it would pass or fail on what the developer did yesterday.
	s.GetFn = func(string, string) (string, error) { return "", ErrNotFound }

	from, err := s.Set("default", "lw_pat_secret")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if from != "file" {
		t.Fatalf("from = %q, want file", from)
	}

	path := filepath.Join(dir, "credentials.toml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("permissions = %o, want 600", perm)
	}

	got, gotFrom, err := s.Get("default")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "lw_pat_secret" || gotFrom != "file" {
		t.Fatalf("Get = %q from %q", got, gotFrom)
	}
}

func TestSetPrefersTheKeychain(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	stored := map[string]string{}
	s.SetFn = func(service, user, password string) error { stored[user] = password; return nil }
	s.GetFn = func(service, user string) (string, error) {
		v, ok := stored[user]
		if !ok {
			return "", ErrNotFound
		}
		return v, nil
	}

	from, err := s.Set("default", "lw_pat_secret")
	if err != nil || from != "keychain" {
		t.Fatalf("Set = %q, %v", from, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "credentials.toml")); !os.IsNotExist(err) {
		t.Fatal("a keychain success must not also write the file")
	}

	got, gotFrom, err := s.Get("default")
	if err != nil || got != "lw_pat_secret" || gotFrom != "keychain" {
		t.Fatalf("Get = %q from %q, %v", got, gotFrom, err)
	}
}

// Nothing stored is a first run, not a failure.
func TestGetWithNothingStored(t *testing.T) {
	s := New(t.TempDir())
	s.GetFn = func(string, string) (string, error) { return "", ErrNotFound }

	token, from, err := s.Get("default")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if token != "" || from != "" {
		t.Fatalf("Get = %q from %q, want empty", token, from)
	}
}

// Logging out has to clear both, or a keychain entry left behind would keep
// signing the user in after the file was removed.
func TestDeleteClearsBoth(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.SetFn = func(string, string, string) error { return errors.New("no keychain") }
	// Both stubbed for the same reason as above, and this one matters more:
	// the real DeleteFn would erase whatever this developer's keychain holds
	// for the profile, so the suite would log them out.
	s.GetFn = func(string, string) (string, error) { return "", ErrNotFound }
	s.DeleteFn = func(string, string) error { return ErrNotFound }

	if _, err := s.Set("default", "lw_pat_secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete("default"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	token, _, err := s.Get("default")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if token != "" {
		t.Fatalf("token survived delete: %q", token)
	}
}
