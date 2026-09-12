package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The published example is `linkwise feeds export > linkwise.opml`, so what
// lands on stdout has to be the OPML itself and nothing else. An "Exported 12
// feeds" line would corrupt the file.
func TestFeedsExportWritesOnlyOpmlToStdout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-opml+xml; charset=utf-8")
		w.Write([]byte("<?xml version=\"1.0\"?><opml version=\"2.0\"></opml>\n"))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	root := NewRoot("test")
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"feeds", "export", "--api", srv.URL, "--token", "lw_pat_x"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.HasPrefix(out.String(), "<?xml") {
		t.Fatalf("stdout = %q", out.String())
	}
	if strings.Contains(out.String(), "Exported") {
		t.Fatal("stdout must carry the document alone")
	}
}
