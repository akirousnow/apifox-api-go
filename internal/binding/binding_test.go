package binding_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"apifox-api/go-version/internal/binding"
)

func TestAuthFingerprintStable(t *testing.T) {
	t.Parallel()
	got := binding.AuthFingerprint("secret-token")
	if len(got) != 16 {
		t.Fatalf("fingerprint length = %d, want 16", len(got))
	}
	if binding.AuthFingerprint("secret-token") != got {
		t.Fatal("fingerprint not stable")
	}
	if binding.AuthFingerprint("other") == got {
		t.Fatal("different tokens should differ")
	}
}

func TestRuntimeAuthPrecedence(t *testing.T) {
	t.Parallel()
	env := map[string]string{binding.EnvAuthKey: "from-env"}
	key, _, usedEnv, err := binding.ResolveRuntimeAuthKey(env, "from-binding", "from-global")
	if err != nil || key != "from-env" || !usedEnv {
		t.Fatalf("env should win: key=%q usedEnv=%v err=%v", key, usedEnv, err)
	}

	key, _, usedEnv, err = binding.ResolveRuntimeAuthKey(map[string]string{}, "from-binding", "from-global")
	if err != nil || key != "from-binding" || usedEnv {
		t.Fatalf("binding should win: key=%q usedEnv=%v err=%v", key, usedEnv, err)
	}

	key, _, usedEnv, err = binding.ResolveRuntimeAuthKey(map[string]string{}, "", "from-global")
	if err != nil || key != "from-global" || usedEnv {
		t.Fatalf("global should win: key=%q usedEnv=%v err=%v", key, usedEnv, err)
	}

	_, _, _, err = binding.ResolveRuntimeAuthKey(map[string]string{}, "", "")
	if err == nil || !strings.Contains(err.Error(), "未配置 Apifox Auth Key") {
		t.Fatalf("expected missing auth error, got %v", err)
	}
}

func TestInitAuthGlobalNotPersisted(t *testing.T) {
	t.Parallel()
	resolution := binding.ResolveInitAuthKey("", map[string]string{}, nil, "global-only")
	if resolution.PrefetchAuthKey != "global-only" {
		t.Fatalf("prefetch=%q", resolution.PrefetchAuthKey)
	}
	if resolution.PersistAuthKey != "" {
		t.Fatalf("persist must be empty for global-only, got %q", resolution.PersistAuthKey)
	}
	if resolution.PrefetchSource != "全局默认" {
		t.Fatalf("source=%q", resolution.PrefetchSource)
	}
}

func TestInitAuthFlagWins(t *testing.T) {
	t.Parallel()
	existing := &binding.RegistryBinding{AuthKey: "stored"}
	resolution := binding.ResolveInitAuthKey("flag-key", map[string]string{binding.EnvAuthKey: "env-key"}, existing, "global")
	if resolution.PersistAuthKey != "flag-key" || resolution.PrefetchAuthKey != "flag-key" {
		t.Fatalf("flag should win: %+v", resolution)
	}
	if resolution.PrefetchSource != "命令行参数" {
		t.Fatalf("source=%q", resolution.PrefetchSource)
	}
}

func assertRegistryMode(t *testing.T, homeDir string) {
	t.Helper()
	path := binding.GetGlobalRegistryPath(homeDir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("registry mode = %o, want 0600", info.Mode().Perm())
		}
	}
}

func TestUpsertAndResolveExactAndWalkUp(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "work", "proj")
	if err := os.MkdirAll(filepath.Join(workspace, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD:       workspace,
		HomeDir:   homeDir,
		ProjectID: "proj-1",
		AuthKey:   "binding-token",
		ModuleIDs: []int{5, 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkspaceKey == "" {
		t.Fatal("empty workspace key")
	}

	// exact
	resolved, err := binding.ResolveBinding(binding.ResolveOptions{
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ProjectID != "proj-1" || resolved.AuthKey != "binding-token" {
		t.Fatalf("exact resolve: %+v", resolved)
	}

	// walk-up from subdir
	resolved, err = binding.ResolveBinding(binding.ResolveOptions{
		CWD:     filepath.Join(workspace, "subdir"),
		HomeDir: homeDir,
		Env:     map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ProjectID != "proj-1" {
		t.Fatalf("walk-up resolve: %+v", resolved)
	}

	// env overrides binding
	resolved, err = binding.ResolveBinding(binding.ResolveOptions{
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{binding.EnvAuthKey: "env-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.AuthKey != "env-token" {
		t.Fatalf("env override failed: %q", resolved.AuthKey)
	}
}

func TestMissingBindingError(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "empty-ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	// seed unrelated binding
	other := filepath.Join(homeDir, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD: other, HomeDir: homeDir, ProjectID: "other-id", ModuleIDs: []int{},
	}); err != nil {
		t.Fatal(err)
	}

	_, err := binding.ResolveBinding(binding.ResolveOptions{
		CWD: workspace, HomeDir: homeDir, Env: map[string]string{binding.EnvAuthKey: "x"},
	})
	if err == nil {
		t.Fatal("expected missing binding error")
	}
	msg := err.Error()
	for _, needle := range []string{
		"当前工作目录还没有绑定 Apifox 项目",
		"apifox-api init",
		"other-id",
	} {
		if !strings.Contains(msg, needle) {
			t.Fatalf("missing %q in error:\n%s", needle, msg)
		}
	}
}

func TestInitWithoutTokenAndGlobalNotCopied(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := binding.SetGlobalAuthKey(homeDir, "global-token"); err != nil {
		t.Fatal(err)
	}

	auth, err := binding.ResolveInitAuthKeyForCWD("", map[string]string{}, workspace, homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if auth.PersistAuthKey != "" || auth.PrefetchAuthKey != "global-token" {
		t.Fatalf("global prefetch only: %+v", auth)
	}

	// upsert without persist key
	if _, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD: workspace, HomeDir: homeDir, ProjectID: "p1", ModuleIDs: []int{},
	}); err != nil {
		t.Fatal(err)
	}

	registry, err := binding.ReadGlobalRegistry(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	key, err := binding.NormaliseWorkspaceKey(workspace)
	if err != nil {
		t.Fatal(err)
	}
	item := registry.Bindings[key]
	if item.AuthKey != "" {
		t.Fatalf("global must not be copied into binding, got %q", item.AuthKey)
	}
	if registry.AuthKey != "global-token" {
		t.Fatalf("global key missing: %q", registry.AuthKey)
	}
}

func TestReInitOverwrite(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	first, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD: workspace, HomeDir: homeDir, ProjectID: "old", AuthKey: "a", ModuleIDs: []int{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.PreviousBinding != nil {
		t.Fatal("first upsert should have no previous")
	}

	second, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD: workspace, HomeDir: homeDir, ProjectID: "new", ModuleIDs: []int{2, 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.PreviousBinding == nil || second.PreviousBinding.ProjectID != "old" {
		t.Fatalf("previous: %+v", second.PreviousBinding)
	}
	// no authKey on second means overwrite clears stored key
	registry, _ := binding.ReadGlobalRegistry(homeDir)
	item := registry.Bindings[second.WorkspaceKey]
	if item.ProjectID != "new" || item.AuthKey != "" || len(item.ModuleIDs) != 2 {
		t.Fatalf("overwrite result: %+v", item)
	}
}

func TestSetGlobalAuthKey(t *testing.T) {
	homeDir := t.TempDir()
	result, err := binding.SetGlobalAuthKey(homeDir, "  token-1  ")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasPrevious || result.NextFingerprint == "" {
		t.Fatalf("%+v", result)
	}
	// raw token not in path
	if strings.Contains(result.RegistryPath, "token-1") {
		t.Fatal("token leaked into path")
	}

	data, err := os.ReadFile(result.RegistryPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "token-1") == false {
		// token is stored in file (expected) but should not appear in stdout paths we return besides storage
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["authKey"] != "token-1" {
		t.Fatalf("stored=%v", parsed["authKey"])
	}

	result2, err := binding.SetGlobalAuthKey(homeDir, "token-2")
	if err != nil {
		t.Fatal(err)
	}
	if !result2.HasPrevious || result2.PreviousFingerprint != result.NextFingerprint {
		t.Fatalf("previous fingerprint: %+v", result2)
	}
}

func TestLegacyBinding(t *testing.T) {
	cwd := t.TempDir()
	legacyPath := filepath.Join(cwd, binding.LegacyBindingFileName)
	if err := os.WriteFile(legacyPath, []byte(`{"projectId":"legacy-1","projectName":"Old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy, err := binding.ReadLegacyBindingForMigration(cwd)
	if err != nil || legacy == nil {
		t.Fatalf("legacy=%v err=%v", legacy, err)
	}
	if legacy.ProjectID != "legacy-1" || legacy.ProjectName != "Old" {
		t.Fatalf("%+v", legacy)
	}
}

func TestAtomicRegistryWriteLeavesValidJSON(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := binding.UpsertBinding(binding.UpsertOptions{
		CWD: workspace, HomeDir: homeDir, ProjectID: "p", ModuleIDs: []int{},
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(binding.GetGlobalRegistryPath(homeDir))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if int(raw["schemaVersion"].(float64)) != 1 {
		t.Fatalf("schema=%v", raw["schemaVersion"])
	}
}
