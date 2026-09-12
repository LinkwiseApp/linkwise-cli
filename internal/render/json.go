package render

import (
	"encoding/json"
	"io"
)

// WriteNDJSON writes one object per line, which is what jq, and every other
// line oriented tool, expects to receive from a pipe.
func WriteNDJSON[T any](w io.Writer, items []T) error {
	enc := json.NewEncoder(w)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

// WriteJSON writes a single object, for the commands that return one thing
// rather than a list.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
