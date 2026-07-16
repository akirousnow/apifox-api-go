package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FieldEndpoint embeds Operation Key metadata with harvested field index data.
type FieldEndpoint struct {
	Endpoint
	Fields OperationFieldIndex
}

// FieldMatch is one keyword hit on a param or nested field for display.
type FieldMatch struct {
	// Kind is "param", "body", or "response".
	Kind string `json:"kind"`
	// Location is "query", "path", "body", or "response".
	Location string `json:"location"`
	// Name is the leaf property or parameter name.
	Name string `json:"name"`
	// JSONPath is the dotted path for body/response fields (empty for params).
	JSONPath string `json:"jsonPath,omitempty"`
	// Description is the OpenAPI description text when present.
	Description string `json:"description,omitempty"`
	// Display is the human-facing label, e.g. "query.phone(手机号)" or "body.user.email".
	Display string `json:"display"`
}

// FieldSearchItem is one ranked endpoint with its field-level hits.
type FieldSearchItem struct {
	Endpoint FieldEndpoint
	Matches  []FieldMatch
	Score    int
}

// FieldSearchResultWindow is the bounded field-search result payload.
type FieldSearchResultWindow struct {
	Total     int
	Showing   int
	Truncated bool
	Limit     int
	Items     []FieldSearchItem
}

// BuildFieldIndex flattens OpenAPI paths into FieldEndpoints with full field walks
// (query/path params + body + primary 2xx JSON response).
func BuildFieldIndex(openapiJSON []byte) ([]FieldEndpoint, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(openapiJSON, &doc); err != nil {
		return nil, err
	}
	pathsRaw, ok := doc["paths"]
	if !ok {
		return []FieldEndpoint{}, nil
	}
	var paths map[string]json.RawMessage
	if err := json.Unmarshal(pathsRaw, &paths); err != nil {
		return nil, err
	}

	componentsSchemas := ExtractComponentsSchemas(doc)
	walkOptions := FieldWalkOptions{
		IncludeBody:     true,
		IncludeResponse: true,
	}

	fieldEndpoints := make([]FieldEndpoint, 0)
	for apiPath, pathItemRaw := range paths {
		var pathItem map[string]json.RawMessage
		if err := json.Unmarshal(pathItemRaw, &pathItem); err != nil {
			continue
		}
		for methodKey, operationRaw := range pathItem {
			lowerMethod := strings.ToLower(methodKey)
			if _, ok := httpMethods[lowerMethod]; !ok {
				continue
			}
			var operation map[string]json.RawMessage
			if err := json.Unmarshal(operationRaw, &operation); err != nil {
				continue
			}
			endpoint := Endpoint{
				Method:      strings.ToUpper(lowerMethod),
				Path:        apiPath,
				Tags:        []string{},
				SchemaNames: []string{},
			}
			if summaryRaw, ok := operation["summary"]; ok {
				_ = json.Unmarshal(summaryRaw, &endpoint.Summary)
			}
			if operationIDRaw, ok := operation["operationId"]; ok {
				_ = json.Unmarshal(operationIDRaw, &endpoint.OperationID)
			}
			if descriptionRaw, ok := operation["description"]; ok {
				_ = json.Unmarshal(descriptionRaw, &endpoint.Description)
			}
			if tagsRaw, ok := operation["tags"]; ok {
				var tags []string
				if err := json.Unmarshal(tagsRaw, &tags); err == nil {
					endpoint.Tags = tags
				}
			}
			if endpoint.Summary != "" {
				endpoint.Title = endpoint.Summary
			} else {
				endpoint.Title = endpoint.Method + " " + endpoint.Path
			}
			endpoint.SchemaNames = collectSchemaNames(pathItem, operation)

			fields := CollectOperationFields(pathItem, operation, componentsSchemas, walkOptions)
			fieldEndpoints = append(fieldEndpoints, FieldEndpoint{
				Endpoint: endpoint,
				Fields:   fields,
			})
		}
	}

	sort.Slice(fieldEndpoints, func(i, j int) bool {
		if fieldEndpoints[i].Path != fieldEndpoints[j].Path {
			return fieldEndpoints[i].Path < fieldEndpoints[j].Path
		}
		return fieldEndpoints[i].Method < fieldEndpoints[j].Method
	})
	return fieldEndpoints, nil
}

// SearchByFields filters, ranks, and windows endpoints by field-level keyword hits.
// Keywords are required (method-only search is rejected for this command).
func SearchByFields(endpoints []FieldEndpoint, options SearchOptions) (FieldSearchResultWindow, error) {
	limit, err := NormalizeSearchLimit(options.Limit)
	if err != nil {
		return FieldSearchResultWindow{}, err
	}

	keywords := normalizeKeywords(options.Keywords)
	if len(keywords) == 0 {
		return FieldSearchResultWindow{}, fmt.Errorf("请提供 keywords，search-fields 必须按字段关键词检索。")
	}

	method, err := ValidateHTTPMethod(options.Method)
	if err != nil {
		return FieldSearchResultWindow{}, err
	}

	mode := strings.ToLower(strings.TrimSpace(options.Mode))
	if mode == "" {
		mode = "or"
	}
	if mode != "or" && mode != "and" {
		return FieldSearchResultWindow{}, fmt.Errorf("--mode 只接受 or 或 and，收到: %s。", options.Mode)
	}

	type ranked struct {
		item  FieldSearchItem
		score int
	}
	matched := make([]ranked, 0)
	for _, endpoint := range endpoints {
		if method != "" && endpoint.Method != method {
			continue
		}
		matches, score, ok := matchFieldEndpoint(endpoint, keywords, mode)
		if !ok {
			continue
		}
		matched = append(matched, ranked{
			item: FieldSearchItem{
				Endpoint: endpoint,
				Matches:  matches,
				Score:    score,
			},
			score: score,
		})
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].score != matched[j].score {
			return matched[i].score > matched[j].score
		}
		if matched[i].item.Endpoint.Path != matched[j].item.Endpoint.Path {
			return matched[i].item.Endpoint.Path < matched[j].item.Endpoint.Path
		}
		return matched[i].item.Endpoint.Method < matched[j].item.Endpoint.Method
	})

	items := make([]FieldSearchItem, 0, len(matched))
	for _, entry := range matched {
		items = append(items, entry.item)
	}
	truncated := false
	if len(items) > limit {
		items = items[:limit]
		truncated = true
	}

	return FieldSearchResultWindow{
		Total:     len(matched),
		Showing:   len(items),
		Truncated: truncated,
		Limit:     limit,
		Items:     items,
	}, nil
}

func matchFieldEndpoint(endpoint FieldEndpoint, keywords []string, mode string) ([]FieldMatch, int, bool) {
	// Collect unique matches across all keywords while accumulating score.
	matchByDisplay := map[string]FieldMatch{}
	matchOrder := make([]string, 0)
	totalScore := 0

	keywordHit := make([]bool, len(keywords))
	for keywordIndex, keyword := range keywords {
		keywordMatches, keywordScore := scoreFieldKeyword(endpoint, keyword)
		if len(keywordMatches) == 0 {
			continue
		}
		keywordHit[keywordIndex] = true
		totalScore += keywordScore
		for _, match := range keywordMatches {
			if _, exists := matchByDisplay[match.Display]; !exists {
				matchOrder = append(matchOrder, match.Display)
			}
			matchByDisplay[match.Display] = match
		}
	}

	if mode == "and" {
		for _, hit := range keywordHit {
			if !hit {
				return nil, 0, false
			}
		}
	} else {
		anyHit := false
		for _, hit := range keywordHit {
			if hit {
				anyHit = true
				break
			}
		}
		if !anyHit {
			return nil, 0, false
		}
	}

	matches := make([]FieldMatch, 0, len(matchOrder))
	for _, display := range matchOrder {
		matches = append(matches, matchByDisplay[display])
	}
	return matches, totalScore, true
}

func scoreFieldKeyword(endpoint FieldEndpoint, keyword string) ([]FieldMatch, int) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return nil, 0
	}

	matches := make([]FieldMatch, 0)
	score := 0

	for _, param := range endpoint.Fields.RequestParams {
		nameLower := strings.ToLower(param.Name)
		nameHit := fieldContains(param.Name, keyword)
		descHit := param.Description != "" && fieldContains(param.Description, keyword)
		if !nameHit && !descHit {
			continue
		}
		if nameLower == keyword {
			score += 1000
		} else if nameHit {
			score += 700
		}
		if descHit {
			score += 200
		}
		matches = append(matches, formatParamMatch(param))
	}

	for _, field := range endpoint.Fields.RequestFields {
		fieldScore, hit := scoreIndexedField(field, keyword)
		if !hit {
			continue
		}
		score += fieldScore
		matches = append(matches, formatIndexedFieldMatch(field))
	}

	for _, field := range endpoint.Fields.ResponseFields {
		fieldScore, hit := scoreIndexedField(field, keyword)
		if !hit {
			continue
		}
		score += fieldScore
		matches = append(matches, formatIndexedFieldMatch(field))
	}

	return matches, score
}

func scoreIndexedField(field IndexedField, keyword string) (int, bool) {
	nameLower := strings.ToLower(field.Name)
	nameHit := fieldContains(field.Name, keyword)
	// Also allow matching the dotted path segments (e.g. "user.email").
	pathHit := field.JSONPath != "" && fieldContains(field.JSONPath, keyword)
	descHit := field.Description != "" && fieldContains(field.Description, keyword)
	if !nameHit && !pathHit && !descHit {
		return 0, false
	}
	score := 0
	if nameLower == keyword {
		score += 800
	} else if nameHit || pathHit {
		score += 500
	}
	if descHit {
		score += 100
	}
	return score, true
}

func formatParamMatch(param IndexedParam) FieldMatch {
	display := param.In + "." + param.Name
	if strings.TrimSpace(param.Description) != "" {
		display += "(" + strings.TrimSpace(param.Description) + ")"
	}
	return FieldMatch{
		Kind:        "param",
		Location:    param.In,
		Name:        param.Name,
		Description: param.Description,
		Display:     display,
	}
}

func formatIndexedFieldMatch(field IndexedField) FieldMatch {
	path := field.JSONPath
	if path == "" {
		path = field.Name
	}
	display := field.Source + "." + path
	if strings.TrimSpace(field.Description) != "" {
		display += "(" + strings.TrimSpace(field.Description) + ")"
	}
	return FieldMatch{
		Kind:        field.Source,
		Location:    field.Source,
		Name:        field.Name,
		JSONPath:    field.JSONPath,
		Description: field.Description,
		Display:     display,
	}
}

// FormatMatchedFields joins hit displays for markdown 命中字段 column.
func FormatMatchedFields(matches []FieldMatch) string {
	if len(matches) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		parts = append(parts, match.Display)
	}
	return strings.Join(parts, "; ")
}
