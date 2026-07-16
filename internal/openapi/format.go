package openapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatSearchResults renders the Markdown table + selection guidance (TS parity).
func FormatSearchResults(result SearchResultWindow, staleWarning string) string {
	lines := make([]string, 0, 16+len(result.Items))
	if staleWarning != "" {
		lines = append(lines, "> 警告: "+staleWarning, "")
	}

	if result.Total == 0 {
		lines = append(lines, "未找到符合条件的接口。可以换一个接口名、业务关键词、路径片段或 method 再搜索。")
		return strings.Join(lines, "\n")
	}

	summary := fmt.Sprintf("共找到 %d 个接口，当前展示 %d 个。", result.Total, result.Showing)
	if result.Truncated {
		summary += "结果已截断，请收窄关键词或提高 limit（最大 50）。"
	}
	lines = append(lines, summary, "")
	lines = append(lines, "| 方法 | 路径 | 接口名称 | Tags |")
	lines = append(lines, "|---|---|---|---|")
	for _, endpoint := range result.Items {
		name := endpoint.Summary
		if name == "" {
			name = endpoint.Title
		}
		tags := strings.Join(endpoint.Tags, ", ")
		if tags == "" {
			tags = "-"
		}
		lines = append(lines, fmt.Sprintf("| %s | `%s` | %s | %s |", endpoint.Method, endpoint.Path, name, tags))
	}
	lines = append(lines, "")
	if result.Truncated {
		lines = append(lines, "当前结果已截断，请先收窄搜索范围；不要基于截断列表直接调用 `get`。")
	} else if result.Total == 1 {
		lines = append(lines, "已找到唯一接口。只有用户明确要求生成 TypeScript 类型或入参返回值类型时，才使用该 `method + path` 调用 `get`。")
	} else {
		lines = append(lines, "找到多个候选接口。请先让用户选择一个具体接口；不要直接调用 `get`。")
	}
	return strings.Join(lines, "\n")
}

// SearchJSONDocument is the stable Agent-facing search --json schema (v0.1).
type SearchJSONDocument struct {
	Total     int              `json:"total"`
	Showing   int              `json:"showing"`
	Truncated bool             `json:"truncated"`
	Limit     int              `json:"limit"`
	Module    *int             `json:"module"`
	Stale     bool             `json:"stale"`
	Items     []SearchJSONItem `json:"items"`
}

// SearchJSONItem is one Operation Key candidate in JSON mode.
type SearchJSONItem struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Summary     string   `json:"summary"`
	Tags        []string `json:"tags"`
	OperationID string   `json:"operationId"`
}

// FormatSearchJSON emits a single JSON object with no strategy prose.
func FormatSearchJSON(result SearchResultWindow, moduleID *int, stale bool) ([]byte, error) {
	items := make([]SearchJSONItem, 0, len(result.Items))
	for _, endpoint := range result.Items {
		tags := endpoint.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, SearchJSONItem{
			Method:      endpoint.Method,
			Path:        endpoint.Path,
			Summary:     endpoint.Summary,
			Tags:        tags,
			OperationID: endpoint.OperationID,
		})
	}
	doc := SearchJSONDocument{
		Total:     result.Total,
		Showing:   result.Showing,
		Truncated: result.Truncated,
		Limit:     result.Limit,
		Module:    moduleID,
		Stale:     stale,
		Items:     items,
	}
	return json.Marshal(doc)
}

// FormatFieldSearchResults renders the Markdown table with 命中字段 column.
func FormatFieldSearchResults(result FieldSearchResultWindow, staleWarning string) string {
	lines := make([]string, 0, 16+len(result.Items))
	if staleWarning != "" {
		lines = append(lines, "> 警告: "+staleWarning, "")
	}

	if result.Total == 0 {
		lines = append(lines, "未找到字段匹配的接口。可以换字段名、描述关键词或 method 再搜索。")
		return strings.Join(lines, "\n")
	}

	summary := fmt.Sprintf("共找到 %d 个接口，当前展示 %d 个。", result.Total, result.Showing)
	if result.Truncated {
		summary += "结果已截断，请收窄关键词或提高 limit（最大 50）。"
	}
	lines = append(lines, summary, "")
	lines = append(lines, "| 方法 | 路径 | 接口名称 | 命中字段 |")
	lines = append(lines, "|---|---|---|---|")
	for _, item := range result.Items {
		name := item.Endpoint.Summary
		if name == "" {
			name = item.Endpoint.Title
		}
		hits := FormatMatchedFields(item.Matches)
		lines = append(lines, fmt.Sprintf("| %s | `%s` | %s | %s |", item.Endpoint.Method, item.Endpoint.Path, name, hits))
	}
	lines = append(lines, "")
	if result.Truncated {
		lines = append(lines, "当前结果已截断，请先收窄搜索范围；不要基于截断列表直接调用 `get`。")
	} else if result.Total == 1 {
		lines = append(lines, "已找到唯一接口。只有用户明确要求生成 TypeScript 类型或入参返回值类型时，才使用该 `method + path` 调用 `get`。")
	} else {
		lines = append(lines, "找到多个候选接口。请先让用户选择一个具体接口；不要直接调用 `get`。")
	}
	return strings.Join(lines, "\n")
}

// FieldSearchJSONDocument is the Agent-facing search-fields --json schema.
type FieldSearchJSONDocument struct {
	Total     int                   `json:"total"`
	Showing   int                   `json:"showing"`
	Truncated bool                  `json:"truncated"`
	Limit     int                   `json:"limit"`
	Module    *int                  `json:"module"`
	Stale     bool                  `json:"stale"`
	Items     []FieldSearchJSONItem `json:"items"`
}

// FieldSearchJSONItem is one Operation Key candidate with field hits in JSON mode.
type FieldSearchJSONItem struct {
	Method      string       `json:"method"`
	Path        string       `json:"path"`
	Summary     string       `json:"summary"`
	Tags        []string     `json:"tags"`
	OperationID string       `json:"operationId"`
	Matches     []FieldMatch `json:"matches"`
}

// FormatFieldSearchJSON emits a single JSON object with no strategy prose.
func FormatFieldSearchJSON(result FieldSearchResultWindow, moduleID *int, stale bool) ([]byte, error) {
	items := make([]FieldSearchJSONItem, 0, len(result.Items))
	for _, item := range result.Items {
		tags := item.Endpoint.Tags
		if tags == nil {
			tags = []string{}
		}
		matches := item.Matches
		if matches == nil {
			matches = []FieldMatch{}
		}
		items = append(items, FieldSearchJSONItem{
			Method:      item.Endpoint.Method,
			Path:        item.Endpoint.Path,
			Summary:     item.Endpoint.Summary,
			Tags:        tags,
			OperationID: item.Endpoint.OperationID,
			Matches:     matches,
		})
	}
	doc := FieldSearchJSONDocument{
		Total:     result.Total,
		Showing:   result.Showing,
		Truncated: result.Truncated,
		Limit:     result.Limit,
		Module:    moduleID,
		Stale:     stale,
		Items:     items,
	}
	return json.Marshal(doc)
}
