package openapi

import (
	"strings"
)

// ScoreEndpoint returns the cumulative multi-keyword score for an endpoint.
// Every keyword contributes independently (order-independent).
func ScoreEndpoint(endpoint Endpoint, keywords []string) int {
	if len(keywords) == 0 {
		return 0
	}
	total := 0
	for _, keyword := range keywords {
		total += scoreOneKeyword(endpoint, keyword)
	}
	return total
}

func scoreOneKeyword(endpoint Endpoint, keyword string) int {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return 0
	}
	path := strings.ToLower(endpoint.Path)
	summary := strings.ToLower(endpoint.Summary)
	operationID := strings.ToLower(endpoint.OperationID)
	tags := strings.ToLower(strings.Join(endpoint.Tags, " "))
	schemaNames := strings.ToLower(strings.Join(endpoint.SchemaNames, " "))
	description := strings.ToLower(endpoint.Description)

	if path == keyword {
		return 1000
	}
	if summary == keyword {
		return 900
	}
	if fieldContains(path, keyword) && strings.Contains(path, "/"+keyword) {
		return 700
	}
	if fieldContains(path, keyword) {
		return 600
	}
	if fieldContains(summary, keyword) {
		return 500
	}
	if fieldContains(operationID, keyword) {
		return 400
	}
	if fieldContains(tags, keyword) {
		return 300
	}
	if fieldContains(schemaNames, keyword) {
		return 200
	}
	if fieldContains(description, keyword) {
		return 100
	}
	return 0
}

// MatchesEndpoint reports whether the endpoint should be recalled for the keyword set.
func MatchesEndpoint(endpoint Endpoint, keywords []string, mode string) bool {
	if len(keywords) == 0 {
		return true
	}
	if mode == "and" {
		for _, keyword := range keywords {
			if scoreOneKeyword(endpoint, keyword) == 0 && !endpointMatchesKeyword(endpoint, keyword) {
				return false
			}
			// score 0 but fuzzy/subword may still match via endpointMatchesKeyword
			if !endpointMatchesKeyword(endpoint, keyword) {
				return false
			}
		}
		return true
	}
	// OR
	for _, keyword := range keywords {
		if endpointMatchesKeyword(endpoint, keyword) {
			return true
		}
	}
	return false
}

func endpointMatchesKeyword(endpoint Endpoint, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	fields := []string{
		endpoint.Path,
		endpoint.Summary,
		endpoint.OperationID,
		strings.Join(endpoint.Tags, " "),
		strings.Join(endpoint.SchemaNames, " "),
		endpoint.Description,
		endpoint.Title,
	}
	for _, field := range fields {
		if fieldContains(field, keyword) {
			return true
		}
	}
	return false
}
