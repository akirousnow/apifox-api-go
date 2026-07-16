package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultHTTPTimeout is the total request timeout for export-openapi.
	DefaultHTTPTimeout = 60 * time.Second
	// MaxExportBodyBytes is the maximum successful response body size (50 MiB).
	MaxExportBodyBytes = 50 << 20
	// ExportBaseURL is the Apifox export-openapi endpoint template.
	ExportBaseURL = "https://api.apifox.com/v1/projects/%s/export-openapi?locale=zh-CN"
)

// ExportBaseBody is the module-independent export request body (TS parity).
var ExportBaseBody = map[string]any{
	"scope": map[string]any{
		"type": "ALL",
	},
	"options": map[string]any{
		"includeApifoxExtensionProperties": false,
		"addFoldersToTags":                 false,
	},
	"oasVersion":   "3.1",
	"exportFormat": "JSON",
}

// FetchFunc fetches an OpenAPI document for tests or production.
type FetchFunc func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error)

// NewDefaultHTTPClient returns an HTTP client with timeout and redirect auth stripping.
func NewDefaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: DefaultHTTPTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			original := via[0]
			if original.URL.Host != req.URL.Host {
				req.Header.Del("Authorization")
			}
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
}

// FetchOpenAPIExport calls Apifox export-openapi and returns the OpenAPI JSON object.
func FetchOpenAPIExport(ctx context.Context, client *http.Client, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
	if client == nil {
		client = NewDefaultHTTPClient()
	}
	endpoint := fmt.Sprintf(ExportBaseURL, url.PathEscape(projectID))
	bodyMap := map[string]any{
		"scope":        ExportBaseBody["scope"],
		"options":      ExportBaseBody["options"],
		"oasVersion":   ExportBaseBody["oasVersion"],
		"exportFormat": ExportBaseBody["exportFormat"],
	}
	if moduleID != nil {
		bodyMap["moduleId"] = *moduleID
	}
	payload, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Apifox-Api-Version", OpenAPIExportAPIVersion)
	req.Header.Set("Authorization", "Bearer "+authKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, MaxExportBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxExportBodyBytes {
		return nil, fmt.Errorf("OpenAPI 导出响应超过 %d 字节上限", MaxExportBodyBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(data)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		if strings.TrimSpace(snippet) != "" {
			return nil, fmt.Errorf("请求 Apifox OpenAPI 导出失败 (HTTP %d): %s", resp.StatusCode, snippet)
		}
		return nil, fmt.Errorf("请求 Apifox OpenAPI 导出失败 (HTTP %d)", resp.StatusCode)
	}

	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, fmt.Errorf("Apifox OpenAPI 导出响应不是有效的 OpenAPI JSON，缺少 paths。")
	}
	object, ok := generic.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Apifox OpenAPI 导出响应不是有效的 OpenAPI JSON，缺少 paths。")
	}
	if _, ok := object["paths"]; !ok {
		return nil, fmt.Errorf("Apifox OpenAPI 导出响应不是有效的 OpenAPI JSON，缺少 paths。")
	}
	return json.RawMessage(data), nil
}

// IsTransientExportError reports whether a failure should allow stale fallback for search/get.
// Auth/4xx and validation errors are not transient.
func IsTransientExportError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	if strings.Contains(message, "HTTP 4") {
		return false
	}
	if strings.Contains(message, "缺少 paths") {
		return false
	}
	if strings.Contains(message, "超过") && strings.Contains(message, "字节") {
		return false
	}
	// timeout, connection, 5xx, cancel after partial network, etc.
	return true
}
