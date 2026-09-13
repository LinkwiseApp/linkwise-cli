package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LinkwiseApp/linkwise-cli/internal/selfupdate"
)

// releasesServer stands in for the GitHub release page, and points the
// update command at itself for the duration of one test.
func releasesServer(t *testing.T, version string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v"+version, http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	prev := releasesURL
	releasesURL = srv.URL
	t.Cleanup(func() { releasesURL = prev })
}

// installedVia fixes the detected channel, since the test binary lives in a
// temp directory that resembles none of them.
func installedVia(t *testing.T, m selfupdate.Method) {
	t.Helper()
	prev := detectMethod
	detectMethod = func() selfupdate.Method { return m }
	t.Cleanup(func() { detectMethod = prev })
}

type ranCommand struct {
	line   string
	stdout io.Writer
}

// captureRunner replaces the shell-out with a recorder, so --run is covered
// without a test upgrading the developer's actual install.
func captureRunner(t *testing.T) *[]ranCommand {
	t.Helper()
	var ran []ranCommand
	prev := runCommand
	runCommand = func(line string, stdout io.Writer) error {
		ran = append(ran, ranCommand{line: line, stdout: stdout})
		return nil
	}
	t.Cleanup(func() { runCommand = prev })
	return &ran
}

// Output is JSON here whatever the flag says: a bytes.Buffer is not a
// terminal, and the CLI treats a pipe as a request for JSON. That is the
// shape every one of these assertions has to speak.
func runUpdate(t *testing.T, args ...string) (updateReport, string, error) {
	t.Helper()
	root := NewRoot("0.1.0")
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"update"}, args...))

	err := root.Execute()
	if err != nil {
		return updateReport{}, errOut.String(), err
	}

	var report updateReport
	if decodeErr := json.Unmarshal(out.Bytes(), &report); decodeErr != nil {
		t.Fatalf("output is not the documented JSON: %v\n%s", decodeErr, out.String())
	}
	return report, errOut.String(), nil
}

func TestUpdateReportsTheCommandForAHomebrewInstall(t *testing.T) {
	releasesServer(t, "0.2.0")
	installedVia(t, selfupdate.Homebrew)
	captureRunner(t)

	report, _, err := runUpdate(t)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if report.Current != "0.1.0" || report.Latest != "0.2.0" {
		t.Fatalf("versions are %q -> %q", report.Current, report.Latest)
	}
	if !report.UpdateAvailable {
		t.Fatal("update_available is false with a newer release out")
	}
	if report.InstalledVia != "homebrew" {
		t.Fatalf("installed_via = %q, want homebrew", report.InstalledVia)
	}
	if report.Command != selfupdate.Homebrew.Command() {
		t.Fatalf("command = %q", report.Command)
	}
}

func TestUpdateReportsTheNpmCommandForAnNpmInstall(t *testing.T) {
	releasesServer(t, "0.2.0")
	installedVia(t, selfupdate.NPM)
	captureRunner(t)

	report, _, err := runUpdate(t)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if report.InstalledVia != "npm" || report.Command != selfupdate.NPM.Command() {
		t.Fatalf("got %q / %q", report.InstalledVia, report.Command)
	}
}

func TestUpdateReportsNoUpdateOnTheNewest(t *testing.T) {
	releasesServer(t, "0.1.0")
	installedVia(t, selfupdate.Shell)
	captureRunner(t)

	report, _, err := runUpdate(t)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if report.UpdateAvailable {
		t.Fatal("update_available is true on the newest release")
	}
}

// Overwriting the running binary is the user's call, not something a command
// called `update` gets to assume.
func TestUpdateDoesNotRunAnythingByDefault(t *testing.T) {
	releasesServer(t, "0.2.0")
	installedVia(t, selfupdate.Homebrew)
	ran := captureRunner(t)

	if _, _, err := runUpdate(t); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(*ran) != 0 {
		t.Fatalf("ran %v without being asked", *ran)
	}
}

// --run is an instruction, not a rendering preference, so a piped run has to
// honour it too.
func TestUpdateRunExecutesTheCommand(t *testing.T) {
	releasesServer(t, "0.2.0")
	installedVia(t, selfupdate.Homebrew)
	ran := captureRunner(t)

	if _, _, err := runUpdate(t, "--run"); err != nil {
		t.Fatalf("update --run: %v", err)
	}
	if len(*ran) != 1 {
		t.Fatalf("ran %d commands, want 1", len(*ran))
	}
	if (*ran)[0].line != selfupdate.Homebrew.Command() {
		t.Fatalf("ran %q", (*ran)[0].line)
	}
}

// stdout is a data channel when the output is JSON, so brew's progress bars
// cannot be allowed onto it or the document a script is parsing is ruined.
func TestUpdateRunKeepsJSONOutputClean(t *testing.T) {
	releasesServer(t, "0.2.0")
	installedVia(t, selfupdate.Homebrew)
	ran := captureRunner(t)

	report, _, err := runUpdate(t, "--run")
	if err != nil {
		t.Fatalf("update --run: %v", err)
	}
	if !report.UpdateAvailable {
		t.Fatal("update_available is false with a newer release out")
	}
	if len(*ran) != 1 {
		t.Fatalf("ran %d commands, want 1", len(*ran))
	}
	if w := (*ran)[0].stdout; w == io.Writer(nil) {
		t.Fatal("no stream was given to the command")
	}
}

func TestUpdateRunDoesNothingWhenAlreadyCurrent(t *testing.T) {
	releasesServer(t, "0.1.0")
	installedVia(t, selfupdate.Homebrew)
	ran := captureRunner(t)

	if _, _, err := runUpdate(t, "--run"); err != nil {
		t.Fatalf("update --run: %v", err)
	}
	if len(*ran) != 0 {
		t.Fatalf("ran %v on an install that was already newest", *ran)
	}
}

func TestUpdateFailsWhenTheLookupFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	prev := releasesURL
	releasesURL = srv.URL
	t.Cleanup(func() { releasesURL = prev })

	installedVia(t, selfupdate.Shell)
	captureRunner(t)

	if _, _, err := runUpdate(t); err == nil {
		t.Fatal("want an error when the release lookup fails")
	}
}

// An argument to `update` is a typo, and the published table says a usage
// error exits 2.
func TestUpdateRejectsArguments(t *testing.T) {
	installedVia(t, selfupdate.Shell)
	captureRunner(t)

	_, _, err := runUpdate(t, "now")
	if err == nil {
		t.Fatal("want an error for an unexpected argument")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code %d, want 2", got)
	}
}

// The command has to be discoverable from the help text, which is the whole
// reason it exists.
func TestUpdateAppearsInHelp(t *testing.T) {
	root := NewRoot("0.1.0")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(out.String(), "update") {
		t.Fatalf("help does not mention update:\n%s", out.String())
	}
}

// The prose output never reaches a test through a pipe, so it is asserted
// where it is actually decided.
func TestReportLinesNameBothVersionsAndTheCommand(t *testing.T) {
	report := updateReport{
		Current:         "0.1.0",
		Latest:          "0.2.0",
		UpdateAvailable: true,
		InstalledVia:    "homebrew",
		Command:         selfupdate.Homebrew.Command(),
	}

	got := strings.Join(report.lines(selfupdate.Homebrew, false), "\n")
	for _, want := range []string{"0.1.0", "0.2.0", selfupdate.Homebrew.Command(), "Homebrew"} {
		if !strings.Contains(got, want) {
			t.Errorf("lines are missing %q:\n%s", want, got)
		}
	}
}

func TestReportLinesSayNothingToRunWhenCurrent(t *testing.T) {
	report := updateReport{
		Current:         "0.1.1",
		Latest:          "0.1.1",
		UpdateAvailable: false,
		InstalledVia:    "shell",
		Command:         selfupdate.Shell.Command(),
	}

	got := strings.Join(report.lines(selfupdate.Shell, false), "\n")
	if !strings.Contains(got, "0.1.1") {
		t.Errorf("lines do not name the version:\n%s", got)
	}
	if strings.Contains(got, selfupdate.Shell.Command()) {
		t.Errorf("told an up to date install to update:\n%s", got)
	}
}

// With --run the command is about to be executed, so printing it as a thing
// to go and type would be wrong.
func TestReportLinesAnnounceARunInsteadOfSuggestingIt(t *testing.T) {
	report := updateReport{
		Current:         "0.1.0",
		Latest:          "0.2.0",
		UpdateAvailable: true,
		InstalledVia:    "homebrew",
		Command:         selfupdate.Homebrew.Command(),
	}

	got := strings.Join(report.lines(selfupdate.Homebrew, true), "\n")
	if strings.Contains(got, "--run") {
		t.Errorf("suggested --run to somebody who passed it:\n%s", got)
	}
	if !strings.Contains(got, selfupdate.Homebrew.Command()) {
		t.Errorf("did not say what it is about to run:\n%s", got)
	}
}
