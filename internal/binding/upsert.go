package binding

import (
	"fmt"
	"os"
	"strings"
)

// UpsertOptions writes or overwrites the Project Binding for the exact cwd key.
type UpsertOptions struct {
	CWD          string
	HomeDir      string
	ProjectID    string
	AuthKey      string
	ModuleIDs    []int
	ProjectName  string
	CustomSource string
}

// UpsertBinding overwrites the exact workspace key binding in the global registry.
func UpsertBinding(options UpsertOptions) (UpsertResult, error) {
	cwd := options.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return UpsertResult{}, err
		}
	}
	homeDir := options.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return UpsertResult{}, err
		}
	}

	workspaceKey, err := NormaliseWorkspaceKey(cwd)
	if err != nil {
		return UpsertResult{}, err
	}

	projectID, err := ValidateProjectID(options.ProjectID)
	if err != nil {
		return UpsertResult{}, err
	}

	moduleIDs := make([]int, 0, len(options.ModuleIDs))
	for _, moduleID := range options.ModuleIDs {
		validated, err := ValidateModuleID(moduleID)
		if err != nil {
			return UpsertResult{}, err
		}
		moduleIDs = append(moduleIDs, validated)
	}

	registry, err := ReadGlobalRegistry(homeDir)
	if err != nil {
		return UpsertResult{}, err
	}

	var previous *RegistryBinding
	if existing, ok := registry.Bindings[workspaceKey]; ok {
		copyExisting := existing
		previous = &copyExisting
	}

	next := RegistryBinding{
		ProjectID: projectID,
		ModuleIDs: moduleIDs,
	}
	if trimmed := strings.TrimSpace(options.AuthKey); trimmed != "" {
		next.AuthKey = trimmed
	}
	if trimmedName := strings.TrimSpace(options.ProjectName); trimmedName != "" {
		next.ProjectName = trimmedName
	}
	if customSource := strings.TrimSpace(options.CustomSource); customSource != "" {
		next.CustomSource = customSource
	}

	if registry.Bindings == nil {
		registry.Bindings = map[string]RegistryBinding{}
	}
	registry.Bindings[workspaceKey] = next

	registryPath, err := WriteGlobalRegistry(homeDir, registry)
	if err != nil {
		return UpsertResult{}, err
	}

	return UpsertResult{
		RegistryPath:    registryPath,
		WorkspaceKey:    workspaceKey,
		PreviousBinding: previous,
	}, nil
}

// SetGlobalAuthKeyResult is the desensitised outcome of config set-auth-key.
type SetGlobalAuthKeyResult struct {
	RegistryPath        string
	PreviousFingerprint string
	HasPrevious         bool
	NextFingerprint     string
}

// SetGlobalAuthKey writes the Global Auth Key into the registry top-level authKey field.
func SetGlobalAuthKey(homeDir string, authKey string) (SetGlobalAuthKeyResult, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return SetGlobalAuthKeyResult{}, err
		}
	}
	trimmed := strings.TrimSpace(authKey)
	if trimmed == "" {
		return SetGlobalAuthKeyResult{}, fmt.Errorf("authKey 不能为空。")
	}

	registry, err := ReadGlobalRegistry(homeDir)
	if err != nil {
		return SetGlobalAuthKeyResult{}, err
	}

	result := SetGlobalAuthKeyResult{
		NextFingerprint: AuthFingerprint(trimmed),
	}
	if strings.TrimSpace(registry.AuthKey) != "" {
		result.HasPrevious = true
		result.PreviousFingerprint = AuthFingerprint(registry.AuthKey)
	}

	registry.AuthKey = trimmed
	registryPath, err := WriteGlobalRegistry(homeDir, registry)
	if err != nil {
		return SetGlobalAuthKeyResult{}, err
	}
	result.RegistryPath = registryPath
	return result, nil
}

// ResolveInitAuthKeyForCWD loads the registry and applies init auth precedence
// for the exact cwd workspace key (no walk-up for existing binding).
func ResolveInitAuthKeyForCWD(flagAuthKey string, env map[string]string, cwd string, homeDir string) (InitAuthKeyResolution, error) {
	if env == nil {
		env = envMapFromOS()
	}
	workspaceKey, err := NormaliseWorkspaceKey(cwd)
	if err != nil {
		return InitAuthKeyResolution{}, err
	}
	registry, err := ReadGlobalRegistry(homeDir)
	if err != nil {
		return InitAuthKeyResolution{}, err
	}
	var existing *RegistryBinding
	if binding, ok := registry.Bindings[workspaceKey]; ok {
		copyBinding := binding
		existing = &copyBinding
	}
	return ResolveInitAuthKey(flagAuthKey, env, existing, registry.AuthKey), nil
}
