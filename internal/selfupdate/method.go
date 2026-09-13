// Package selfupdate works out how this binary was installed, and what the
// newest published release is.
//
// It deliberately does not replace the binary in place. Three install
// channels each own the file they put on disk, and a CLI that overwrote a
// Homebrew symlink would leave brew convinced it had installed something it
// no longer has. Naming the right command is the honest version of updating.
package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
)

// Method is the channel a binary came from.
type Method int

const (
	// Unknown is a build nobody published: `go build`, or a binary moved
	// somewhere that says nothing about where it came from.
	Unknown Method = iota
	Homebrew
	NPM
	Shell
)

func (m Method) String() string {
	switch m {
	case Homebrew:
		return "Homebrew"
	case NPM:
		return "npm"
	case Shell:
		return "the shell installer"
	default:
		return "an unknown source"
	}
}

// Slug is the name a script matches on. Separate from String because the
// prose one reads as part of a sentence and would be a poor key.
func (m Method) Slug() string {
	switch m {
	case Homebrew:
		return "homebrew"
	case NPM:
		return "npm"
	case Shell:
		return "shell"
	default:
		return "unknown"
	}
}

// Command is what the user should run to get the newest release.
func (m Method) Command() string {
	switch m {
	case Homebrew:
		return "brew upgrade --cask linkwiseapp/tap/linkwise"
	case NPM:
		return "npm install -g @linkwise/cli@latest"
	default:
		return "curl -fsSL https://linkwise.app/install.sh | sh"
	}
}

// Detect classifies an already-resolved executable path.
//
// It takes a path rather than looking one up so that every channel can be
// covered by a test on a machine that has only one of them installed.
func Detect(path string) Method {
	// Separators are normalised so that the same substrings answer on
	// Windows, where npm is the only channel we ship.
	p := filepath.ToSlash(path)

	switch {
	case strings.Contains(p, "/Caskroom/"), strings.Contains(p, "/Cellar/"):
		return Homebrew
	case strings.Contains(p, "/node_modules/"):
		return NPM
	default:
		return Shell
	}
}

// Current classifies the running binary.
//
// The symlink is followed because that is the whole signal: Homebrew puts a
// link at /usr/local/bin/linkwise pointing into the Caskroom, and the link
// itself sits exactly where the shell installer writes a real file.
func Current() Method {
	exe, err := os.Executable()
	if err != nil {
		return Unknown
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return Detect(exe)
}
