package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func boolPtr(value bool) *bool {
	return &value
}

func TestLoadModuleSnapshotFreshHitNoNetwork(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{ProjectID: "proj1", AuthFingerprint: "fp1"}
	nowMs := int64(1_000_000_000_000)
	writeFixtureEnvelope(t, workspaceDir, identity, nowMs-1000, map[string]any{
		"openapi": "3.1.0",
		"paths":   map[string]any{"/a": map[string]any{}},
	})

	var fetchCalls atomic.Int32
	result, err := LoadModuleSnapshot(LoadOptions{
		WorkspaceDir:    workspaceDir,
		ProjectID:       identity.ProjectID,
		AuthFingerprint: identity.AuthFingerprint,
		AuthKey:         "secret",
		NowMs:           nowMs,
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			fetchCalls.Add(1)
			return nil, errors.New("should not be called")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stale || result.Refreshed {
		t.Fatalf("unexpected result: %+v", result)
	}
	if fetchCalls.Load() != 0 {
		t.Fatal("fresh hit must not perform network I/O")
	}
}

func TestLoadModuleSnapshotStaleRefreshSuccess(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{ProjectID: "proj1", AuthFingerprint: "fp1"}
	nowMs := int64(1_000_000_000_000)
	writeFixtureEnvelope(t, workspaceDir, identity, nowMs-DefaultCacheTTLMs-1, map[string]any{
		"openapi": "3.1.0",
		"paths":   map[string]any{"/old": map[string]any{}},
	})

	freshData := json.RawMessage(`{"openapi":"3.1.0","paths":{"/new":{}}}`)
	result, err := LoadModuleSnapshot(LoadOptions{
		WorkspaceDir:    workspaceDir,
		ProjectID:       identity.ProjectID,
		AuthFingerprint: identity.AuthFingerprint,
		AuthKey:         "secret",
		NowMs:           nowMs,
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			if authKey != "secret" {
				t.Fatalf("authKey leaked or wrong: %q", authKey)
			}
			return freshData, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stale || !result.Refreshed {
		t.Fatalf("expected refreshed fresh result: %+v", result)
	}
	if !strings.Contains(string(result.Data), "/new") {
		t.Fatalf("data = %s", string(result.Data))
	}

	// Disk must contain the new envelope.
	envelope, err := ReadEnvelope(result.CachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envelope.Data), "/new") {
		t.Fatalf("disk data = %s", string(envelope.Data))
	}
	if envelope.Timestamp != nowMs {
		t.Fatalf("timestamp = %d", envelope.Timestamp)
	}
}

func TestLoadModuleSnapshotStaleTransientFallback(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{ProjectID: "proj1", AuthFingerprint: "fp1"}
	nowMs := int64(1_000_000_000_000)
	writeFixtureEnvelope(t, workspaceDir, identity, nowMs-DefaultCacheTTLMs-1, map[string]any{
		"openapi": "3.1.0",
		"paths":   map[string]any{"/stale": map[string]any{}},
	})

	result, err := LoadModuleSnapshot(LoadOptions{
		WorkspaceDir:    workspaceDir,
		ProjectID:       identity.ProjectID,
		AuthFingerprint: identity.AuthFingerprint,
		AuthKey:         "secret",
		NowMs:           nowMs,
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			return nil, fmt.Errorf("请求 Apifox OpenAPI 导出失败 (HTTP 503): boom")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stale {
		t.Fatal("expected stale fallback")
	}
	if !strings.Contains(result.Warning, "已使用本地过期缓存") {
		t.Fatalf("warning = %s", result.Warning)
	}
	if !strings.Contains(string(result.Data), "/stale") {
		t.Fatalf("data = %s", string(result.Data))
	}
}

func TestLoadModuleSnapshotForceRefreshHardFail(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{ProjectID: "proj1", AuthFingerprint: "fp1"}
	nowMs := int64(1_000_000_000_000)
	writeFixtureEnvelope(t, workspaceDir, identity, nowMs-DefaultCacheTTLMs-1, map[string]any{
		"openapi": "3.1.0",
		"paths":   map[string]any{},
	})

	_, err := LoadModuleSnapshot(LoadOptions{
		WorkspaceDir:      workspaceDir,
		ProjectID:         identity.ProjectID,
		AuthFingerprint:   identity.AuthFingerprint,
		AuthKey:           "secret",
		NowMs:             nowMs,
		ForceRefresh:      true,
		AllowStaleOnError: boolPtr(false),
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			return nil, fmt.Errorf("请求 Apifox OpenAPI 导出失败 (HTTP 503)")
		},
	})
	if err == nil {
		t.Fatal("expected hard error")
	}
	if !strings.Contains(err.Error(), "刷新 OpenAPI 快照失败") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadAllModuleSnapshotsPartialFailureKeepsCompleted(t *testing.T) {
	workspaceDir := t.TempDir()
	nowMs := int64(1_000_000_000_000)
	moduleA := 10
	moduleB := 20
	fp := "fp1"
	projectID := "proj1"

	// Pre-seed nothing; first module succeeds, second fails.
	var calls atomic.Int32
	results, err := LoadAllModuleSnapshots(workspaceDir, projectID, "secret", fp, []int{moduleA, moduleB}, LoadOptions{
		NowMs:             nowMs,
		ForceRefresh:      true,
		AllowStaleOnError: boolPtr(false),
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			calls.Add(1)
			if moduleID != nil && *moduleID == moduleA {
				return json.RawMessage(`{"openapi":"3.1.0","paths":{"/a":{}}}`), nil
			}
			return nil, fmt.Errorf("请求 Apifox OpenAPI 导出失败 (HTTP 500)")
		},
	})
	if err == nil {
		t.Fatal("expected partial failure")
	}
	if !strings.Contains(err.Error(), "m20") && !strings.Contains(err.Error(), "moduleId=m20") && !strings.Contains(err.Error(), "m20") {
		// error uses ModuleIDFilePart which is m20
		if !strings.Contains(err.Error(), "m20") {
			t.Fatalf("error should name failing module: %v", err)
		}
	}
	if len(results) != 1 {
		t.Fatalf("completed results = %d, want 1", len(results))
	}
	// Module A cache must remain valid on disk.
	identityA := CacheIdentity{ProjectID: projectID, AuthFingerprint: fp, ModuleID: &moduleA}
	pathA := GetOpenAPICachePathForWorkspace(workspaceDir, identityA)
	if _, statErr := os.Stat(pathA); statErr != nil {
		t.Fatalf("module A cache missing: %v", statErr)
	}
}

func TestWriteEnvelopeAtomicLeavesCompleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.openapi.json")
	// Seed previous complete file.
	previous := Envelope{
		Timestamp:        1,
		ProjectID:        "p",
		AuthFingerprint:  "fp",
		ExportAPIVersion: OpenAPIExportAPIVersion,
		ExportParamsHash: ExportParamsHash(),
		Data:             json.RawMessage(`{"openapi":"3.1.0","paths":{"/old":{}}}`),
	}
	if err := WriteEnvelope(path, previous); err != nil {
		t.Fatal(err)
	}

	next := previous
	next.Timestamp = 2
	next.Data = json.RawMessage(`{"openapi":"3.1.0","paths":{"/new":{}}}`)
	if err := WriteEnvelope(path, next); err != nil {
		t.Fatal(err)
	}

	got, err := ReadEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Timestamp != 2 {
		t.Fatalf("timestamp = %d", got.Timestamp)
	}
	if !json.Valid(mustRead(t, path)) {
		t.Fatal("cache file must be valid complete JSON")
	}
	// No leftover temp files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".openapi-") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("leftover temp file: %s", entry.Name())
		}
	}
}

func TestLoadModuleSnapshotCancelDoesNotLeavePartial(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{ProjectID: "proj1", AuthFingerprint: "fp1"}
	nowMs := int64(1_000_000_000_000)
	// No prior cache.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := LoadModuleSnapshot(LoadOptions{
		WorkspaceDir:      workspaceDir,
		ProjectID:         identity.ProjectID,
		AuthFingerprint:   identity.AuthFingerprint,
		AuthKey:           "secret",
		NowMs:             nowMs,
		ForceRefresh:      true,
		AllowStaleOnError: boolPtr(false),
		Context:           ctx,
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			return nil, ctx.Err()
		},
	})
	if err == nil {
		t.Fatal("expected cancel error")
	}
	cachePath := GetOpenAPICachePathForWorkspace(workspaceDir, identity)
	if _, statErr := os.Stat(cachePath); !os.IsNotExist(statErr) {
		t.Fatalf("partial cache should not exist: %v", statErr)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLoadModuleSnapshotOfflineStillWorks(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{ProjectID: "proj1", AuthFingerprint: "fp1"}
	nowMs := int64(1_000_000_000_000)
	writeFixtureEnvelope(t, workspaceDir, identity, nowMs-1000, map[string]any{
		"openapi": "3.1.0",
		"paths":   map[string]any{},
	})
	result, err := LoadModuleSnapshotOffline(LoadOptions{
		WorkspaceDir:    workspaceDir,
		ProjectID:       identity.ProjectID,
		AuthFingerprint: identity.AuthFingerprint,
		NowMs:           nowMs,
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			t.Fatal("offline must not fetch")
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stale {
		t.Fatal("expected fresh offline")
	}
	_ = time.Now()
}
