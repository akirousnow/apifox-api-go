package openapi

import (
	"encoding/json"
	"sort"
	"strings"
)

// Endpoint is one Operation Key flattened from an OpenAPI Snapshot.
type Endpoint struct {
	Method      string
	Path        string
	Summary     string
	Title       string
	OperationID string
	Tags        []string
	Description string
	SchemaNames []string
}

var httpMethods = map[string]struct{}{
	"get": {}, "post": {}, "put": {}, "delete": {},
	"patch": {}, "head": {}, "options": {}, "trace": {},
}

// BuildIndex flattens OpenAPI paths into an Endpoint Index sorted by path then method.
func BuildIndex(openapiJSON []byte) ([]Endpoint, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(openapiJSON, &doc); err != nil {
		return nil, err
	}
	pathsRaw, ok := doc["paths"]
	if !ok {
		return []Endpoint{}, nil
	}
	var paths map[string]json.RawMessage
	if err := json.Unmarshal(pathsRaw, &paths); err != nil {
		return nil, err
	}

	endpoints := make([]Endpoint, 0)
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
			endpoints = append(endpoints, endpoint)
		}
	}

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})
	return endpoints, nil
}

func collectSchemaNames(pathItem map[string]json.RawMessage, operation map[string]json.RawMessage) []string {
	seen := map[string]struct{}{}
	var walk func(value any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if ref, ok := typed["$ref"].(string); ok && ref != "" {
				name := refName(ref)
				if name != "" {
					seen[name] = struct{}{}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}

	for _, key := range []string{"parameters", "requestBody", "responses"} {
		if raw, ok := operation[key]; ok {
			var decoded any
			if json.Unmarshal(raw, &decoded) == nil {
				walk(decoded)
			}
		}
	}
	if raw, ok := pathItem["parameters"]; ok {
		var decoded any
		if json.Unmarshal(raw, &decoded) == nil {
			walk(decoded)
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func refName(ref string) string {
	parts := strings.Split(ref, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
