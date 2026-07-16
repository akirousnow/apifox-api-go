package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportParamsHashMatchesTypeScriptCanonicalJSON(t *testing.T) {
	canonical := `{"apiVersion":"2024-03-28","locale":"zh-CN","body":{"scope":{"type":"ALL"},"options":{"includeApifoxExtensionProperties":false,"addFoldersToTags":false},"oasVersion":"3.1","exportFormat":"JSON"}}`
	sum := sha256.Sum256([]byte(canonical))
	want := hex.EncodeToString(sum[:])[:16]
	if got := ExportParamsHash(); got != want {
		t.Fatalf("ExportParamsHash() = %q, want %q", got, want)
	}
}

func TestGetOpenAPICachePathScopesIdentity(t *testing.T) {
	moduleID := 42
	identity := CacheIdentity{
		ProjectID:       "proj/123",
		AuthFingerprint: "a55da776ce3539ce",
		ModuleID:        &moduleID,
	}
	path := GetOpenAPICachePath(identity, "/tmp/cache")
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "proj_123.m42.a55da776ce3539ce.") {
		t.Fatalf("unexpected cache basename: %s", base)
	}
	if !strings.HasSuffix(base, ".openapi.json") {
		t.Fatalf("expected .openapi.json suffix, got %s", base)
	}
	if !strings.Contains(base, ExportParamsHash()) {
		t.Fatalf("expected export params hash in path: %s", base)
	}
}

func TestModuleIDFilePartDefault(t *testing.T) {
	if got := ModuleIDFilePart(nil); got != "default" {
		t.Fatalf("ModuleIDFilePart(nil) = %q", got)
	}
}

func TestSafeFilePart(t *testing.T) {
	if got := SafeFilePart("a/b:c"); got != "a_b_c" {
		t.Fatalf("SafeFilePart = %q", got)
	}
}

func TestGetOpenAPICachePathForWorkspace(t *testing.T) {
	path := GetOpenAPICachePathForWorkspace("/ws", CacheIdentity{
		ProjectID:       "p1",
		AuthFingerprint: "fp",
	})
	wantPrefix := filepath.Join("/ws", ".cache", "apifox-api", "p1.default.fp.")
	if !strings.HasPrefix(path, wantPrefix) {
		t.Fatalf("path = %s, want prefix %s", path, wantPrefix)
	}
}
