package binding

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// ResolveOptions controls Project Binding resolution for the current process.
type ResolveOptions struct {
	CWD     string
	HomeDir string
	Env     map[string]string
}

// ResolveBinding finds the Project Binding for cwd (exact key first, then walk-up)
// and resolves the runtime Apifox Auth Key (env → binding → global).
func ResolveBinding(options ResolveOptions) (ResolvedBinding, error) {
	cwd := options.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return ResolvedBinding{}, err
		}
	}
	homeDir := options.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return ResolvedBinding{}, err
		}
	}
	env := options.Env
	if env == nil {
		env = envMapFromOS()
	}

	workspaceKey, err := NormaliseWorkspaceKey(cwd)
	if err != nil {
		return ResolvedBinding{}, err
	}
	homeKey, err := NormaliseWorkspaceKey(homeDir)
	if err != nil {
		return ResolvedBinding{}, err
	}

	registry, err := ReadGlobalRegistry(homeDir)
	if err != nil {
		return ResolvedBinding{}, err
	}

	checkedKeys := AncestorKeys(workspaceKey, homeKey)
	registryPath := GetGlobalRegistryPath(homeDir)

	for _, candidateKey := range checkedKeys {
		candidate, ok := registry.Bindings[candidateKey]
		if !ok {
			continue
		}
		authKey, fingerprint, _, authErr := ResolveRuntimeAuthKey(env, candidate.AuthKey, registry.AuthKey)
		if authErr != nil {
			return ResolvedBinding{}, authErr
		}
		resolved := ResolvedBinding{
			ProjectID:       candidate.ProjectID,
			AuthKey:         authKey,
			AuthFingerprint: fingerprint,
			ModuleIDs:       append([]int(nil), candidate.ModuleIDs...),
			ProjectName:     candidate.ProjectName,
			WorkspaceDir:    candidateKey,
			RegistryPath:    registryPath,
			Source:          fmt.Sprintf("全局注册表 %s -> %s", registryPath, candidateKey),
		}
		if strings.TrimSpace(candidate.AuthKey) != "" {
			resolved.StoredAuthKey = candidate.AuthKey
		}
		return resolved, nil
	}

	return ResolvedBinding{}, createMissingBindingError(workspaceKey, checkedKeys, registry.Bindings)
}

func createMissingBindingError(workspaceKey string, checkedKeys []string, bindings map[string]RegistryBinding) error {
	checkedLines := make([]string, 0, len(checkedKeys))
	for _, key := range checkedKeys {
		checkedLines = append(checkedLines, "- "+key)
	}
	return fmt.Errorf(
		"当前工作目录还没有绑定 Apifox 项目。\n已解析工作目录: %s\n已检查的目录:\n%s\n\n全局注册表中已有的绑定:\n%s\n\n请在目标项目根目录运行:\n  apifox-api init <projectId> [--moduleIds 5,8,12] [--authKey <token>]",
		workspaceKey,
		strings.Join(checkedLines, "\n"),
		formatExistingBindingsList(bindings),
	)
}

func formatExistingBindingsList(bindings map[string]RegistryBinding) string {
	if len(bindings) == 0 {
		return "（全局注册表中目前没有任何绑定。）"
	}
	keys := make([]string, 0, len(bindings))
	for key := range bindings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		item := bindings[key]
		moduleHint := ""
		if len(item.ModuleIDs) > 0 {
			parts := make([]string, len(item.ModuleIDs))
			for i, id := range item.ModuleIDs {
				parts[i] = fmt.Sprintf("%d", id)
			}
			moduleHint = ", moduleIds=[" + strings.Join(parts, ",") + "]"
		}
		nameHint := ""
		if item.ProjectName != "" {
			nameHint = " (" + item.ProjectName + ")"
		}
		lines = append(lines, fmt.Sprintf("- %s -> projectId=%s%s%s", key, item.ProjectID, nameHint, moduleHint))
	}
	return strings.Join(lines, "\n")
}

func envMapFromOS() map[string]string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}
