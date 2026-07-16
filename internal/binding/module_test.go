package binding

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveCurrentModuleEmptyDefault(t *testing.T) {
	moduleID, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		ModuleIDs: []int{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moduleID != nil {
		t.Fatalf("expected nil default module, got %v", *moduleID)
	}
}

func TestResolveCurrentModuleEmptyRejectsModuleIDFlag(t *testing.T) {
	flag := 5
	_, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		ModuleIDs:    []int{},
		ModuleIDFlag: &flag,
	})
	if err == nil {
		t.Fatal("expected error for --moduleId on default-only project")
	}
	if !strings.Contains(err.Error(), "当前项目只使用默认模块，不接受 --moduleId") {
		t.Fatalf("error message: %v", err)
	}
}

func TestResolveCurrentModuleSingleNoFile(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	moduleID, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ModuleIDs: []int{42},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moduleID == nil || *moduleID != 42 {
		t.Fatalf("expected 42, got %v", moduleID)
	}

	// No .current-module required for single-module bindings.
	if _, err := os.Stat(filepath.Join(workspace, CurrentModuleFileName)); !os.IsNotExist(err) {
		t.Fatalf("did not expect .current-module file: %v", err)
	}
}

func TestResolveCurrentModuleMultiFromFile(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteCurrentModuleFile(workspace, 8); err != nil {
		t.Fatal(err)
	}

	moduleID, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ModuleIDs: []int{5, 8, 12},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moduleID == nil || *moduleID != 8 {
		t.Fatalf("expected 8, got %v", moduleID)
	}
}

func TestResolveCurrentModuleMultiMissingFile(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ModuleIDs: []int{5, 8},
	})
	if err == nil {
		t.Fatal("expected error when multi-module has no current file")
	}
	if !strings.Contains(err.Error(), "当前项目绑定了多个 module，但未指定当前 module") {
		t.Fatalf("error message: %v", err)
	}
}

func TestResolveCurrentModuleOverride(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteCurrentModuleFile(workspace, 5); err != nil {
		t.Fatal(err)
	}

	flag := 12
	moduleID, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		CWD:          workspace,
		HomeDir:      homeDir,
		ModuleIDs:    []int{5, 8, 12},
		ModuleIDFlag: &flag,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moduleID == nil || *moduleID != 12 {
		t.Fatalf("expected override 12, got %v", moduleID)
	}

	// Override must not rewrite .current-module.
	data, err := os.ReadFile(filepath.Join(workspace, CurrentModuleFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "5" {
		t.Fatalf("override rewrote .current-module: %q", data)
	}
}

func TestResolveCurrentModuleUnboundOverride(t *testing.T) {
	flag := 99
	_, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		ModuleIDs:    []int{5, 8},
		ModuleIDFlag: &flag,
	})
	if err == nil {
		t.Fatal("expected unbound --moduleId error")
	}
	if !strings.Contains(err.Error(), "--moduleId 99 不在当前项目绑定的 moduleIds") {
		t.Fatalf("error message: %v", err)
	}
}

func TestModulesForRefreshAllBoundIndependentOfCurrent(t *testing.T) {
	// Non-empty: every bound module, independent of Current Module.
	got := ModulesForRefresh([]int{5, 8, 12})
	want := []int{5, 8, 12}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}

	// Empty moduleIds: default module once — empty slice is the refresh unit marker.
	gotEmpty := ModulesForRefresh([]int{})
	if len(gotEmpty) != 0 {
		t.Fatalf("expected empty slice for default-only refresh, got %v", gotEmpty)
	}

	// Mutation safety: returned slice is a copy.
	original := []int{1, 2}
	selected := ModulesForRefresh(original)
	selected[0] = 99
	if original[0] != 1 {
		t.Fatal("ModulesForRefresh must not mutate input")
	}
}

func TestCurrentModuleFileFormatCompatibility(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	// TypeScript writes "${id}\n".
	tsFixturePath := filepath.Join(workspace, CurrentModuleFileName)
	if err := os.WriteFile(tsFixturePath, []byte("8\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := ReadCurrentModuleFile(workspace, homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if content != "8" {
		t.Fatalf("read content = %q", content)
	}

	moduleID, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ModuleIDs: []int{5, 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if moduleID == nil || *moduleID != 8 {
		t.Fatalf("got %v", moduleID)
	}

	// Go write must match TS on-disk format.
	if err := WriteCurrentModuleFile(workspace, 5); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(tsFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "5\n" {
		t.Fatalf("on-disk format = %q, want %q", raw, "5\n")
	}
}

func TestFormatAndSummariseModuleIDs(t *testing.T) {
	if got := FormatModuleIDs([]int{}); got != "[]（默认模块）" {
		t.Fatalf("FormatModuleIDs empty = %q", got)
	}
	if got := FormatModuleIDs([]int{1, 2}); got != "[1, 2]" {
		t.Fatalf("FormatModuleIDs = %q", got)
	}
	if got := SummariseModuleID(nil); got != "默认模块" {
		t.Fatalf("SummariseModuleID nil = %q", got)
	}
	value := 7
	if got := SummariseModuleID(&value); got != "moduleId=7" {
		t.Fatalf("SummariseModuleID = %q", got)
	}
}

func TestResolveCurrentModuleWalkUp(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	nested := filepath.Join(workspace, "packages", "svc")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteCurrentModuleFile(workspace, 12); err != nil {
		t.Fatal(err)
	}

	moduleID, err := ResolveCurrentModule(ResolveCurrentModuleOptions{
		CWD:       nested,
		HomeDir:   homeDir,
		ModuleIDs: []int{5, 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	if moduleID == nil || *moduleID != 12 {
		t.Fatalf("expected walk-up to find 12, got %v", moduleID)
	}
}
