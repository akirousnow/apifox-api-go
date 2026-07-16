// Package typesgen generates TypeScript types from OpenAPI operations.
package typesgen

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

var validJSIdent = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// Sanitize converts an OpenAPI name into a TypeScript type name base.
// Matches TypeScript: strip non [A-Za-z0-9_\u4e00-\u9fa5], upper first char.
func Sanitize(value string) string {
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || (r >= 0x4E00 && r <= 0x9FFF) {
			builder.WriteRune(r)
		}
	}
	s := builder.String()
	if s == "" {
		return "Anonymous"
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// PropertyKey returns a valid TS property key, quoting when needed.
func PropertyKey(name string) string {
	if validJSIdent.MatchString(name) {
		return name
	}
	encoded, _ := json.Marshal(name)
	return string(encoded)
}

// SafeComment sanitizes text for JSDoc (no premature comment close / newlines).
func SafeComment(value string) string {
	s := strings.ReplaceAll(value, "*/", "* /")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}
