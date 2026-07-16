package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFixtureEnvelope(t *testing.T, workspaceDir string, identity CacheIdentity, timestamp int64, data map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	envelope := Envelope{
		Timestamp:        timestamp,
		ProjectID:        identity.ProjectID,
		ModuleID:         identity.ModuleID,
		AuthFingerprint:  identity.AuthFingerprint,
		ExportAPIVersion: OpenAPIExportAPIVersion,
		ExportParamsHash: ExportParamsHash(),
		Data:             raw,
	}
	path := GetOpenAPICachePathForWorkspace(workspaceDir, identity)
	if err := WriteEnvelope(path, envelope); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadModuleSnapshotOfflineFreshHit(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{
		ProjectID:       "proj1",
		AuthFingerprint: "fp1",
	}
	nowMs := int64(1_000_000_000_000)
	cachePath := writeFixtureEnvelope(t, workspaceDir, identity, nowMs-1000, map[string]any{
		"openapi": "3.1.0",
		"paths":   map[string]any{},
	})

	result, err := LoadModuleSnapshotOffline(LoadOptions{
		WorkspaceDir:    workspaceDir,
		ProjectID:       identity.ProjectID,
		AuthFingerprint: identity.AuthFingerprint,
		NowMs:           nowMs,
		Env:             map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stale {
		t.Fatal("expected fresh cache hit")
	}
	if result.Warning != "" {
		t.Fatalf("unexpected warning: %s", result.Warning)
	}
	if result.CachePath != cachePath {
		t.Fatalf("cache path = %s, want %s", result.CachePath, cachePath)
	}
	var doc map[string]any
	if err := json.Unmarshal(result.Data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("unexpected data: %v", doc)
	}
}

func TestLoadModuleSnapshotOfflineStaleStillReturnsData(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{
		ProjectID:       "proj1",
		AuthFingerprint: "fp1",
	}
	nowMs := int64(1_000_000_000_000)
	writeFixtureEnvelope(t, workspaceDir, identity, nowMs-DefaultCacheTTLMs-1, map[string]any{
		"openapi": "3.1.0",
	})

	result, err := LoadModuleSnapshotOffline(LoadOptions{
		WorkspaceDir:    workspaceDir,
		ProjectID:       identity.ProjectID,
		AuthFingerprint: identity.AuthFingerprint,
		NowMs:           nowMs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Stale {
		t.Fatal("expected stale=true")
	}
	if result.Warning == "" {
		t.Fatal("expected stale warning")
	}
}

func TestLoadModuleSnapshotOfflineMissingCache(t *testing.T) {
	_, err := LoadModuleSnapshotOffline(LoadOptions{
		WorkspaceDir:    t.TempDir(),
		ProjectID:       "missing",
		AuthFingerprint: "fp",
		NowMs:           1,
	})
	if err == nil {
		t.Fatal("expected error for missing cache")
	}
}

func TestLoadModuleSnapshotOfflineIdentityMismatch(t *testing.T) {
	workspaceDir := t.TempDir()
	identity := CacheIdentity{ProjectID: "proj1", AuthFingerprint: "fp1"}
	path := writeFixtureEnvelope(t, workspaceDir, identity, 100, map[string]any{"openapi": "3.1.0"})

	// Corrupt identity fields on disk while keeping filename.
	envelope, err := ReadEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	envelope.AuthFingerprint = "other"
	if err := WriteEnvelope(path, envelope); err != nil {
		t.Fatal(err)
	}

	_, err = LoadModuleSnapshotOffline(LoadOptions{
		WorkspaceDir:    workspaceDir,
		ProjectID:       identity.ProjectID,
		AuthFingerprint: identity.AuthFingerprint,
		NowMs:           200,
	})
	if err == nil {
		t.Fatal("expected identity mismatch error")
	}
}

func TestCacheTTLMsFromEnv(t *testing.T) {
	ttl, err := CacheTTLMs(map[string]string{EnvOpenAPICacheTTLMs: "3600000"})
	if err != nil {
		t.Fatal(err)
	}
	if ttl != 3600000 {
		t.Fatalf("ttl = %d", ttl)
	}
	_, err = CacheTTLMs(map[string]string{EnvOpenAPICacheTTLMs: "nope"})
	if err == nil {
		t.Fatal("expected invalid TTL error")
	}
}

func TestReadWriteEnvelopeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.openapi.json")
	envelope := Envelope{
		Timestamp:        42,
		ProjectID:        "p",
		AuthFingerprint:  "fp",
		ExportAPIVersion: OpenAPIExportAPIVersion,
		ExportParamsHash: ExportParamsHash(),
		Data:             json.RawMessage(`{"openapi":"3.1.0"}`),
	}
	if err := WriteEnvelope(path, envelope); err != nil {
		t.Fatal(err)
	}
	got, err := ReadEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Timestamp != 42 || got.ProjectID != "p" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	// Ensure no network: only local file exists.
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
