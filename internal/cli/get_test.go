package cli

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
	"apifox-api/go-version/internal/snapshot"
)

func TestGetCommand_MethodPathAndPathOnly(t *testing.T) {
	homeDir := t.TempDir()
	cwd := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	authKey := "test-auth-key-for-get"
	_, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD:       cwd,
		HomeDir:   homeDir,
		ProjectID: "12345",
		AuthKey:   authKey,
		ModuleIDs: []int{7},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(cwd, ".current-module"), []byte("7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	openapiDoc := json.RawMessage(`{
  "openapi": "3.0.1",
  "paths": {
    "/ping": {
      "get": {
        "operationId": "ping",
        "summary": "Ping",
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "ok": { "type": "boolean" }
                  },
                  "required": ["ok"]
                }
              }
            }
          }
        }
      },
      "post": {
        "operationId": "pingPost",
        "summary": "Ping post",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": { "type": "object", "properties": { "msg": { "type": "string" } } }
            }
          }
        },
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": { "type": "object", "properties": { "ok": { "type": "boolean" } } }
              }
            }
          }
        }
      }
    }
  }
}`)

	moduleID := 7
	authFingerprint := binding.AuthFingerprint(authKey)
	cachePath := snapshot.GetOpenAPICachePathForWorkspace(cwd, snapshot.CacheIdentity{
		ProjectID:       "12345",
		AuthFingerprint: authFingerprint,
		ModuleID:        &moduleID,
	})
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeGetEnvelope(cachePath, snapshot.Envelope{
		Timestamp:        time.Now().UnixMilli(),
		ProjectID:        "12345",
		AuthFingerprint:  authFingerprint,
		ModuleID:         &moduleID,
		ExportAPIVersion: snapshot.OpenAPIExportAPIVersion,
		ExportParamsHash: snapshot.ExportParamsHash(),
		Data:             openapiDoc,
	}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runGetCLI(homeDir, cwd, []string{"get", "GET", "/ping"})
	if err != nil {
		t.Fatalf("get GET /ping failed: %v\nstderr=%s\nstdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "export type PingResponse") {
		t.Fatalf("expected PingResponse in stdout:\n%s", stdout)
	}
	if strings.Contains(stdout, "警告") {
		t.Fatalf("stdout must be pure TS, got warning mixed in:\n%s", stdout)
	}

	stdout, stderr, err = runGetCLI(homeDir, cwd, []string{"get", "/ping", "--method", "POST"})
	if err != nil {
		t.Fatalf("get /ping --method POST failed: %v\n%s\n%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "export type PingPostRequestBody") {
		t.Fatalf("expected request body type:\n%s", stdout)
	}

	stdout, stderr, err = runGetCLI(homeDir, cwd, []string{"get", "/ping"})
	if err != nil {
		t.Fatalf("get /ping failed: %v\n%s\n%s", err, stderr, stdout)
	}
	getIdx := strings.Index(stdout, "// ═════════ GET /ping ═════════")
	postIdx := strings.Index(stdout, "// ═════════ POST /ping ═════════")
	if getIdx < 0 || postIdx < 0 || getIdx > postIdx {
		t.Fatalf("expected GET before POST separators:\n%s", stdout)
	}
}

func TestGetCommand_InvalidMethod(t *testing.T) {
	homeDir := t.TempDir()
	cwd := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD:       cwd,
		HomeDir:   homeDir,
		ProjectID: "12345",
		AuthKey:   "test-auth-key-for-get",
		ModuleIDs: []int{7},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runGetCLI(homeDir, cwd, []string{"get", "/x", "--method", "NOPE"})
	if err == nil {
		t.Fatal("expected invalid method error")
	}
	combined := err.Error() + stderr
	if !strings.Contains(combined, "合法 HTTP 方法") && !strings.Contains(combined, "method") {
		t.Fatalf("unexpected error: %v / %s", err, stderr)
	}
}

func TestParseGetArgs_UsageOnEmpty(t *testing.T) {
	_, err := parseGetArgs(nil, "")
	if err == nil || !strings.Contains(err.Error(), "用法:") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func runGetCLI(homeDir, cwd string, args []string) (stdout string, stderr string, err error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	deps := Dependencies{
		Streams: Streams{Out: &outBuf, Err: &errBuf},
		CWD:     cwd,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}
	err = Execute(context.Background(), deps, args)
	return outBuf.String(), errBuf.String(), err
}

func writeGetEnvelope(cachePath string, envelope snapshot.Envelope) error {
	payload := map[string]any{
		"timestamp":        envelope.Timestamp,
		"projectId":        envelope.ProjectID,
		"authFingerprint":  envelope.AuthFingerprint,
		"exportApiVersion": envelope.ExportAPIVersion,
		"exportParamsHash": envelope.ExportParamsHash,
		"data":             json.RawMessage(envelope.Data),
	}
	if envelope.ModuleID != nil {
		payload["moduleId"] = *envelope.ModuleID
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, raw, 0o644)
}
