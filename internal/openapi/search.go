package openapi

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// DefaultSearchLimit is the default Search Result Window size.
	DefaultSearchLimit = 20
	// MaxSearchLimit is the maximum Search Result Window size.
	MaxSearchLimit = 50
)

// ValidHTTPMethods is the fixed set accepted by --method.
var ValidHTTPMethods = map[string]struct{}{
	"GET": {}, "POST": {}, "PUT": {}, "DELETE": {},
	"PATCH": {}, "HEAD": {}, "OPTIONS": {}, "TRACE": {},
}

// SearchOptions controls multi-field search with product-owned ranking.
type SearchOptions struct {
	Keywords []string
	Mode     string // "or" (default) or "and"
	Method   string
	Limit    int // 0 means default
}

// SearchResultWindow is the bounded search result payload.
type SearchResultWindow struct {
	Total     int
	Showing   int
	Truncated bool
	Limit     int
	Items     []Endpoint
}

// NormalizeSearchLimit validates and clamps the window size.
func NormalizeSearchLimit(limit int) (int, error) {
	if limit == 0 {
		return DefaultSearchLimit, nil
	}
	if limit < 1 || limit > MaxSearchLimit {
		return 0, fmt.Errorf("search 的 --limit 必须是 1 到 50 的整数。")
	}
	return limit, nil
}

// ValidateHTTPMethod normalizes and validates a --method value.
// Empty method is allowed (no filter). Invalid non-empty method is an error.
func ValidateHTTPMethod(method string) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(method))
	if trimmed == "" {
		return "", nil
	}
	if _, ok := ValidHTTPMethods[trimmed]; !ok {
		return "", fmt.Errorf(
			"--method 必须是合法 HTTP 方法（GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS/TRACE），收到: %s。",
			method,
		)
	}
	return trimmed, nil
}

// SearchWindow filters, ranks, and windows endpoints.
func SearchWindow(endpoints []Endpoint, options SearchOptions) (SearchResultWindow, error) {
	limit, err := NormalizeSearchLimit(options.Limit)
	if err != nil {
		return SearchResultWindow{}, err
	}

	keywords := normalizeKeywords(options.Keywords)
	method, err := ValidateHTTPMethod(options.Method)
	if err != nil {
		return SearchResultWindow{}, err
	}
	if len(keywords) == 0 && method == "" {
		return SearchResultWindow{}, fmt.Errorf("请提供 keywords 或 method，避免一次性返回整个项目的接口列表。")
	}

	mode := strings.ToLower(strings.TrimSpace(options.Mode))
	if mode == "" {
		mode = "or"
	}
	if mode != "or" && mode != "and" {
		return SearchResultWindow{}, fmt.Errorf("--mode 只接受 or 或 and，收到: %s。", options.Mode)
	}

	type ranked struct {
		endpoint Endpoint
		score    int
	}
	matched := make([]ranked, 0)
	for _, endpoint := range endpoints {
		if method != "" && endpoint.Method != method {
			continue
		}
		if len(keywords) == 0 {
			matched = append(matched, ranked{endpoint: endpoint, score: 0})
			continue
		}
		if !MatchesEndpoint(endpoint, keywords, mode) {
			continue
		}
		matched = append(matched, ranked{
			endpoint: endpoint,
			score:    ScoreEndpoint(endpoint, keywords),
		})
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].score != matched[j].score {
			return matched[i].score > matched[j].score
		}
		if matched[i].endpoint.Path != matched[j].endpoint.Path {
			return matched[i].endpoint.Path < matched[j].endpoint.Path
		}
		return matched[i].endpoint.Method < matched[j].endpoint.Method
	})

	items := make([]Endpoint, 0, len(matched))
	for _, item := range matched {
		items = append(items, item.endpoint)
	}
	truncated := false
	if len(items) > limit {
		items = items[:limit]
		truncated = true
	}

	return SearchResultWindow{
		Total:     len(matched),
		Showing:   len(items),
		Truncated: truncated,
		Limit:     limit,
		Items:     items,
	}, nil
}

func normalizeKeywords(keywords []string) []string {
	out := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		trimmed := strings.ToLower(strings.TrimSpace(keyword))
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
