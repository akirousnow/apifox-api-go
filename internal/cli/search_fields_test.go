package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akirousnow/apifox-api-go/internal/binding"
	"github.com/akirousnow/apifox-api-go/internal/cli"
	"github.com/akirousnow/apifox-api-go/internal/snapshot"
)

func setupSearchFieldsWorkspace(t *testing.T) (homeDir string, workspace string, authKey string) {
	t.Helper()
	homeDir = t.TempDir()
	workspace = filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	authKey = "search-fields-fixture-token"
	_, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ProjectID: "proj-search-fields-1",
		AuthKey:   authKey,
		ModuleIDs: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	return homeDir, workspace, authKey
}

// writeSearchFieldsCache writes a snapshot with query/path params, nested body fields,
// and primary 2xx JSON response fields (plus a header param that must NOT be indexed).
func writeSearchFieldsCache(t *testing.T, workspace, projectID, authKey string, moduleID *int, timestamp int64) string {
	t.Helper()
	identity := snapshot.CacheIdentity{
		ProjectID:       projectID,
		AuthFingerprint: binding.AuthFingerprint(authKey),
		ModuleID:        moduleID,
	}
	cachePath := snapshot.GetOpenAPICachePathForWorkspace(workspace, identity)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{
		"timestamp":        timestamp,
		"projectId":        projectID,
		"authFingerprint":  binding.AuthFingerprint(authKey),
		"exportApiVersion": snapshot.OpenAPIExportAPIVersion,
		"exportParamsHash": snapshot.ExportParamsHash(),
		"data": map[string]any{
			"openapi": "3.1.0",
			"paths": map[string]any{
				"/users/{userId}": map[string]any{
					"get": map[string]any{
						"summary":     "获取用户",
						"operationId": "getUser",
						"tags":        []any{"users"},
						"parameters": []any{
							map[string]any{
								"name":        "userId",
								"in":          "path",
								"description": "用户ID",
								"required":    true,
								"schema":      map[string]any{"type": "string"},
							},
							map[string]any{
								"name":        "phone",
								"in":          "query",
								"description": "手机号",
								"schema":      map[string]any{"type": "string"},
							},
							map[string]any{
								"name":        "X-Trace-Id",
								"in":          "header",
								"description": "trace header must not be indexed",
								"schema":      map[string]any{"type": "string"},
							},
						},
						"requestBody": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"user": map[string]any{
												"type": "object",
												"properties": map[string]any{
													"email": map[string]any{
														"type":        "string",
														"description": "邮箱地址",
													},
													"profile": map[string]any{
														"type": "object",
														"properties": map[string]any{
															"nickname": map[string]any{
																"type":        "string",
																"description": "昵称",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						"responses": map[string]any{
							"200": map[string]any{
								"description": "ok",
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"id": map[string]any{
													"type":        "string",
													"description": "响应ID",
												},
												"balance": map[string]any{
													"type":        "number",
													"description": "账户余额",
												},
											},
										},
									},
								},
							},
						},
					},
					"post": map[string]any{
						"summary":     "创建用户",
						"operationId": "createUser",
						"tags":        []any{"users", "write"},
						"requestBody": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"username": map[string]any{
												"type":        "string",
												"description": "用户名",
											},
										},
									},
								},
							},
						},
						"responses": map[string]any{
							"201": map[string]any{
								"description": "created",
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"createdAt": map[string]any{
													"type":        "string",
													"description": "创建时间",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				"/orders": map[string]any{
					"get": map[string]any{
						"summary":     "订单列表",
						"operationId": "listOrders",
						"tags":        []any{"orders"},
						"parameters": []any{
							map[string]any{
								"name":        "orderNo",
								"in":          "query",
								"description": "订单号",
								"schema":      map[string]any{"type": "string"},
							},
						},
						"responses": map[string]any{
							"200": map[string]any{
								"description": "ok",
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"items": map[string]any{
													"type": "array",
													"items": map[string]any{
														"type": "object",
														"properties": map[string]any{
															"sku": map[string]any{
																"type":        "string",
																"description": "商品SKU",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if moduleID != nil {
		entry["moduleId"] = *moduleID
	}
	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return cachePath
}

func TestSearchFieldsCLIEmptyReject(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	var stdout, stderr bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout, Err: &stderr},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields"})
	if err == nil {
		t.Fatal("expected empty search-fields rejection")
	}
	if !strings.Contains(err.Error(), "请提供 keywords") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchFieldsCLIMethodOnlyReject(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--method", "GET"})
	if err == nil {
		t.Fatal("expected method-only search-fields rejection")
	}
	if !strings.Contains(err.Error(), "keywords") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchFieldsCLIE2EOffline(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	var stdout, stderr bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout, Err: &stderr},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "phone"})
	if err != nil {
		t.Fatalf("execute: %v\nstderr=%s\nstdout=%s", err, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "共找到") {
		t.Fatalf("missing total line: %s", out)
	}
	if !strings.Contains(out, "命中字段") {
		t.Fatalf("missing 命中字段 column: %s", out)
	}
	if !strings.Contains(out, "query.phone") {
		t.Fatalf("missing phone hit: %s", out)
	}
	if !strings.Contains(out, "手机号") {
		t.Fatalf("missing phone description: %s", out)
	}
	if !strings.Contains(out, "| GET | `/users/{userId}` |") {
		t.Fatalf("missing user row: %s", out)
	}
}

func TestSearchFieldsCLIBodyAndResponse(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	// Body nested field
	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "email"})
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "body.user.email") {
		t.Fatalf("missing body hit: %s", out)
	}

	// Response field description
	stdout.Reset()
	err = cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "响应ID"})
	if err != nil {
		t.Fatal(err)
	}
	out = stdout.String()
	if !strings.Contains(out, "response.") {
		t.Fatalf("missing response hit: %s", out)
	}
	if !strings.Contains(out, "命中字段") {
		t.Fatalf("missing 命中字段: %s", out)
	}
}

func TestSearchFieldsCLINoHeaderIndexing(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	// Use a keyword unique to the header description so substring hits on userId/id
	// do not mask the product rule (headers are not indexed).
	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--json", "trace header must not be indexed"})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if int(doc["total"].(float64)) != 0 {
		t.Fatalf("header params must not be indexed, total=%v out=%s", doc["total"], stdout.String())
	}
}

func TestSearchFieldsCLIMethodFilter(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--method", "POST", "email"})
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	// email only exists on GET body; POST filter should yield zero field matches for email
	if !strings.Contains(out, "未找到字段匹配") && !strings.Contains(out, "共找到 0") {
		// zero matches may render as 未找到...
		if strings.Contains(out, "| GET |") {
			t.Fatalf("GET should be filtered: %s", out)
		}
	}
	if strings.Contains(out, "body.user.email") {
		t.Fatalf("POST filter should exclude GET email hit: %s", out)
	}

	stdout.Reset()
	err = cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--method", "POST", "username"})
	if err != nil {
		t.Fatal(err)
	}
	out = stdout.String()
	if !strings.Contains(out, "body.username") {
		t.Fatalf("expected POST body hit: %s", out)
	}
	if strings.Contains(out, "| GET |") {
		t.Fatalf("GET should be filtered: %s", out)
	}
}

func TestSearchFieldsCLIJSON(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--json", "phone"})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("json parse: %v\n%s", err, stdout.String())
	}
	for _, key := range []string{"total", "showing", "truncated", "limit", "module", "stale", "items"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("missing %s in %s", key, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "找到多个候选") || strings.Contains(stdout.String(), "命中字段") {
		t.Fatalf("JSON must not include markdown/strategy prose: %s", stdout.String())
	}
	items := doc["items"].([]any)
	if len(items) < 1 {
		t.Fatal("expected items")
	}
	item := items[0].(map[string]any)
	for _, key := range []string{"method", "path", "summary", "tags", "operationId", "matches"} {
		if _, ok := item[key]; !ok {
			t.Fatalf("missing item.%s", key)
		}
	}
	matches := item["matches"].([]any)
	if len(matches) < 1 {
		t.Fatal("expected matches[]")
	}
	match := matches[0].(map[string]any)
	for _, key := range []string{"kind", "location", "name", "display"} {
		if _, ok := match[key]; !ok {
			t.Fatalf("missing match.%s", key)
		}
	}
}

func TestSearchFieldsCLIModeAndLimit(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--json", "--mode", "and", "phone", "email"})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if int(doc["total"].(float64)) != 1 {
		t.Fatalf("and mode total=%v", doc["total"])
	}

	stdout.Reset()
	err = cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--json", "--limit", "1", "user"})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if int(doc["limit"].(float64)) != 1 {
		t.Fatalf("limit=%v", doc["limit"])
	}
	if int(doc["showing"].(float64)) > 1 {
		t.Fatalf("showing=%v", doc["showing"])
	}
}

func TestSearchFieldsCLIInvalidMethod(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--method", "FETCH", "phone"})
	if err == nil {
		t.Fatal("expected invalid method error")
	}
	if !strings.Contains(err.Error(), "必须是合法 HTTP 方法") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchFieldsCLIMissingCache(t *testing.T) {
	homeDir, workspace, _ := setupSearchFieldsWorkspace(t)
	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "phone"})
	if err == nil {
		t.Fatal("expected missing cache / refresh failure error")
	}
	message := err.Error()
	if !strings.Contains(message, "未找到") &&
		!strings.Contains(message, "快照缓存") &&
		!strings.Contains(message, "刷新 OpenAPI 快照失败") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchFieldsCLIZeroMatches(t *testing.T) {
	homeDir, workspace, authKey := setupSearchFieldsWorkspace(t)
	writeSearchFieldsCache(t, workspace, "proj-search-fields-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search-fields", "--json", "zzznomatchfield"})
	if err != nil {
		t.Fatalf("zero matches should succeed: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if int(doc["total"].(float64)) != 0 {
		t.Fatalf("total=%v", doc["total"])
	}
}
