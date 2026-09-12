// Package browser opens a URL in whatever the desktop uses, and copies text
// to the system clipboard.
//
// Both are best effort by design. A machine reached over ssh has neither, and
// neither is ever the point of the command that called it, so a failure here
// is reported to the caller and never raised as the command's own error.
package browser

import (
	"os/exec"
	"runtime"
	"strings"
)

// Open launches the platform's URL handler. The caller is expected to have
// printed or displayed the URL already, so that a machine with no browser
// loses nothing by this failing.
func Open(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "explorer"
	default:
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, url).Start()
}

// Copy puts text on the system clipboard, returning whether it worked.
//
// Shelling out rather than taking a clipboard dependency: the three commands
// below are present wherever a clipboard exists, and a cgo-linked clipboard
// library would cost the CLI its pure-Go cross compilation, which is what
// makes one tag produce every platform's binary.
func Copy(text string) bool {
	candidates := [][]string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates, []string{"pbcopy"})
	case "windows":
		candidates = append(candidates, []string{"clip"})
	default:
		// Wayland first: a Wayland session often still has xclip installed
		// against an X server that is not the one in front of the user.
		candidates = append(candidates,
			[]string{"wl-copy"},
			[]string{"xclip", "-selection", "clipboard"},
			[]string{"xsel", "--clipboard", "--input"},
		)
	}

	for _, c := range candidates {
		path, err := exec.LookPath(c[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, c[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return false
}
