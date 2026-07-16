package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akirousnow/apifox-api-go/internal/binding"
	"github.com/akirousnow/apifox-api-go/internal/cli"
)

func TestInitAndConfigCommands(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	// set global auth key via CLI
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{In: strings.NewReader(""), Out: &stdout, Err: &stderr},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"config", "set-auth-key", "global-secret-token"})
	if err != nil {
		t.Fatalf("config: %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "global-secret-token") {
		t.Fatalf("token leaked to stdout: %s", out)
	}
	if !strings.Contains(out, "已设置全局默认") {
		t.Fatalf("stdout=%s", out)
	}

	// init without token: binding written, global not copied
	stdout.Reset()
	stderr.Reset()
	err = cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{In: strings.NewReader(""), Out: &stdout, Err: &stderr},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"init", "proj123"})
	if err != nil {
		t.Fatalf("init: %v stderr=%s", err, stderr.String())
	}
	initOut := stdout.String()
	if strings.Contains(initOut, "global-secret-token") {
		t.Fatalf("token leaked: %s", initOut)
	}
	if !strings.Contains(initOut, "已写入 Apifox Project Binding") {
		t.Fatalf("stdout=%s", initOut)
	}
	if !strings.Contains(initOut, "全局默认") {
		t.Fatalf("expected global-default auth message: %s", initOut)
	}

	registry, err := binding.ReadGlobalRegistry(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if registry.AuthKey != "global-secret-token" {
		t.Fatalf("global key=%q", registry.AuthKey)
	}
	key, _ := binding.NormaliseWorkspaceKey(workspace)
	if registry.Bindings[key].AuthKey != "" {
		t.Fatalf("binding should not store global key: %+v", registry.Bindings[key])
	}
	if registry.Bindings[key].ProjectID != "proj123" {
		t.Fatalf("binding=%+v", registry.Bindings[key])
	}

	// re-init with flag auth
	stdout.Reset()
	stderr.Reset()
	err = cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{In: strings.NewReader(""), Out: &stdout, Err: &stderr},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"init", "proj999", "--authKey", "binding-secret", "--moduleIds", "5,8"})
	if err != nil {
		t.Fatalf("re-init: %v", err)
	}
	if !strings.Contains(stdout.String(), "已覆盖原有绑定") {
		t.Fatalf("expected overwrite summary: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "binding-secret") {
		t.Fatal("token leaked on re-init")
	}

	// multi-module current module file
	currentModulePath := filepath.Join(key, binding.CurrentModuleFileName)
	data, err := os.ReadFile(currentModulePath)
	if err != nil {
		t.Fatalf("current module file: %v", err)
	}
	if strings.TrimSpace(string(data)) != "5" {
		t.Fatalf("current module = %q", data)
	}

	// registry JSON shape
	raw, _ := os.ReadFile(binding.GetGlobalRegistryPath(homeDir))
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if int(parsed["schemaVersion"].(float64)) != 1 {
		t.Fatalf("schema=%v", parsed["schemaVersion"])
	}
}

func TestInitLegacyMessage(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, binding.LegacyBindingFileName), []byte(`{"projectId":"legacy-p","projectName":"N"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{In: strings.NewReader(""), Out: &stdout, Err: &bytes.Buffer{}},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"init", "new-p"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "检测到旧版工作区绑定") {
		t.Fatalf("stdout=%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "建议手动删除") {
		t.Fatalf("stdout=%s", stdout.String())
	}
}
