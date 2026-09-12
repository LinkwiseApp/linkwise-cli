package api

import (
	"context"
	"errors"
	"net/url"
)

// ErrStop ends a walk early without making it look like a failure. `ls
// --limit 5` needs it: the server pages in twenty-fives, so the fifth record
// arrives mid-page and there is nothing left to ask for.
var ErrStop = errors.New("stop paginating")

// Each walks a list endpoint page by page, calling fn once per page.
//
// Every list endpoint in the API pages identically: an opaque cursor in,
// has_more and next_cursor out. Keeping that in one place is what lets the
// cursor's contents change without touching a single command.
func Each[T any](ctx context.Context, c *Client, r Request, fn func([]T) error) error {
	// Copied so a caller can reuse the Request it passed in, and so setting a
	// cursor here never reaches back into their url.Values.
	query := url.Values{}
	for k, v := range r.Query {
		query[k] = append([]string(nil), v...)
	}
	r.Query = query

	for {
		var page []T
		meta, err := c.Do(ctx, r, &page)
		if err != nil {
			return err
		}

		if err := fn(page); err != nil {
			if errors.Is(err, ErrStop) {
				return nil
			}
			return err
		}

		if meta == nil || meta.NextCursor == nil || *meta.NextCursor == "" {
			return nil
		}
		r.Query.Set("cursor", *meta.NextCursor)
	}
}
