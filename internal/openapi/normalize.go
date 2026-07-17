package openapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NormalizeDocument converts supported legacy documents to the OpenAPI 3 shape
// consumed by search-fields and typesgen. OpenAPI 3 documents pass through.
func NormalizeDocument(data []byte) (json.RawMessage, error) {
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("接口文档不是有效 JSON: %w", err)
	}
	if document == nil {
		return nil, fmt.Errorf("接口文档必须是 JSON 对象。")
	}
	if swagger, _ := document["swagger"].(string); swagger != "2.0" {
		return append(json.RawMessage(nil), data...), nil
	}

	normalizeSwagger2(document)
	normalized, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("转换 Swagger 2.0 接口文档失败: %w", err)
	}
	return normalized, nil
}

// NormalizeCustomDocument normalizes a user-provided document and validates the
// minimum shape required by search/get before it is persisted as a snapshot.
func NormalizeCustomDocument(data []byte) (json.RawMessage, error) {
	normalized, err := NormalizeDocument(data)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal(normalized, &document); err != nil {
		return nil, fmt.Errorf("接口文档不是有效 JSON: %w", err)
	}
	paths, ok := document["paths"]
	if !ok {
		return nil, fmt.Errorf("接口文档缺少 paths。")
	}
	if _, ok := paths.(map[string]any); !ok {
		return nil, fmt.Errorf("接口文档的 paths 必须是 JSON 对象。")
	}
	return normalized, nil
}

func normalizeSwagger2(document map[string]any) {
	document["openapi"] = "3.0.3"
	delete(document, "swagger")

	components, _ := document["components"].(map[string]any)
	if components == nil {
		components = map[string]any{}
		document["components"] = components
	}
	if definitions, ok := document["definitions"].(map[string]any); ok {
		components["schemas"] = definitions
		delete(document, "definitions")
	}

	rootConsumes := stringList(document["consumes"])
	rootProduces := stringList(document["produces"])
	if paths, ok := document["paths"].(map[string]any); ok {
		for _, pathValue := range paths {
			pathItem, ok := pathValue.(map[string]any)
			if !ok {
				continue
			}
			for method, operationValue := range pathItem {
				if _, ok := httpMethods[strings.ToLower(method)]; !ok {
					continue
				}
				operation, ok := operationValue.(map[string]any)
				if !ok {
					continue
				}
				normalizeSwagger2Operation(operation, rootConsumes, rootProduces)
			}
		}
	}
	rewriteSwagger2Refs(document)
}

func normalizeSwagger2Operation(operation map[string]any, rootConsumes []string, rootProduces []string) {
	consumes := stringList(operation["consumes"])
	if len(consumes) == 0 {
		consumes = rootConsumes
	}
	if len(consumes) == 0 {
		consumes = []string{"application/json"}
	}
	produces := stringList(operation["produces"])
	if len(produces) == 0 {
		produces = rootProduces
	}
	if len(produces) == 0 {
		produces = []string{"application/json"}
	}

	if parameters, ok := operation["parameters"].([]any); ok {
		kept := make([]any, 0, len(parameters))
		for _, parameterValue := range parameters {
			parameter, ok := parameterValue.(map[string]any)
			if !ok {
				kept = append(kept, parameterValue)
				continue
			}
			in, _ := parameter["in"].(string)
			if in == "body" {
				schema, _ := parameter["schema"].(map[string]any)
				if schema != nil && operation["requestBody"] == nil {
					content := map[string]any{}
					for _, mediaType := range consumes {
						content[mediaType] = map[string]any{"schema": schema}
					}
					requestBody := map[string]any{"content": content}
					if required, _ := parameter["required"].(bool); required {
						requestBody["required"] = true
					}
					if description, _ := parameter["description"].(string); description != "" {
						requestBody["description"] = description
					}
					operation["requestBody"] = requestBody
				}
				continue
			}
			if parameter["schema"] == nil {
				if schema := schemaFromSwagger2Parameter(parameter); len(schema) > 0 {
					parameter["schema"] = schema
				}
			}
			kept = append(kept, parameter)
		}
		operation["parameters"] = kept
	}

	if responses, ok := operation["responses"].(map[string]any); ok {
		for _, responseValue := range responses {
			response, ok := responseValue.(map[string]any)
			if !ok || response["content"] != nil {
				continue
			}
			schema, ok := response["schema"]
			if !ok {
				continue
			}
			content := map[string]any{}
			for _, mediaType := range produces {
				content[mediaType] = map[string]any{"schema": schema}
			}
			response["content"] = content
			delete(response, "schema")
		}
	}
}

func schemaFromSwagger2Parameter(parameter map[string]any) map[string]any {
	schema := map[string]any{}
	for _, key := range []string{
		"type", "format", "items", "enum", "default", "minimum", "maximum",
		"minLength", "maxLength", "pattern", "minItems", "maxItems", "uniqueItems",
	} {
		if value, ok := parameter[key]; ok {
			schema[key] = value
		}
	}
	return schema
}

func stringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, text)
		}
	}
	return result
}

func rewriteSwagger2Refs(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "$ref" {
				if ref, ok := child.(string); ok && strings.HasPrefix(ref, "#/definitions/") {
					typed[key] = "#/components/schemas/" + strings.TrimPrefix(ref, "#/definitions/")
				}
				continue
			}
			rewriteSwagger2Refs(child)
		}
	case []any:
		for _, child := range typed {
			rewriteSwagger2Refs(child)
		}
	}
}
