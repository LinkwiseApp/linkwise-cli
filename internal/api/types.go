package api

import "encoding/json"

// Me is GET /v1/me.
type Me struct {
	UID          string        `json:"uid"`
	Name         *string       `json:"name"`
	Email        string        `json:"email"`
	Username     *string       `json:"username"`
	Subscription *Subscription `json:"subscription"`
	Auth         AuthInfo      `json:"auth"`
}

type Subscription struct {
	Plan     string  `json:"plan"`
	IsTrial  *bool   `json:"is_trial"`
	RenewsAt *string `json:"renews_at"`
	Platform *string `json:"platform"`
}

// AuthInfo describes the credential that made the call.
//
// `scopes` is a union: the array a token holds, or the literal string "all"
// for a first-party session, because a session can already do everything by
// calling PostgREST directly. Declaring it as []string decodes one and fails
// on the other, which is the kind of bug that only appears on somebody else's
// account.
type AuthInfo struct {
	Via    string          `json:"via"`
	Scopes json.RawMessage `json:"scopes"`
}

func (a AuthInfo) ScopeList() []string {
	var list []string
	if err := json.Unmarshal(a.Scopes, &list); err == nil {
		return list
	}
	var single string
	if err := json.Unmarshal(a.Scopes, &single); err == nil && single != "" {
		return []string{single}
	}
	return nil
}

// Link is one saved item. Most fields are nullable because a link exists the
// moment it is saved and is enriched afterwards: a title arrives when the page
// is fetched, not when the row is written.
type Link struct {
	ID           string  `json:"id"`
	URL          string  `json:"url"`
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	CollectionID *string `json:"collection_id"`
	IsVisited    *bool   `json:"is_visited"`
	ArchivedAt   *string `json:"archived_at"`
	CreatedAt    string  `json:"created_at"`
}

// Collection and Tag are the same idea with different column names. The RPCs
// behind them prefix their columns and the API surfaces that rather than
// renaming, so `collection_name` and `tag_name` are what actually arrive.
// Accessors keep every command from having to care which one it holds.
type Collection struct {
	CollectionID   string `json:"collection_id"`
	CollectionName string `json:"collection_name"`
	LinkCount      *int   `json:"link_count"`
}

func (c Collection) ID() string   { return c.CollectionID }
func (c Collection) Name() string { return c.CollectionName }

type Tag struct {
	TagID     string `json:"tag_id"`
	TagName   string `json:"tag_name"`
	LinkCount *int   `json:"link_count"`
}

func (t Tag) ID() string   { return t.TagID }
func (t Tag) Name() string { return t.TagName }

// Named is what the two have in common, so one command implementation can
// list, create and delete either.
type Named interface {
	ID() string
	Name() string
}

// Str reads a nullable string with a fallback, because every table cell needs
// one and `if p == nil` at each call site is noise.
func Str(p *string, fallback string) string {
	if p == nil || *p == "" {
		return fallback
	}
	return *p
}

// ShortID is the first eight characters of a UUID, which is enough to
// recognise a row and to paste back into another command.
//
// Bounds checked rather than sliced directly. Every id here is a UUID today,
// but `id[:8]` on an empty string is a panic, and a CLI that panics on a
// malformed row is worse than one that prints a short cell.
func ShortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// Day is the date part of an RFC 3339 timestamp, for a table cell where the
// time of day is noise. Same bounds reasoning as ShortID.
func Day(ts string) string {
	if len(ts) < 10 {
		return ts
	}
	return ts[:10]
}

// ReaderContent is GET /v1/links/{id}/content.
//
// The API returns sanitised HTML and no Markdown. The docs promise `read`
// prints Markdown by default, so the conversion happens here rather than the
// promise being quietly dropped. --raw prints what the API actually sent.
type ReaderContent struct {
	LinkID      string  `json:"link_id"`
	Title       *string `json:"title"`
	Byline      *string `json:"byline"`
	SiteName    *string `json:"site_name"`
	Excerpt     *string `json:"excerpt"`
	HTMLContent *string `json:"html_content"`
	WordCount   *int    `json:"word_count"`
}

// SearchHit is deliberately narrow. The three search RPCs return different
// column sets, and pinning all of them into one struct here would mean a
// column rename in Postgres silently becoming an empty table cell. These are
// the fields every mode returns.
type SearchHit struct {
	ID    string   `json:"id"`
	Title *string  `json:"title"`
	URL   *string  `json:"url"`
	Score *float64 `json:"score"`
}

type Highlight struct {
	ID           string  `json:"id"`
	LinkID       string  `json:"link_id"`
	ArticleURL   *string `json:"article_url"`
	SelectedText string  `json:"selected_text"`
	StartOffset  int     `json:"start_offset"`
	EndOffset    int     `json:"end_offset"`
	Color        *string `json:"color"`
	Annotation   *string `json:"annotation"`
	CreatedAt    string  `json:"created_at"`
}
