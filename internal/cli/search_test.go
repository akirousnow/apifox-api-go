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

	"apifox-api/go-version/internal/binding"
	"apifox-api/go-version/internal/cli"
	"apifox-api/go-version/internal/snapshot"
)

func setupSearchWorkspace(t *testing.T) (homeDir string, workspace string, authKey string) {
	t.Helper()
	homeDir = t.TempDir()
	workspace = filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	authKey = "search-fixture-token"
	_, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ProjectID: "proj-search-1",
		AuthKey:   authKey,
		ModuleIDs: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	return homeDir, workspace, authKey
}

func writeSearchCache(t *testing.T, workspace, projectID, authKey string, moduleID *int, timestamp int64) string {
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
				"/pets": map[string]any{
					"get": map[string]any{
						"summary":     "List pets",
						"operationId": "listPets",
						"tags":        []any{"pets"},
						"description": "All pets in the store",
					},
					"post": map[string]any{
						"summary":     "Create pet",
						"operationId": "createPet",
						"tags":        []any{"pets", "write"},
					},
				},
				"/users/login": map[string]any{
					"post": map[string]any{
						"summary":     "用户登录",
						"operationId": "loginUser",
						"tags":        []any{"auth"},
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

func TestSearchCLIEmptyReject(t *testing.T) {
	homeDir, workspace, authKey := setupSearchWorkspace(t)
	writeSearchCache(t, workspace, "proj-search-1", authKey, nil, time.Now().UnixMilli())

	var stdout, stderr bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout, Err: &stderr},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search"})
	if err == nil {
		t.Fatal("expected empty search rejection")
	}
	if !strings.Contains(err.Error(), "请提供 keywords 或 --method") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchCLIE2EOffline(t *testing.T) {
	homeDir, workspace, authKey := setupSearchWorkspace(t)
	writeSearchCache(t, workspace, "proj-search-1", authKey, nil, time.Now().UnixMilli())

	var stdout, stderr bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout, Err: &stderr},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search", "pets"})
	if err != nil {
		t.Fatalf("execute: %v\nstderr=%s\nstdout=%s", err, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "共找到") {
		t.Fatalf("missing total line: %s", out)
	}
	if !strings.Contains(out, "| GET | `/pets` | List pets |") {
		t.Fatalf("missing pets row: %s", out)
	}
	if !strings.Contains(out, "| POST | `/pets` | Create pet |") {
		t.Fatalf("missing create row: %s", out)
	}
	if !strings.Contains(out, "找到多个候选接口") {
		t.Fatalf("missing guidance: %s", out)
	}
}

func TestSearchCLIMethodOnly(t *testing.T) {
	homeDir, workspace, authKey := setupSearchWorkspace(t)
	writeSearchCache(t, workspace, "proj-search-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search", "--method", "POST"})
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "POST") {
		t.Fatalf("expected POST rows: %s", out)
	}
	if strings.Contains(out, "| GET |") {
		t.Fatalf("GET should be filtered out: %s", out)
	}
}

func TestSearchCLIMissingCache(t *testing.T) {
	homeDir, workspace, _ := setupSearchWorkspace(t)
	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search", "pets"})
	if err == nil {
		t.Fatal("expected missing cache / refresh failure error")
	}
	// Online path attempts remote export when cache is absent; without a valid token
	// that surfaces as a hard refresh failure. Offline-only still reports 未找到.
	message := err.Error()
	if !strings.Contains(message, "未找到") &&
		!strings.Contains(message, "快照缓存") &&
		!strings.Contains(message, "刷新 OpenAPI 快照失败") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchCLIJSON(t *testing.T) {
	homeDir, workspace, authKey := setupSearchWorkspace(t)
	writeSearchCache(t, workspace, "proj-search-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search", "--json", "pets"})
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
	if strings.Contains(stdout.String(), "找到多个候选") {
		t.Fatalf("JSON must not include strategy prose: %s", stdout.String())
	}
	items := doc["items"].([]any)
	if len(items) < 1 {
		t.Fatal("expected items")
	}
	item := items[0].(map[string]any)
	for _, key := range []string{"method", "path", "summary", "tags", "operationId"} {
		if _, ok := item[key]; !ok {
			t.Fatalf("missing item.%s", key)
		}
	}
}

func TestSearchCLIInvalidMethod(t *testing.T) {
	homeDir, workspace, authKey := setupSearchWorkspace(t)
	writeSearchCache(t, workspace, "proj-search-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search", "--method", "FETCH", "pets"})
	if err == nil {
		t.Fatal("expected invalid method error")
	}
	if !strings.Contains(err.Error(), "必须是合法 HTTP 方法") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchCLIValidMethodZeroMatches(t *testing.T) {
	homeDir, workspace, authKey := setupSearchWorkspace(t)
	writeSearchCache(t, workspace, "proj-search-1", authKey, nil, time.Now().UnixMilli())

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &stdout},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"search", "--json", "--method", "TRACE"})
	if err != nil {
		t.Fatalf("valid method zero matches should succeed: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if int(doc["total"].(float64)) != 0 {
		t.Fatalf("total=%v", doc["total"])
	}
}

