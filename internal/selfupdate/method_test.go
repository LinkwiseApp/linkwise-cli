package selfupdate

import "testing"

func TestDetectClassifiesAnInstallPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want Method
	}{
		{
			// What /usr/local/bin/linkwise resolves to once the cask's
			// symlink is followed.
			name: "homebrew cask",
			path: "/usr/local/Caskroom/linkwise/0.1.1/linkwise",
			want: Homebrew,
		},
		{
			name: "homebrew cask on apple silicon",
			path: "/opt/homebrew/Caskroom/linkwise/0.1.1/linkwise",
			want: Homebrew,
		},
		{
			// A formula is not what we ship today, but the tap could grow one
			// and the answer is the same command either way.
			name: "homebrew cellar",
			path: "/usr/local/Cellar/linkwise/0.1.1/bin/linkwise",
			want: Homebrew,
		},
		{
			// The npm wrapper spawns the real binary out of the package, so
			// this is what the Go process sees as its own path.
			name: "npm global",
			path: "/usr/local/lib/node_modules/@linkwise/cli/bin/linkwise",
			want: NPM,
		},
		{
			name: "npm under a user prefix",
			path: "/Users/x/.nvm/versions/node/v22.3.0/lib/node_modules/@linkwise/cli/bin/linkwise",
			want: NPM,
		},
		{
			name: "shell installer, system wide",
			path: "/usr/local/bin/linkwise",
			want: Shell,
		},
		{
			name: "shell installer, user local",
			path: "/Users/x/.local/bin/linkwise",
			want: Shell,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Detect(c.path); got != c.want {
				t.Fatalf("Detect(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

// The whole point of the command is to print something the user can run, so
// every method has to have one.
func TestEveryMethodNamesACommand(t *testing.T) {
	for _, m := range []Method{Homebrew, NPM, Shell} {
		if m.Command() == "" {
			t.Fatalf("method %v has no command", m)
		}
	}
}

func TestHomebrewCommandNamesTheTap(t *testing.T) {
	if got, want := Homebrew.Command(), "brew upgrade --cask linkwiseapp/tap/linkwise"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNPMCommandPinsLatest(t *testing.T) {
	if got, want := NPM.Command(), "npm install -g @linkwise/cli@latest"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestShellCommandIsTheInstaller(t *testing.T) {
	if got, want := Shell.Command(), "curl -fsSL https://linkwise.app/install.sh | sh"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The JSON output names the channel, and a name a script matches on cannot
// be the prose one that reads well in a sentence.
func TestSlugIsStableAndMachineReadable(t *testing.T) {
	cases := map[Method]string{
		Homebrew: "homebrew",
		NPM:      "npm",
		Shell:    "shell",
		Unknown:  "unknown",
	}
	for m, want := range cases {
		if got := m.Slug(); got != want {
			t.Errorf("%v.Slug() = %q, want %q", m, got, want)
		}
	}
}
