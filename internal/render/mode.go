// Package render decides how a result reaches the reader.
//
// Detected rather than flagged: a person at a terminal wants a table and a
// pipe wants JSON, and making someone remember a flag for that is a papercut
// on every single invocation.
package render

// Mode is how a command writes its result.
type Mode int

const (
	Table Mode = iota
	JSON
)

// Detect chooses. A pipe always wins: redirecting output is a stronger
// statement of intent than a config file written once months ago.
func Detect(isTTY, forceJSON bool, configFormat string) Mode {
	if forceJSON {
		return JSON
	}
	if !isTTY {
		return JSON
	}
	if configFormat == "json" {
		return JSON
	}
	return Table
}

// UseColor follows no-color.org: the presence of NO_COLOR turns colour off
// whatever its value, including an empty one. The config setting can force it
// either way.
func UseColor(setting string, noColorSet, isTTY bool) bool {
	switch setting {
	case "always":
		return true
	case "never":
		return false
	}
	return isTTY && !noColorSet
}
