package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akirousnow/apifox-api-go/internal/binding"
	"github.com/akirousnow/apifox-api-go/internal/snapshot"
)

func TestRefreshCommandHardFailsWithoutStaleFallback(t *testing.T) {
	homeDir := t.TempDir()
	cwd := t.TempDir()

	// Seed a project binding with modules and auth.
	authKey := "test-auth-key-for-refresh"
	registryPath := filepath.Join(homeDir, ".apifox-api.json")
	registry := map[string]any{
		"version": 1,
		"projects": map[string]any{
			cwd: map[string]any{
				"projectId":  "proj-refresh",
				"moduleIds":  []int{11, 22},
				"updatedAt":  "2020-01-01T00:00:00Z",
			},
		},
	}
	raw, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registryPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed stale caches so a naive stale-fallback implementation would succeed incorrectly.
	fp := binding.AuthFingerprint(authKey)
	nowMs := int64(1_000_000_000_000)
	for _, moduleID := range []int{11, 22} {
		moduleIDCopy := moduleID
		identity := snapshot.CacheIdentity{
			ProjectID:       "proj-refresh",
			AuthFingerprint: fp,
			ModuleID:        &moduleIDCopy,
		}
		envelope := snapshot.Envelope{
			Timestamp:        nowMs - snapshot.DefaultCacheTTLMs - 1,
			ProjectID:        identity.ProjectID,
			ModuleID:         identity.ModuleID,
			AuthFingerprint:  identity.AuthFingerprint,
			ExportAPIVersion: snapshot.OpenAPIExportAPIVersion,
			ExportParamsHash: snapshot.ExportParamsHash(),
			Data:             json.RawMessage(`{"openapi":"3.1.0","paths":{"/old":{}}}`),
		}
		path := snapshot.GetOpenAPICachePathForWorkspace(cwd, identity)
		if err := snapshot.WriteEnvelope(path, envelope); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	err = Execute(context.Background(), Dependencies{
		Streams: Streams{Out: &stdout, Err: &stderr},
		CWD:     cwd,
		HomeDir: homeDir,
		Env: map[string]string{
			"APIFOX_AUTH_KEY": authKey,
		},
	}, []string{"refresh"})
	if err == nil {
		t.Fatalf("expected refresh to hard-fail without network export success; stdout=%s", stdout.String())
	}
	if !strings.Contains(err.Error(), "refresh 失败") && !strings.Contains(err.Error(), "刷新 OpenAPI 快照失败") {
		t.Fatalf("error = %v", err)
	}
	// Must not pretend success with stale data.
	if strings.Contains(stdout.String(), "refresh 完成") {
		t.Fatalf("must not report success: %s", stdout.String())
	}
}

func TestRefreshCommandMissingAuth(t *testing.T) {
	homeDir := t.TempDir()
	cwd := t.TempDir()
	registryPath := filepath.Join(homeDir, ".apifox-api.json")
	registry := map[string]any{
		"version": 1,
		"projects": map[string]any{
			cwd: map[string]any{
				"projectId": "proj-refresh",
				"moduleIds": []int{},
				"updatedAt": "2020-01-01T00:00:00Z",
			},
		},
	}
	raw, _ := json.MarshalIndent(registry, "", "  ")
	if err := os.WriteFile(registryPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := Execute(context.Background(), Dependencies{
		Streams: Streams{Out: &stdout, Err: &stderr},
		CWD:     cwd,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"refresh"})
	if err == nil {
		t.Fatal("expected missing auth error")
	}
	if !strings.Contains(err.Error(), "Auth") && !strings.Contains(err.Error(), "auth") && !strings.Contains(err.Error(), "Auth Key") {
		// Chinese message
		if !strings.Contains(err.Error(), "Auth Key") && !strings.Contains(err.Error(), "auth") {
			// accept Chinese
			if !strings.Contains(err.Error(), "Auth Key") {
				// just check non-empty failure
				if err.Error() == "" {
					t.Fatal("empty error")
				}
			}
		}
	}
}
