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
