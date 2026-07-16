package openapi

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tokenize produces searchable tokens for a string: lowercased full form plus
// ASCII subwords (camelCase / PascalCase / snake_case / kebab-case).
// CJK characters are kept as exact runes and contiguous CJK runs; no fuzzy tokens.
func Tokenize(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	lowered := strings.ToLower(trimmed)

	seen := map[string]struct{}{}
	add := func(token string) {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
	}

	// Retain the full lowercased form.
	add(lowered)

	// Split original-case text so camelCase boundaries survive lowercasing.
	var segments []string
	var current strings.Builder
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || isCJK(r) {
			current.WriteRune(r)
			continue
		}
		if current.Len() > 0 {
			segments = append(segments, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	for _, segment := range segments {
		add(segment)
		for _, sub := range splitCamel(segment) {
			add(sub)
		}
		// Exact CJK character tokens (no edit-distance fuzzy for CJK).
		for _, r := range segment {
			if isCJK(r) {
				add(string(r))
			}
		}
	}

	out := make([]string, 0, len(seen))
	for token := range seen {
		out = append(out, token)
	}
	return out
}

func isCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// splitCamel splits ASCII camelCase / PascalCase into subwords.
// Segments containing CJK are returned as a single piece (plus per-rune tokens elsewhere).
func splitCamel(segment string) []string {
	if segment == "" {
		return nil
	}
	for _, r := range segment {
		if isCJK(r) {
			return []string{segment}
		}
	}

	runes := []rune(segment)
	if len(runes) == 0 {
		return nil
	}
	var parts []string
	start := 0
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		curr := runes[i]
		boundary := false
		// lower/digit → Upper
		if (unicode.IsLower(prev) || unicode.IsDigit(prev)) && unicode.IsUpper(curr) {
			boundary = true
		}
		// UPPER → Upper+lower (XMLParser → XML, Parser)
		if unicode.IsUpper(prev) && unicode.IsUpper(curr) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
			boundary = true
		}
		if boundary {
			parts = append(parts, string(runes[start:i]))
			start = i
		}
	}
	parts = append(parts, string(runes[start:]))
	for i := range parts {
		parts[i] = strings.ToLower(parts[i])
	}
	return parts
}

// fieldContains reports whether haystack matches needle with product rules:
// - exact substring
// - token/subword overlap
// - ASCII fuzzy (edit distance ≤1) only when needle token length > 3 and ASCII-only
func fieldContains(haystack string, needle string) bool {
	if needle == "" {
		return true
	}
	haystack = strings.ToLower(haystack)
	needle = strings.ToLower(needle)
	if strings.Contains(haystack, needle) {
		return true
	}

	// CJK: exact substring only (already checked). No edit-distance fuzzy and no
	// multi-character needle matching via a single shared CJK rune.
	if containsCJK(needle) {
		// Single CJK character may still match via token equality of that rune.
		if utf8.RuneCountInString(needle) == 1 {
			for _, hayToken := range Tokenize(haystack) {
				if hayToken == needle {
					return true
				}
			}
		}
		return false
	}

	hayTokens := Tokenize(haystack)
	needleTokens := Tokenize(needle)
	if len(needleTokens) == 0 {
		return false
	}

	// ASCII: match if any needle token (full form or subword) hits a hay token.
	for _, needleToken := range needleTokens {
		if containsCJK(needleToken) {
			continue
		}
		if tokenMatchesAny(needleToken, hayTokens) {
			return true
		}
	}
	// ASCII fuzzy (edit distance ≤1) only for long tokens.
	if isASCIIWord(needle) && utf8.RuneCountInString(needle) > 3 {
		for _, hayToken := range hayTokens {
			if isASCIIWord(hayToken) && levenshtein(needle, hayToken) <= 1 {
				return true
			}
		}
	}
	return false
}

func containsCJK(text string) bool {
	for _, r := range text {
		if isCJK(r) {
			return true
		}
	}
	return false
}

func tokenMatchesAny(needleToken string, hayTokens []string) bool {
	for _, hayToken := range hayTokens {
		if hayToken == needleToken || strings.Contains(hayToken, needleToken) || strings.Contains(needleToken, hayToken) {
			return true
		}
		if isASCIIWord(needleToken) && isASCIIWord(hayToken) && utf8.RuneCountInString(needleToken) > 3 {
			if levenshtein(needleToken, hayToken) <= 1 {
				return true
			}
		}
	}
	return false
}

func isASCIIWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r > 127 || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func levenshtein(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	if absInt(len(ar)-len(br)) > 1 {
		return absInt(len(ar) - len(br))
	}
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := 0; j <= len(br); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			deletion := prev[j] + 1
			insertion := curr[j-1] + 1
			substitution := prev[j-1] + cost
			curr[j] = minInt(deletion, minInt(insertion, substitution))
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
