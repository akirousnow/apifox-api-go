package typesgen

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// HTTPMethods is the canonical method order for path-only generation.
var HTTPMethods = []string{"get", "post", "put", "delete", "patch", "head", "options", "trace"}

// OperationContext is a resolved OpenAPI operation for type generation.
type OperationContext struct {
	Method    string
	Path      string
	PathItem  OrderedObject
	Operation OrderedObject
	Schemas   OrderedObject
}

// GenTypesForOperation generates TypeScript for one method+path.
func GenTypesForOperation(openapiJSON []byte, method string, apiPath string) (string, error) {
	ctx, err := GetOperationContext(json.RawMessage(openapiJSON), method, apiPath)
	if err != nil {
		return "", err
	}
	return GenTypesForOperationContext(ctx), nil
}

// GenTypesForAllOperationsOnPath generates types for every method on a path.
// Shared $ref schemas are drained once so exports are not duplicated.
func GenTypesForAllOperationsOnPath(openapiJSON []byte, apiPath string) (string, error) {
	methods, _, schemas, err := MethodsOnPath(json.RawMessage(openapiJSON), apiPath)
	if err != nil {
		return "", err
	}
	renderer := newTypeRenderer(schemas)
	parts := make([]string, 0, len(methods))
	for _, method := range methods {
		ctx, err := GetOperationContext(json.RawMessage(openapiJSON), method, apiPath)
		if err != nil {
			return "", err
		}
		block := fmt.Sprintf("// ═════════ %s %s ═════════\n\n%s", ctx.Method, ctx.Path, strings.Join(renderOperationTypes(ctx, renderer), "\n"))
		parts = append(parts, block)
	}
	out := strings.Join(parts, "\n\n")
	refs := renderer.drain()
	if len(refs) > 0 {
		out += "\n\n// ====== 关联类型定义 ======\n" + strings.Join(refs, "\n")
	}
	return out, nil
}

// GenTypesForOperationContext generates types for a resolved operation context.
func GenTypesForOperationContext(ctx OperationContext) string {
	renderer := newTypeRenderer(ctx.Schemas)
	lines := renderOperationTypes(ctx, renderer)
	refs := renderer.drain()
	if len(refs) > 0 {
		lines = append(lines, "\n// ====== 关联类型定义 ======")
		lines = append(lines, refs...)
	}
	return strings.Join(lines, "\n")
}

// GetOperationContext resolves method+path against an OpenAPI document.
func GetOperationContext(openapiJSON json.RawMessage, method string, apiPath string) (OperationContext, error) {
	doc, err := ParseOrderedObject(openapiJSON)
	if err != nil {
		return OperationContext{}, fmt.Errorf("无效的 OpenAPI 文档: %w", err)
	}
	normalizedMethod := strings.ToLower(strings.TrimSpace(method))
	normalizedPath := strings.TrimSpace(apiPath)
	if normalizedMethod == "" {
		return OperationContext{}, fmt.Errorf("请提供 method。可先运行 apifox-api search 获取完整的 method + path。")
	}
	if normalizedPath == "" {
		return OperationContext{}, fmt.Errorf("请提供 path。可先运行 apifox-api search 获取完整的 method + path。")
	}
	valid := false
	for _, candidate := range HTTPMethods {
		if candidate == normalizedMethod {
			valid = true
			break
		}
	}
	if !valid {
		return OperationContext{}, fmt.Errorf("无效的 HTTP method: %s", method)
	}
	pathsRaw, ok := doc.Get("paths")
	if !ok {
		return OperationContext{}, fmt.Errorf("未找到接口路径 %s。请先运行 apifox-api search 获取完整 path。", normalizedPath)
	}
	paths, err := ParseOrderedObject(pathsRaw)
	if err != nil {
		return OperationContext{}, err
	}
	pathItemRaw, ok := paths.Get(normalizedPath)
	if !ok {
		return OperationContext{}, fmt.Errorf("未找到接口路径 %s。请先运行 apifox-api search 获取完整 path。", normalizedPath)
	}
	pathItem, err := ParseOrderedObject(pathItemRaw)
	if err != nil {
		return OperationContext{}, err
	}
	operationRaw, ok := pathItem.Get(normalizedMethod)
	if !ok {
		available := make([]string, 0)
		for _, candidate := range HTTPMethods {
			if _, exists := pathItem.Get(candidate); exists {
				available = append(available, strings.ToUpper(candidate))
			}
		}
		suffix := ""
		if len(available) > 0 {
			suffix = "。该路径可用方法: " + strings.Join(available, ", ")
		}
		return OperationContext{}, fmt.Errorf("未找到接口 %s %s%s", strings.ToUpper(normalizedMethod), normalizedPath, suffix)
	}
	operation, err := ParseOrderedObject(operationRaw)
	if err != nil {
		return OperationContext{}, err
	}
	return OperationContext{
		Method:    strings.ToUpper(normalizedMethod),
		Path:      normalizedPath,
		PathItem:  pathItem,
		Operation: operation,
		Schemas:   loadSchemas(doc),
	}, nil
}

// MethodsOnPath returns HTTP methods present on a path in GET..TRACE order.
func MethodsOnPath(openapiJSON json.RawMessage, apiPath string) (methods []string, pathItem OrderedObject, schemas OrderedObject, err error) {
	doc, err := ParseOrderedObject(openapiJSON)
	if err != nil {
		return nil, OrderedObject{}, OrderedObject{}, fmt.Errorf("无效的 OpenAPI 文档: %w", err)
	}
	normalizedPath := strings.TrimSpace(apiPath)
	if normalizedPath == "" {
		return nil, OrderedObject{}, OrderedObject{}, fmt.Errorf("请提供 path。可先运行 apifox-api search 获取完整的 method + path。")
	}
	pathsRaw, ok := doc.Get("paths")
	if !ok {
		return nil, OrderedObject{}, OrderedObject{}, fmt.Errorf("未找到接口路径 %s。请先运行 apifox-api search 获取完整 path。", normalizedPath)
	}
	paths, err := ParseOrderedObject(pathsRaw)
	if err != nil {
		return nil, OrderedObject{}, OrderedObject{}, err
	}
	pathItemRaw, ok := paths.Get(normalizedPath)
	if !ok {
		return nil, OrderedObject{}, OrderedObject{}, fmt.Errorf("未找到接口路径 %s。请先运行 apifox-api search 获取完整 path。", normalizedPath)
	}
	pathItem, err = ParseOrderedObject(pathItemRaw)
	if err != nil {
		return nil, OrderedObject{}, OrderedObject{}, err
	}
	for _, candidate := range HTTPMethods {
		if _, exists := pathItem.Get(candidate); exists {
			methods = append(methods, candidate)
		}
	}
	if len(methods) == 0 {
		return nil, OrderedObject{}, OrderedObject{}, fmt.Errorf("路径 %s 下没有可用的 HTTP method。", normalizedPath)
	}
	return methods, pathItem, loadSchemas(doc), nil
}

func loadSchemas(doc OrderedObject) OrderedObject {
	empty := OrderedObject{Values: map[string]json.RawMessage{}}
	componentsRaw, ok := doc.Get("components")
	if !ok {
		return empty
	}
	components, err := ParseOrderedObject(componentsRaw)
	if err != nil {
		return empty
	}
	schemasRaw, ok := components.Get("schemas")
	if !ok {
		return empty
	}
	schemas, err := ParseOrderedObject(schemasRaw)
	if err != nil {
		return empty
	}
	return schemas
}

func operationTypeName(ctx OperationContext) string {
	source := ""
	if raw, ok := ctx.Operation.Get("operationId"); ok {
		var value string
		if json.Unmarshal(raw, &value) == nil && value != "" {
			source = value
		}
	}
	if source == "" {
		if raw, ok := ctx.Operation.Get("summary"); ok {
			var value string
			if json.Unmarshal(raw, &value) == nil && value != "" {
				source = value
			}
		}
	}
	if source == "" {
		source = ctx.Method + "_" + ctx.Path
	}
	name := Sanitize(source)
	if name == "" {
		return "Api"
	}
	return name
}

type paramSpec struct {
	name        string
	in          string
	required    bool
	description string
	schemaRaw   json.RawMessage
}

type parameterGroupSpec struct {
	inType string
	label  string
	suffix string
}

var parameterGroupSpecs = []parameterGroupSpec{
	{inType: "query", label: "Query 参数（URL 问号后）", suffix: "Query"},
	{inType: "path", label: "Path 参数（URL 路径占位）", suffix: "PathParams"},
	{inType: "header", label: "Header 参数（请求头）", suffix: "Headers"},
	{inType: "cookie", label: "Cookie 参数", suffix: "Cookies"},
}

func mergeParameters(pathItem, operation OrderedObject) []paramSpec {
	byKey := map[string]paramSpec{}
	order := make([]string, 0)
	appendParams := func(raw json.RawMessage) {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return
		}
		for _, itemRaw := range items {
			object, err := ParseOrderedObject(itemRaw)
			if err != nil {
				continue
			}
			var name string
			var inValue string
			if nameRaw, ok := object.Get("name"); ok {
				_ = json.Unmarshal(nameRaw, &name)
			}
			if inRaw, ok := object.Get("in"); ok {
				_ = json.Unmarshal(inRaw, &inValue)
			}
			if name == "" || inValue == "" {
				continue
			}
			param := paramSpec{name: name, in: inValue}
			if requiredRaw, ok := object.Get("required"); ok {
				_ = json.Unmarshal(requiredRaw, &param.required)
			}
			if descRaw, ok := object.Get("description"); ok {
				_ = json.Unmarshal(descRaw, &param.description)
			}
			if schemaRaw, ok := object.Get("schema"); ok {
				param.schemaRaw = schemaRaw
			} else if typeRaw, ok := object.Get("type"); ok {
				var typeName string
				if err := json.Unmarshal(typeRaw, &typeName); err == nil && typeName != "" {
					param.schemaRaw = json.RawMessage(fmt.Sprintf(`{"type":%q}`, typeName))
				}
			}
			if len(param.schemaRaw) == 0 {
				param.schemaRaw = json.RawMessage(`{"type":"string"}`)
			}
			key := inValue + ":" + name
			if _, exists := byKey[key]; !exists {
				order = append(order, key)
			}
			byKey[key] = param
		}
	}
	if pathParamsRaw, ok := pathItem.Get("parameters"); ok {
		appendParams(pathParamsRaw)
	}
	if opParamsRaw, ok := operation.Get("parameters"); ok {
		appendParams(opParamsRaw)
	}
	result := make([]paramSpec, 0, len(order))
	for _, key := range order {
		result = append(result, byKey[key])
	}
	return result
}

type mediaSchema struct {
	mediaType string
	schemaRaw json.RawMessage
}

func selectJSONContent(content OrderedObject) *mediaSchema {
	var exact *mediaSchema
	var suffix *mediaSchema
	for _, mediaType := range content.Keys {
		lower := strings.ToLower(mediaType)
		entry, err := ParseOrderedObject(content.Values[mediaType])
		if err != nil {
			continue
		}
		schemaRaw, _ := entry.Get("schema")
		candidate := &mediaSchema{mediaType: mediaType, schemaRaw: schemaRaw}
		if lower == "application/json" && exact == nil {
			exact = candidate
		}
		if strings.HasSuffix(lower, "+json") && suffix == nil {
			suffix = candidate
		}
	}
	if exact != nil {
		return exact
	}
	return suffix
}

func selectRequestBodyContent(operation OrderedObject) *mediaSchema {
	requestBodyRaw, ok := operation.Get("requestBody")
	if !ok {
		return nil
	}
	requestBody, err := ParseOrderedObject(requestBodyRaw)
	if err != nil {
		return nil
	}
	contentRaw, ok := requestBody.Get("content")
	if !ok {
		return nil
	}
	content, err := ParseOrderedObject(contentRaw)
	if err != nil {
		return nil
	}
	preferred := []string{"application/json", "application/x-www-form-urlencoded", "multipart/form-data"}
	for _, mediaType := range preferred {
		for _, candidate := range content.Keys {
			if strings.ToLower(candidate) == mediaType {
				entry, err := ParseOrderedObject(content.Values[candidate])
				if err != nil {
					continue
				}
				schemaRaw, _ := entry.Get("schema")
				return &mediaSchema{mediaType: candidate, schemaRaw: schemaRaw}
			}
		}
	}
	if len(content.Keys) == 0 {
		return nil
	}
	first := content.Keys[0]
	entry, err := ParseOrderedObject(content.Values[first])
	if err != nil {
		return nil
	}
	schemaRaw, _ := entry.Get("schema")
	return &mediaSchema{mediaType: first, schemaRaw: schemaRaw}
}

type responseSchema struct {
	statusCode string
	mediaType  string
	schemaRaw  json.RawMessage
}

func selectPrimarySuccessJSON(operation OrderedObject) *responseSchema {
	responsesRaw, ok := operation.Get("responses")
	if !ok {
		return nil
	}
	responses, err := ParseOrderedObject(responsesRaw)
	if err != nil {
		return nil
	}
	if status200Raw, ok := responses.Get("200"); ok {
		status200, err := ParseOrderedObject(status200Raw)
		if err == nil {
			if contentRaw, ok := status200.Get("content"); ok {
				content, err := ParseOrderedObject(contentRaw)
				if err == nil {
					if selected := selectJSONContent(content); selected != nil {
						return &responseSchema{statusCode: "200", mediaType: selected.mediaType, schemaRaw: selected.schemaRaw}
					}
				}
			}
		}
	}
	for _, statusCode := range responses.Keys {
		if statusCode == "200" {
			continue
		}
		if len(statusCode) != 3 || statusCode[0] != '2' {
			continue
		}
		if statusCode[1] < '0' || statusCode[1] > '9' || statusCode[2] < '0' || statusCode[2] > '9' {
			continue
		}
		response, err := ParseOrderedObject(responses.Values[statusCode])
		if err != nil {
			continue
		}
		contentRaw, ok := response.Get("content")
		if !ok {
			continue
		}
		content, err := ParseOrderedObject(contentRaw)
		if err != nil {
			continue
		}
		selected := selectJSONContent(content)
		if selected == nil {
			continue
		}
		return &responseSchema{statusCode: statusCode, mediaType: selected.mediaType, schemaRaw: selected.schemaRaw}
	}
	return nil
}

func renderParameterGroup(typeName string, params []paramSpec, renderer *typeRenderer) string {
	if len(params) == 0 {
		return ""
	}
	lines := make([]string, 0, len(params))
	for _, param := range params {
		desc := ""
		if param.description != "" {
			desc = fmt.Sprintf("  /** %s */\n", SafeComment(param.description))
		}
		optional := "?"
		if param.required {
			optional = ""
		}
		lines = append(lines, fmt.Sprintf("%s  %s%s: %s;", desc, PropertyKey(param.name), optional, renderer.renderSchema(param.schemaRaw)))
	}
	return fmt.Sprintf("export interface %s {\n%s\n}", typeName, strings.Join(lines, "\n"))
}

func renderOperationTypes(ctx OperationContext, renderer *typeRenderer) []string {
	apiName := operationTypeName(ctx)
	operation := ctx.Operation
	description := ctx.Method + " " + ctx.Path
	if descRaw, ok := operation.Get("description"); ok {
		var text string
		if err := json.Unmarshal(descRaw, &text); err == nil && text != "" {
			description = text
		}
	} else if summaryRaw, ok := operation.Get("summary"); ok {
		var text string
		if err := json.Unmarshal(summaryRaw, &text); err == nil && text != "" {
			description = text
		}
	}
	lines := []string{
		fmt.Sprintf("/** %s */", SafeComment(description)),
		fmt.Sprintf("// %s %s", ctx.Method, ctx.Path),
	}
	params := mergeParameters(ctx.PathItem, operation)
	if len(params) > 0 {
		lines = append(lines, "\n// ====== 请求参数 ======")
		for _, spec := range parameterGroupSpecs {
			groupParams := make([]paramSpec, 0)
			for _, param := range params {
				if param.in == spec.inType {
					groupParams = append(groupParams, param)
				}
			}
			groupBlock := renderParameterGroup(apiName+spec.suffix, groupParams, renderer)
			if groupBlock != "" {
				lines = append(lines, "\n// "+spec.label)
				lines = append(lines, groupBlock)
			}
		}
	}
	if selected := selectRequestBodyContent(operation); selected != nil && len(selected.schemaRaw) > 0 {
		lines = append(lines, "\n// ====== 请求体 ======")
		lines = append(lines, "// "+selected.mediaType)
		lines = append(lines, fmt.Sprintf("export type %sRequestBody = %s;", apiName, renderer.renderSchema(selected.schemaRaw)))
	}
	if selected := selectPrimarySuccessJSON(operation); selected != nil && len(selected.schemaRaw) > 0 {
		lines = append(lines, "\n// ====== 返回响应 ======")
		lines = append(lines, fmt.Sprintf("// %s %s", selected.statusCode, selected.mediaType))
		lines = append(lines, fmt.Sprintf("export type %sResponse = %s;", apiName, renderer.renderSchema(selected.schemaRaw)))
	} else {
		lines = append(lines, "\n// 未找到 2xx JSON 响应，未生成 Response 类型。")
	}
	return lines
}

// typeRenderer ports TypeScript TypeRenderer: deterministic $ref emission.
type typeRenderer struct {
	schemas     OrderedObject
	emitted     map[string]bool
	queue       []string
	orderedRefs []string
	refNames    map[string]string
	usedNames   map[string]int
	bodies      map[string]string
}

func newTypeRenderer(schemas OrderedObject) *typeRenderer {
	if schemas.Values == nil {
		schemas.Values = map[string]json.RawMessage{}
	}
	return &typeRenderer{
		schemas:   schemas,
		emitted:   map[string]bool{},
		refNames:  map[string]string{},
		usedNames: map[string]int{},
		bodies:    map[string]string{},
	}
}

func (renderer *typeRenderer) renderSchema(schemaRaw json.RawMessage) string {
	return renderer.typeOf(schemaRaw, 0)
}

func (renderer *typeRenderer) drain() []string {
	for len(renderer.queue) > 0 {
		ref := renderer.queue[0]
		renderer.queue = renderer.queue[1:]
		if renderer.emitted[ref] {
			continue
		}
		renderer.emitRef(ref)
	}
	out := make([]string, 0, len(renderer.orderedRefs))
	for _, ref := range renderer.orderedRefs {
		if body, ok := renderer.bodies[ref]; ok && body != "" {
			out = append(out, body)
		}
	}
	return out
}

func (renderer *typeRenderer) allocateName(ref string) string {
	if existing, ok := renderer.refNames[ref]; ok {
		return existing
	}
	baseName := Sanitize(refName(ref))
	if baseName == "" {
		baseName = "Schema"
	}
	count := renderer.usedNames[baseName]
	nextCount := count + 1
	renderer.usedNames[baseName] = nextCount
	name := baseName
	if count > 0 {
		name = fmt.Sprintf("%s%d", baseName, nextCount)
	}
	renderer.refNames[ref] = name
	return name
}

func (renderer *typeRenderer) enqueueRef(ref string) string {
	name := renderer.allocateName(ref)
	if renderer.emitted[ref] {
		return name
	}
	for _, queued := range renderer.queue {
		if queued == ref {
			return name
		}
	}
	renderer.queue = append(renderer.queue, ref)
	return name
}

func (renderer *typeRenderer) emitRef(ref string) {
	renderer.emitted[ref] = true
	name := renderer.allocateName(ref)
	schemaRaw, ok := schemaFromRef(ref, renderer.schemas)
	if !ok {
		renderer.bodies[ref] = fmt.Sprintf("export type %s = any; // missing schema %s", name, ref)
		renderer.orderedRefs = append(renderer.orderedRefs, ref)
		return
	}
	object, err := ParseOrderedObject(schemaRaw)
	if err != nil {
		renderer.bodies[ref] = fmt.Sprintf("export type %s = any; // missing schema %s", name, ref)
		renderer.orderedRefs = append(renderer.orderedRefs, ref)
		return
	}
	if isObjectSchema(object) {
		renderer.bodies[ref] = renderer.renderInterface(name, object)
	} else {
		renderer.bodies[ref] = fmt.Sprintf("export type %s = %s;", name, renderer.typeOf(schemaRaw, 0))
	}
	renderer.orderedRefs = append(renderer.orderedRefs, ref)
}

func isObjectSchema(object OrderedObject) bool {
	if typeRaw, ok := object.Get("type"); ok {
		var typeName string
		if err := json.Unmarshal(typeRaw, &typeName); err == nil && typeName == "object" {
			return true
		}
	}
	_, hasProps := object.Get("properties")
	return hasProps
}

func (renderer *typeRenderer) renderInterface(name string, object OrderedObject) string {
	propsRaw, _ := object.Get("properties")
	props, _ := ParseOrderedObject(propsRaw)
	required := requiredSet(object)
	lines := make([]string, 0, len(props.Keys))
	for _, key := range props.Keys {
		propRaw := props.Values[key]
		propObject, _ := ParseOrderedObject(propRaw)
		desc := ""
		if descRaw, ok := propObject.Get("description"); ok {
			var description string
			if err := json.Unmarshal(descRaw, &description); err == nil && description != "" {
				desc = fmt.Sprintf("  /** %s */\n", SafeComment(description))
			}
		}
		optional := "?"
		if required[key] {
			optional = ""
		}
		lines = append(lines, fmt.Sprintf("%s  %s%s: %s;", desc, PropertyKey(key), optional, renderer.typeOf(propRaw, 2)))
	}
	return fmt.Sprintf("export interface %s {\n%s\n}", name, strings.Join(lines, "\n"))
}

func requiredSet(object OrderedObject) map[string]bool {
	result := map[string]bool{}
	requiredRaw, ok := object.Get("required")
	if !ok {
		return result
	}
	var required []string
	if err := json.Unmarshal(requiredRaw, &required); err != nil {
		return result
	}
	for _, key := range required {
		result[key] = true
	}
	return result
}

func (renderer *typeRenderer) enumType(object OrderedObject) string {
	enumRaw, ok := object.Get("enum")
	if !ok {
		return ""
	}
	var values []any
	if err := json.Unmarshal(enumRaw, &values); err != nil || len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, jsonLiteral(value))
	}
	return strings.Join(uniqueStrings(parts), " | ")
}

func (renderer *typeRenderer) typeOf(schemaRaw json.RawMessage, indent int) string {
	if len(schemaRaw) == 0 || string(schemaRaw) == "null" {
		return "any"
	}
	object, err := ParseOrderedObject(schemaRaw)
	if err != nil {
		return "any"
	}
	if refRaw, ok := object.Get("$ref"); ok {
		var ref string
		if err := json.Unmarshal(refRaw, &ref); err == nil && ref != "" {
			return renderer.enqueueRef(ref)
		}
	}
	if enumType := renderer.enumType(object); enumType != "" {
		return withNullable(enumType, object)
	}
	if typeRaw, ok := object.Get("type"); ok {
		var typeList []string
		if err := json.Unmarshal(typeRaw, &typeList); err == nil && len(typeList) > 0 {
			parts := make([]string, 0, len(typeList))
			for _, typeName := range typeList {
				parts = append(parts, renderer.typeOf(cloneSchemaWithType(schemaRaw, typeName), indent))
			}
			return strings.Join(uniqueStrings(parts), " | ")
		}
	}
	if oneOfRaw, ok := object.Get("oneOf"); ok {
		return withNullable(renderer.joinSchemas(oneOfRaw, " | ", indent), object)
	}
	if anyOfRaw, ok := object.Get("anyOf"); ok {
		return withNullable(renderer.joinSchemas(anyOfRaw, " | ", indent), object)
	}
	if allOfRaw, ok := object.Get("allOf"); ok {
		return withNullable(renderer.joinSchemas(allOfRaw, " & ", indent), object)
	}
	typeName := ""
	if typeRaw, ok := object.Get("type"); ok {
		_ = json.Unmarshal(typeRaw, &typeName)
	}
	if typeName == "array" || hasKey(object, "items") {
		itemsRaw, _ := object.Get("items")
		return withNullable(fmt.Sprintf("Array<%s>", renderer.typeOf(itemsRaw, indent)), object)
	}
	if typeName == "object" || hasKey(object, "properties") {
		return renderer.renderInlineObject(object, indent)
	}
	switch typeName {
	case "string":
		return withNullable("string", object)
	case "integer", "number":
		return withNullable("number", object)
	case "boolean":
		return withNullable("boolean", object)
	case "null":
		return "null"
	default:
		return withNullable("any", object)
	}
}

func (renderer *typeRenderer) joinSchemas(raw json.RawMessage, sep string, indent int) string {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		return "any"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, renderer.typeOf(item, indent))
	}
	joined := strings.Join(parts, sep)
	if joined == "" {
		return "any"
	}
	return joined
}

func (renderer *typeRenderer) renderInlineObject(object OrderedObject, indent int) string {
	propsRaw, _ := object.Get("properties")
	props, _ := ParseOrderedObject(propsRaw)
	required := requiredSet(object)
	if len(props.Keys) == 0 {
		if additionalRaw, ok := object.Get("additionalProperties"); ok {
			if len(additionalRaw) > 0 && string(additionalRaw) != "true" && string(additionalRaw) != "false" {
				if _, err := ParseOrderedObject(additionalRaw); err == nil {
					return withNullable(fmt.Sprintf("Record<string, %s>", renderer.typeOf(additionalRaw, indent)), object)
				}
			}
		}
		return withNullable("Record<string, any>", object)
	}
	childIndent := indent + 2
	fieldTypeStrings := make([]string, 0, len(props.Keys))
	for _, key := range props.Keys {
		fieldTypeStrings = append(fieldTypeStrings, renderer.typeOf(props.Values[key], childIndent))
	}
	needsMultiline := false
	for index, key := range props.Keys {
		propObject, _ := ParseOrderedObject(props.Values[key])
		if descRaw, ok := propObject.Get("description"); ok {
			var description string
			if err := json.Unmarshal(descRaw, &description); err == nil && description != "" {
				needsMultiline = true
				break
			}
		}
		if strings.Contains(fieldTypeStrings[index], "\n") {
			needsMultiline = true
			break
		}
	}
	if !needsMultiline {
		fields := make([]string, 0, len(props.Keys))
		for index, key := range props.Keys {
			optional := "?"
			if required[key] {
				optional = ""
			}
			fieldType := "any"
			if index < len(fieldTypeStrings) {
				fieldType = fieldTypeStrings[index]
			}
			fields = append(fields, fmt.Sprintf("%s%s: %s", PropertyKey(key), optional, fieldType))
		}
		return withNullable(fmt.Sprintf("{ %s }", strings.Join(fields, "; ")), object)
	}
	fieldPad := strings.Repeat(" ", childIndent)
	closePad := strings.Repeat(" ", indent)
	lines := make([]string, 0, len(props.Keys))
	for index, key := range props.Keys {
		propObject, _ := ParseOrderedObject(props.Values[key])
		desc := ""
		if descRaw, ok := propObject.Get("description"); ok {
			var description string
			if err := json.Unmarshal(descRaw, &description); err == nil && description != "" {
				desc = fmt.Sprintf("%s/** %s */\n", fieldPad, SafeComment(description))
			}
		}
		optional := "?"
		if required[key] {
			optional = ""
		}
		fieldType := "any"
		if index < len(fieldTypeStrings) {
			fieldType = fieldTypeStrings[index]
		}
		lines = append(lines, fmt.Sprintf("%s%s%s%s: %s;", desc, fieldPad, PropertyKey(key), optional, fieldType))
	}
	return withNullable(fmt.Sprintf("{\n%s\n%s}", strings.Join(lines, "\n"), closePad), object)
}

func refName(ref string) string {
	parts := strings.Split(ref, "/")
	last := ""
	if len(parts) > 0 {
		last = parts[len(parts)-1]
	}
	decoded, err := url.PathUnescape(last)
	if err != nil {
		return last
	}
	return decoded
}

func schemaFromRef(ref string, schemas OrderedObject) (json.RawMessage, bool) {
	if !strings.HasPrefix(ref, "#/components/schemas/") {
		return nil, false
	}
	return schemas.Get(refName(ref))
}

func jsonLiteral(value any) string {
	if value == nil {
		return "null"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "undefined"
	}
	return string(encoded)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func withNullable(typeName string, object OrderedObject) string {
	nullableRaw, ok := object.Get("nullable")
	if !ok {
		return typeName
	}
	var nullable bool
	if err := json.Unmarshal(nullableRaw, &nullable); err != nil || !nullable {
		return typeName
	}
	for _, part := range strings.Split(typeName, "|") {
		if strings.TrimSpace(part) == "null" {
			return typeName
		}
	}
	return typeName + " | null"
}

func hasKey(object OrderedObject, key string) bool {
	_, ok := object.Get(key)
	return ok
}

func cloneSchemaWithType(schemaRaw json.RawMessage, typeName string) json.RawMessage {
	object, err := ParseOrderedObject(schemaRaw)
	if err != nil {
		return schemaRaw
	}
	encoded, err := json.Marshal(typeName)
	if err != nil {
		return schemaRaw
	}
	var builder strings.Builder
	builder.WriteByte('{')
	first := true
	hasType := false
	for _, key := range object.Keys {
		if !first {
			builder.WriteByte(',')
		}
		first = false
		keyJSON, _ := json.Marshal(key)
		builder.Write(keyJSON)
		builder.WriteByte(':')
		if key == "type" {
			builder.Write(encoded)
			hasType = true
		} else {
			builder.Write(object.Values[key])
		}
	}
	if !hasType {
		if !first {
			builder.WriteByte(',')
		}
		builder.WriteString(`"type":`)
		builder.Write(encoded)
	}
	builder.WriteByte('}')
	return json.RawMessage(builder.String())
}
