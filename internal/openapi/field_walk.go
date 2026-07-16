package openapi

import (
	"encoding/json"
	"sort"
	"strings"
)

// MaxFieldWalkDepth is the maximum nesting depth when walking schema properties.
// Locked product decision for search-fields.
const MaxFieldWalkDepth = 8

// IndexedParam is a request parameter harvested for field-level search.
// Only query and path are indexed by product decision (no header/cookie).
type IndexedParam struct {
	Name        string
	In          string // "query" | "path"
	Description string
}

// IndexedField is a nested schema property harvested for field-level search.
type IndexedField struct {
	Name        string
	Description string
	JSONPath    string // dotted path, e.g. "user.profile.email"
	Source      string // "body" | "response"
}

// OperationFieldIndex holds param/body/response field text for one operation.
type OperationFieldIndex struct {
	RequestParams  []IndexedParam
	RequestFields  []IndexedField
	ResponseFields []IndexedField
}

// FieldWalkOptions controls which surfaces are harvested.
type FieldWalkOptions struct {
	// IncludeBody walks request body schema properties.
	IncludeBody bool
	// IncludeResponse walks primary 2xx JSON response schema properties.
	IncludeResponse bool
}

// CollectOperationFields extracts query/path parameters and optional body/response
// nested fields from one OpenAPI path item + operation, using components.schemas
// for $ref resolution.
//
// Pure data walk — does not render TypeScript.
func CollectOperationFields(
	pathItem map[string]json.RawMessage,
	operation map[string]json.RawMessage,
	componentsSchemas map[string]json.RawMessage,
	options FieldWalkOptions,
) OperationFieldIndex {
	result := OperationFieldIndex{
		RequestParams:  mergeIndexedParameters(pathItem, operation),
		RequestFields:  []IndexedField{},
		ResponseFields: []IndexedField{},
	}

	if options.IncludeBody {
		if schemaRaw := selectRequestBodySchema(operation); len(schemaRaw) > 0 {
			result.RequestFields = walkSchemaFields(
				schemaRaw,
				componentsSchemas,
				"body",
				"",
				0,
				map[string]struct{}{},
			)
		}
	}

	if options.IncludeResponse {
		if schemaRaw := selectPrimarySuccessJSONSchema(operation); len(schemaRaw) > 0 {
			result.ResponseFields = walkSchemaFields(
				schemaRaw,
				componentsSchemas,
				"response",
				"",
				0,
				map[string]struct{}{},
			)
		}
	}

	return result
}

// ExtractComponentsSchemas pulls components.schemas from a root OpenAPI document map.
func ExtractComponentsSchemas(doc map[string]json.RawMessage) map[string]json.RawMessage {
	schemas := map[string]json.RawMessage{}
	componentsRaw, ok := doc["components"]
	if !ok {
		return schemas
	}
	var components map[string]json.RawMessage
	if err := json.Unmarshal(componentsRaw, &components); err != nil {
		return schemas
	}
	schemasRaw, ok := components["schemas"]
	if !ok {
		return schemas
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(schemasRaw, &decoded); err != nil {
		return schemas
	}
	return decoded
}

func mergeIndexedParameters(
	pathItem map[string]json.RawMessage,
	operation map[string]json.RawMessage,
) []IndexedParam {
	byKey := map[string]IndexedParam{}
	order := make([]string, 0)

	appendParams := func(raw json.RawMessage) {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return
		}
		for _, itemRaw := range items {
			var object map[string]json.RawMessage
			if err := json.Unmarshal(itemRaw, &object); err != nil {
				continue
			}
			var name string
			var inValue string
			if nameRaw, ok := object["name"]; ok {
				_ = json.Unmarshal(nameRaw, &name)
			}
			if inRaw, ok := object["in"]; ok {
				_ = json.Unmarshal(inRaw, &inValue)
			}
			name = strings.TrimSpace(name)
			inValue = strings.ToLower(strings.TrimSpace(inValue))
			// Locked: only query + path (no header/cookie).
			if name == "" || (inValue != "query" && inValue != "path") {
				continue
			}
			param := IndexedParam{Name: name, In: inValue}
			if descRaw, ok := object["description"]; ok {
				_ = json.Unmarshal(descRaw, &param.Description)
			}
			key := inValue + ":" + name
			if _, exists := byKey[key]; !exists {
				order = append(order, key)
			}
			byKey[key] = param
		}
	}

	if pathParamsRaw, ok := pathItem["parameters"]; ok {
		appendParams(pathParamsRaw)
	}
	if opParamsRaw, ok := operation["parameters"]; ok {
		appendParams(opParamsRaw)
	}

	result := make([]IndexedParam, 0, len(order))
	for _, key := range order {
		result = append(result, byKey[key])
	}
	return result
}

func selectRequestBodySchema(operation map[string]json.RawMessage) json.RawMessage {
	requestBodyRaw, ok := operation["requestBody"]
	if !ok {
		return nil
	}
	var requestBody map[string]json.RawMessage
	if err := json.Unmarshal(requestBodyRaw, &requestBody); err != nil {
		return nil
	}
	contentRaw, ok := requestBody["content"]
	if !ok {
		return nil
	}
	var content map[string]json.RawMessage
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return nil
	}
	preferred := []string{
		"application/json",
		"application/x-www-form-urlencoded",
		"multipart/form-data",
	}
	for _, mediaType := range preferred {
		for candidate, entryRaw := range content {
			if strings.ToLower(candidate) != mediaType {
				continue
			}
			var entry map[string]json.RawMessage
			if err := json.Unmarshal(entryRaw, &entry); err != nil {
				continue
			}
			if schemaRaw, ok := entry["schema"]; ok {
				return schemaRaw
			}
		}
	}
	for _, entryRaw := range content {
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(entryRaw, &entry); err != nil {
			continue
		}
		if schemaRaw, ok := entry["schema"]; ok {
			return schemaRaw
		}
	}
	return nil
}

func selectPrimarySuccessJSONSchema(operation map[string]json.RawMessage) json.RawMessage {
	responsesRaw, ok := operation["responses"]
	if !ok {
		return nil
	}
	var responses map[string]json.RawMessage
	if err := json.Unmarshal(responsesRaw, &responses); err != nil {
		return nil
	}

	if status200Raw, ok := responses["200"]; ok {
		if schemaRaw := jsonSchemaFromResponse(status200Raw); len(schemaRaw) > 0 {
			return schemaRaw
		}
	}

	statusCodes := make([]string, 0, len(responses))
	for statusCode := range responses {
		statusCodes = append(statusCodes, statusCode)
	}
	sort.Strings(statusCodes)

	for _, statusCode := range statusCodes {
		if statusCode == "200" {
			continue
		}
		if !isTwoXXStatus(statusCode) {
			continue
		}
		if schemaRaw := jsonSchemaFromResponse(responses[statusCode]); len(schemaRaw) > 0 {
			return schemaRaw
		}
	}
	return nil
}

func isTwoXXStatus(statusCode string) bool {
	if len(statusCode) != 3 || statusCode[0] != '2' {
		return false
	}
	return statusCode[1] >= '0' && statusCode[1] <= '9' && statusCode[2] >= '0' && statusCode[2] <= '9'
}

func jsonSchemaFromResponse(responseRaw json.RawMessage) json.RawMessage {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(responseRaw, &response); err != nil {
		return nil
	}
	contentRaw, ok := response["content"]
	if !ok {
		return nil
	}
	var content map[string]json.RawMessage
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return nil
	}

	var exact json.RawMessage
	var suffix json.RawMessage
	for mediaType, entryRaw := range content {
		lower := strings.ToLower(mediaType)
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(entryRaw, &entry); err != nil {
			continue
		}
		schemaRaw, ok := entry["schema"]
		if !ok || len(schemaRaw) == 0 {
			continue
		}
		if lower == "application/json" && len(exact) == 0 {
			exact = schemaRaw
		}
		if strings.HasSuffix(lower, "+json") && len(suffix) == 0 {
			suffix = schemaRaw
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return suffix
}

// walkSchemaFields recursively collects property name + description from a schema.
// depth counts property nesting levels; MaxFieldWalkDepth stops deeper walks.
// visited tracks component $ref strings already entered on the current path (cycle-safe).
func walkSchemaFields(
	schemaRaw json.RawMessage,
	componentsSchemas map[string]json.RawMessage,
	source string,
	parentPath string,
	depth int,
	visited map[string]struct{},
) []IndexedField {
	if len(schemaRaw) == 0 || depth >= MaxFieldWalkDepth {
		return nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(schemaRaw, &object); err != nil {
		return nil
	}

	if refRaw, ok := object["$ref"]; ok {
		var ref string
		if err := json.Unmarshal(refRaw, &ref); err != nil || ref == "" {
			return nil
		}
		if _, seen := visited[ref]; seen {
			return nil
		}
		resolved, ok := schemaFromComponentsRef(ref, componentsSchemas)
		if !ok {
			return nil
		}
		nextVisited := copyStringSet(visited)
		nextVisited[ref] = struct{}{}
		return walkSchemaFields(resolved, componentsSchemas, source, parentPath, depth, nextVisited)
	}

	fields := make([]IndexedField, 0)

	if allOfRaw, ok := object["allOf"]; ok {
		var parts []json.RawMessage
		if err := json.Unmarshal(allOfRaw, &parts); err == nil {
			for _, partRaw := range parts {
				fields = append(fields, walkSchemaFields(
					partRaw, componentsSchemas, source, parentPath, depth, visited,
				)...)
			}
		}
	}

	if itemsRaw, ok := object["items"]; ok {
		fields = append(fields, walkSchemaFields(
			itemsRaw, componentsSchemas, source, parentPath, depth, visited,
		)...)
	}

	propsRaw, ok := object["properties"]
	if !ok {
		return fields
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(propsRaw, &properties); err != nil {
		return fields
	}

	propertyNames := make([]string, 0, len(properties))
	for propertyName := range properties {
		propertyNames = append(propertyNames, propertyName)
	}
	sort.Strings(propertyNames)

	for _, propertyName := range propertyNames {
		propertyRaw := properties[propertyName]
		jsonPath := propertyName
		if parentPath != "" {
			jsonPath = parentPath + "." + propertyName
		}

		var description string
		var propertyObject map[string]json.RawMessage
		if err := json.Unmarshal(propertyRaw, &propertyObject); err == nil {
			if descRaw, ok := propertyObject["description"]; ok {
				_ = json.Unmarshal(descRaw, &description)
			}
			if description == "" {
				if refRaw, ok := propertyObject["$ref"]; ok {
					var ref string
					if json.Unmarshal(refRaw, &ref) == nil && ref != "" {
						if resolved, ok := schemaFromComponentsRef(ref, componentsSchemas); ok {
							var resolvedObject map[string]json.RawMessage
							if json.Unmarshal(resolved, &resolvedObject) == nil {
								if descRaw, ok := resolvedObject["description"]; ok {
									_ = json.Unmarshal(descRaw, &description)
								}
							}
						}
					}
				}
			}
		}

		fields = append(fields, IndexedField{
			Name:        propertyName,
			Description: description,
			JSONPath:    jsonPath,
			Source:      source,
		})

		if depth+1 < MaxFieldWalkDepth {
			fields = append(fields, walkSchemaFields(
				propertyRaw,
				componentsSchemas,
				source,
				jsonPath,
				depth+1,
				visited,
			)...)
		}
	}

	return fields
}

func schemaFromComponentsRef(ref string, componentsSchemas map[string]json.RawMessage) (json.RawMessage, bool) {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, false
	}
	name := strings.TrimPrefix(ref, prefix)
	if name == "" || strings.Contains(name, "/") {
		name = refName(ref)
	}
	raw, ok := componentsSchemas[name]
	return raw, ok && len(raw) > 0
}

func copyStringSet(source map[string]struct{}) map[string]struct{} {
	copied := make(map[string]struct{}, len(source))
	for key := range source {
		copied[key] = struct{}{}
	}
	return copied
}
