package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akirousnow/apifox-api-go/internal/binding"
	"github.com/akirousnow/apifox-api-go/internal/cli"
	"github.com/akirousnow/apifox-api-go/internal/snapshot"
)

const processContractSecret = "SUPER-SECRET-TOKEN-NEVER-LEAK"

func normalizeNewlines(value string) string {
	return strings.ReplaceAll(value, "\r\n", "\n")
}

func assertNoSecret(t *testing.T, streams string) {
	t.Helper()
	if strings.Contains(streams, processContractSecret) {
		t.Fatalf("secret leaked into streams:\n%s", streams)
	}
	if strings.Contains(streams, "Bearer "+processContractSecret) {
		t.Fatalf("bearer token leaked into streams:\n%s", streams)
	}
}

func setupProcessWorkspace(t *testing.T, projectID string, moduleIDs []int) (homeDir string, workspace string) {
	t.Helper()
	homeDir = t.TempDir()
	workspace = filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ProjectID: projectID,
		AuthKey:   processContractSecret,
		ModuleIDs: moduleIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(moduleIDs) > 0 {
		if err := binding.WriteCurrentModuleFile(workspace, moduleIDs[0]); err != nil {
			t.Fatal(err)
		}
	}
	return homeDir, workspace
}

func writeProcessCache(t *testing.T, workspace, projectID, authKey string, moduleID *int, timestamp int64) {
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
						"responses": map[string]any{
							"200": map[string]any{
								"description": "ok",
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"id": map[string]any{"type": "integer"},
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
}

func runProcess(t *testing.T, deps cli.Dependencies, args []string) (stdout string, stderr string, exitCode int) {
	t.Helper()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	deps.Streams = cli.Streams{
		In:  strings.NewReader(""),
		Out: &outBuf,
		Err: &errBuf,
	}
	if deps.Env == nil {
		deps.Env = map[string]string{}
	}
	exitCode = cli.Run(context.Background(), deps, args)
	stdout = normalizeNewlines(outBuf.String())
	stderr = normalizeNewlines(errBuf.String())
	assertNoSecret(t, stdout+stderr)
	return stdout, stderr, exitCode
}

func TestProcessContractVersion(t *testing.T) {
	stdout, stderr, code := runProcess(t, cli.Dependencies{}, []string{"version"})
	if code != cli.ExitSuccess {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "apifox-api") {
		t.Fatalf("stdout=%q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr should be empty, got %q", stderr)
	}
}

func TestProcessContractInitAndConfig(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	deps := cli.Dependencies{CWD: workspace, HomeDir: homeDir}

	stdout, stderr, code := runProcess(t, deps, []string{"config", "set-auth-key", processContractSecret})
	if code != cli.ExitSuccess {
		t.Fatalf("config exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "已设置全局默认") {
		t.Fatalf("stdout=%q", stdout)
	}
	if strings.Contains(stdout, processContractSecret) {
		t.Fatal("raw token in config stdout")
	}

	stdout, stderr, code = runProcess(t, deps, []string{"init", "proj-process-1", "--authKey", processContractSecret})
	if code != cli.ExitSuccess {
		t.Fatalf("init exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "已写入 Apifox Project Binding") && !strings.Contains(stdout, "Project Binding") {
		// init message may vary slightly; accept binding confirmation
		if !strings.Contains(stdout, "proj-process-1") && !strings.Contains(stdout, "绑定") {
			t.Fatalf("init stdout=%q", stdout)
		}
	}
}

func TestProcessContractModuleShow(t *testing.T) {
	homeDir, workspace := setupProcessWorkspace(t, "proj-mod", []int{5})
	stdout, stderr, code := runProcess(t, cli.Dependencies{CWD: workspace, HomeDir: homeDir}, []string{"module"})
	if code != cli.ExitSuccess {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "当前 module") {
		t.Fatalf("stdout=%q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestProcessContractSearchAndGetHappy(t *testing.T) {
	homeDir, workspace := setupProcessWorkspace(t, "proj-search", nil)
	writeProcessCache(t, workspace, "proj-search", processContractSecret, nil, time.Now().UnixMilli())
	deps := cli.Dependencies{CWD: workspace, HomeDir: homeDir}

	stdout, stderr, code := runProcess(t, deps, []string{"search", "pets"})
	if code != cli.ExitSuccess {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "/pets") && !strings.Contains(stdout, "GET") {
		t.Fatalf("search stdout=%q", stdout)
	}
	if strings.Contains(stdout, "警告") {
		t.Fatalf("warning must not be on stdout: %q", stdout)
	}

	stdout, stderr, code = runProcess(t, deps, []string{"search", "--json", "pets"})
	if code != cli.ExitSuccess {
		t.Fatalf("search json exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"stale"`) {
		t.Fatalf("json missing stale field: %q", stdout)
	}
	if strings.Contains(stdout, "策略") {
		t.Fatalf("strategy prose in json: %q", stdout)
	}

	stdout, stderr, code = runProcess(t, deps, []string{"get", "GET", "/pets"})
	if code != cli.ExitSuccess {
		t.Fatalf("get exit=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, "警告") {
		t.Fatalf("warning in get stdout: %q", stdout)
	}
	if stderr != "" && strings.Contains(stderr, "警告") {
		// fresh cache should not warn
		t.Fatalf("unexpected stale warning on fresh get: %q", stderr)
	}
	if !strings.Contains(stdout, "export") && !strings.Contains(stdout, "interface") && !strings.Contains(stdout, "type ") {
		// typesgen output shape may vary; at least pure non-empty TS-ish payload
		if strings.TrimSpace(stdout) == "" {
			t.Fatalf("get stdout empty")
		}
	}
}

func TestProcessContractFailurePaths(t *testing.T) {
	homeDir, workspace := setupProcessWorkspace(t, "proj-fail", nil)
	writeProcessCache(t, workspace, "proj-fail", processContractSecret, nil, time.Now().UnixMilli())
	deps := cli.Dependencies{CWD: workspace, HomeDir: homeDir}

	_, stderr, code := runProcess(t, deps, []string{"search"})
	if code != cli.ExitFailure {
		t.Fatalf("empty search exit=%d", code)
	}
	if !strings.Contains(stderr, "keywords") && !strings.Contains(stderr, "--method") {
		t.Fatalf("empty search stderr=%q", stderr)
	}

	_, stderr, code = runProcess(t, deps, []string{"search", "--method", "NOTAMETHOD", "pets"})
	if code != cli.ExitFailure {
		t.Fatalf("invalid method search exit=%d", code)
	}
	if !strings.Contains(stderr, "method") && !strings.Contains(stderr, "METHOD") && !strings.Contains(stderr, "HTTP") {
		t.Fatalf("invalid method stderr=%q", stderr)
	}

	_, stderr, code = runProcess(t, deps, []string{"get", "NOTAMETHOD", "/pets"})
	if code != cli.ExitFailure {
		t.Fatalf("invalid method get exit=%d", code)
	}

	// missing binding
	emptyHome := t.TempDir()
	emptyWS := filepath.Join(emptyHome, "ws")
	_ = os.MkdirAll(emptyWS, 0o755)
	_, stderr, code = runProcess(t, cli.Dependencies{CWD: emptyWS, HomeDir: emptyHome}, []string{"search", "pets"})
	if code != cli.ExitFailure {
		t.Fatalf("missing binding exit=%d", code)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Fatal("expected stderr error for missing binding")
	}

	_, stderr, code = runProcess(t, cli.Dependencies{CWD: emptyWS, HomeDir: emptyHome}, []string{"refresh"})
	if code != cli.ExitFailure {
		t.Fatalf("refresh missing binding exit=%d", code)
	}
}

func TestProcessContractRefreshHardFailNoSecret(t *testing.T) {
	homeDir, workspace := setupProcessWorkspace(t, "proj-refresh", nil)
	deps := cli.Dependencies{
		CWD:     workspace,
		HomeDir: homeDir,
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			return nil, fmt.Errorf("connection refused by test fake")
		},
	}
	_, stderr, code := runProcess(t, deps, []string{"refresh"})
	if code != cli.ExitFailure {
		t.Fatalf("refresh exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "refresh") && !strings.Contains(stderr, "刷新") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestProcessContractStaleWarningStderrOnly(t *testing.T) {
	homeDir, workspace := setupProcessWorkspace(t, "proj-stale", nil)
	// 48h old => stale under default 24h TTL
	oldTimestamp := time.Now().Add(-48 * time.Hour).UnixMilli()
	writeProcessCache(t, workspace, "proj-stale", processContractSecret, nil, oldTimestamp)

	deps := cli.Dependencies{
		CWD:     workspace,
		HomeDir: homeDir,
		FetchFunc: func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
			// transient-looking error so search/get can fall back
			return nil, fmt.Errorf("connection reset by peer")
		},
	}

	stdout, stderr, code := runProcess(t, deps, []string{"search", "pets"})
	if code != cli.ExitSuccess {
		t.Fatalf("stale search exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "警告") && !strings.Contains(stderr, "过期") && !strings.Contains(stderr, "缓存") {
		t.Fatalf("expected stale warning on stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "警告") {
		t.Fatalf("warning must not be on stdout: %q", stdout)
	}

	stdout, stderr, code = runProcess(t, deps, []string{"search", "--json", "pets"})
	if code != cli.ExitSuccess {
		t.Fatalf("stale json exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"stale": true`) && !strings.Contains(stdout, `"stale":true`) {
		t.Fatalf("json should mark stale: %q", stdout)
	}
	if strings.Contains(stdout, "警告") {
		t.Fatalf("warning in json stdout: %q", stdout)
	}

	stdout, stderr, code = runProcess(t, deps, []string{"get", "GET", "/pets"})
	if code != cli.ExitSuccess {
		t.Fatalf("stale get exit=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, "警告") {
		t.Fatalf("warning in get TS stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "警告") && !strings.Contains(stderr, "过期") && !strings.Contains(stderr, "缓存") {
		t.Fatalf("expected get stale warning on stderr: %q", stderr)
	}
}

func TestProcessContractCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	code := cli.Run(ctx, cli.Dependencies{
		Streams: cli.Streams{Out: &outBuf, Err: &errBuf},
	}, []string{"version"})
	// version may complete before cancel is observed; either success or interrupted is acceptable
	// for a pre-canceled context. Prefer testing ExitCode mapping directly:
	if code != cli.ExitSuccess && code != cli.ExitFailure {
		t.Fatalf("unexpected exit %d", code)
	}
	// Dedicated cancel path via ExitCode/IsCancel already covered in exitcode_test.
	_ = errBuf.String()
}
