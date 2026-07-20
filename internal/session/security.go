package session

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

var sessionIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidID reports whether id has the UUID shape used by Claude sessions.
func ValidID(id string) bool {
	return sessionIDRe.MatchString(id)
}

// SanitizeDisplay removes terminal escape sequences and control characters
// from untrusted session data before it reaches terminal rendering. Newlines
// and tabs are retained because previews need them for layout.
func SanitizeDisplay(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
