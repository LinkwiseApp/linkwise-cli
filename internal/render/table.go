package render

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// WriteTable prints aligned columns. text/tabwriter is in the standard
// library and does the whole job, so there is no table dependency to keep
// current.
func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	if len(headers) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
			return err
		}
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// Truncate shortens to max characters, counted in runes. Bytes would misalign
// any column holding a title with an accent in it.
func Truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}
