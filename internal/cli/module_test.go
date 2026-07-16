package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"apifox-api/go-version/internal/binding"
	"apifox-api/go-version/internal/cli"
)

func setupBoundWorkspace(t *testing.T, moduleIDs []int) (homeDir string, workspace string) {
	t.Helper()
	homeDir = t.TempDir()
	workspace = filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ProjectID: "proj-module",
		AuthKey:   "token-module",
		ModuleIDs: moduleIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(moduleIDs) > 1 {
		if err := binding.WriteCurrentModuleFile(workspace, moduleIDs[0]); err != nil {
			t.Fatal(err)
		}
	}
	return homeDir, workspace
}

func executeCLI(t *testing.T, cwd string, homeDir string, args ...string) (stdout string, stderr string, err error) {
	t.Helper()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	err = cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{In: strings.NewReader(""), Out: &outBuf, Err: &errBuf},
		CWD:     cwd,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, args)
	return outBuf.String(), errBuf.String(), err
}

func TestModuleShowDefault(t *testing.T) {
	homeDir, workspace := setupBoundWorkspace(t, []int{})

	stdout, stderr, err := executeCLI(t, workspace, homeDir, "module")
	if err != nil {
		t.Fatalf("err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "当前 module: 默认模块") {
		t.Fatalf("stdout=%s", stdout)
	}
	if !strings.Contains(stdout, "绑定的 moduleIds: []（默认模块）") {
		t.Fatalf("stdout=%s", stdout)
	}
}

func TestModuleShowSingle(t *testing.T) {
	homeDir, workspace := setupBoundWorkspace(t, []int{42})

	stdout, stderr, err := executeCLI(t, workspace, homeDir, "module")
	if err != nil {
		t.Fatalf("err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "当前 module: moduleId=42") {
		t.Fatalf("stdout=%s", stdout)
	}
	if !strings.Contains(stdout, "绑定的 moduleIds: [42]") {
		t.Fatalf("stdout=%s", stdout)
	}
}

func TestModuleShowAndSetMulti(t *testing.T) {
	homeDir, workspace := setupBoundWorkspace(t, []int{5, 8, 12})

	stdout, stderr, err := executeCLI(t, workspace, homeDir, "module")
	if err != nil {
		t.Fatalf("show err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "当前 module: moduleId=5") {
		t.Fatalf("stdout=%s", stdout)
	}
	if !strings.Contains(stdout, "绑定的 moduleIds: [5, 8, 12]") {
		t.Fatalf("stdout=%s", stdout)
	}

	stdout, stderr, err = executeCLI(t, workspace, homeDir, "module", "8")
	if err != nil {
		t.Fatalf("set err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "已切换当前 module 为 8。") {
		t.Fatalf("stdout=%s", stdout)
	}

	data, readErr := os.ReadFile(filepath.Join(workspace, binding.CurrentModuleFileName))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "8\n" {
		t.Fatalf("on-disk format = %q", data)
	}

	stdout, stderr, err = executeCLI(t, workspace, homeDir, "module")
	if err != nil {
		t.Fatalf("reshow err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "当前 module: moduleId=8") {
		t.Fatalf("stdout=%s", stdout)
	}
}

func TestModuleSetUnboundRejected(t *testing.T) {
	homeDir, workspace := setupBoundWorkspace(t, []int{5, 8})

	_, _, err := executeCLI(t, workspace, homeDir, "module", "99")
	if err == nil {
		t.Fatal("expected unbound module set to fail")
	}
	if !strings.Contains(err.Error(), "不在绑定的 moduleIds") {
		t.Fatalf("err=%v", err)
	}
}

func TestModuleShowMultiMissingCurrent(t *testing.T) {
	homeDir, workspace := setupBoundWorkspace(t, []int{5, 8})
	// Remove the file written by setup to simulate missing current module.
	_ = os.Remove(filepath.Join(workspace, binding.CurrentModuleFileName))

	stdout, stderr, err := executeCLI(t, workspace, homeDir, "module")
	if err == nil {
		t.Fatal("expected failure when multi has no current module")
	}
	// Process contract: Execute returns the error; Run maps it to stderr.
	// This unit test uses Execute, so assert the returned error text.
	if !strings.Contains(err.Error(), "当前项目绑定了多个 module，但未指定当前 module") {
		t.Fatalf("stderr=%s err=%v", stderr, err)
	}
	if !strings.Contains(stdout, "绑定的 moduleIds: [5, 8]") {
		t.Fatalf("stdout should still list bound modules: %s", stdout)
	}
}




