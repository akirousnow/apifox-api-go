package binding

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// GetGlobalRegistryPath returns ~/.apifox-api.json under homeDir.
func GetGlobalRegistryPath(homeDir string) string {
	return filepath.Join(homeDir, GlobalRegistryFileName)
}

// EmptyRegistry returns a valid empty schema v1 registry.
func EmptyRegistry() GlobalRegistry {
	return GlobalRegistry{
		SchemaVersion: SchemaVersion,
		Bindings:      map[string]RegistryBinding{},
	}
}

// ReadGlobalRegistry loads and validates the registry. Missing file → empty registry.
func ReadGlobalRegistry(homeDir string) (GlobalRegistry, error) {
	registryPath := GetGlobalRegistryPath(homeDir)
	data, err := os.ReadFile(registryPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			// Also treat path-not-a-dir style missing as empty.
			return EmptyRegistry(), nil
		}
		// ENOTDIR / missing parent: treat as empty when file simply does not exist yet.
		if os.IsNotExist(err) {
			return EmptyRegistry(), nil
		}
		return GlobalRegistry{}, fmt.Errorf("读取 Apifox 全局注册表失败: %s: %v", registryPath, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return GlobalRegistry{}, fmt.Errorf("无效的 Apifox 全局注册表: %s 不是有效 JSON。", registryPath)
	}

	registry, err := parseGlobalRegistry(raw, registryPath)
	if err != nil {
		return GlobalRegistry{}, err
	}
	return registry, nil
}

func parseGlobalRegistry(raw map[string]json.RawMessage, registryPath string) (GlobalRegistry, error) {
	// schemaVersion
	schemaRaw, ok := raw["schemaVersion"]
	if !ok {
		return GlobalRegistry{}, fmt.Errorf("无效的 Apifox 全局注册表: %s 的 schemaVersion 必须是 1。", registryPath)
	}
	var schemaVersion int
	if err := json.Unmarshal(schemaRaw, &schemaVersion); err != nil || schemaVersion != SchemaVersion {
		return GlobalRegistry{}, fmt.Errorf("无效的 Apifox 全局注册表: %s 的 schemaVersion 必须是 1。", registryPath)
	}

	registry := GlobalRegistry{
		SchemaVersion: SchemaVersion,
		Bindings:      map[string]RegistryBinding{},
	}

	if authRaw, ok := raw["authKey"]; ok && string(authRaw) != "null" {
		var authKey string
		if err := json.Unmarshal(authRaw, &authKey); err != nil {
			return GlobalRegistry{}, fmt.Errorf("无效的 Apifox 全局注册表: %s 的 authKey 必须是字符串。", registryPath)
		}
		if trimmed := strings.TrimSpace(authKey); trimmed != "" {
			registry.AuthKey = trimmed
		}
	}

	bindingsRaw, ok := raw["bindings"]
	if !ok {
		return registry, nil
	}
	var bindingsMap map[string]json.RawMessage
	if err := json.Unmarshal(bindingsRaw, &bindingsMap); err != nil {
		return GlobalRegistry{}, fmt.Errorf("无效的 Apifox 全局注册表: %s 的 bindings 必须是对象。", registryPath)
	}

	for workspaceKey, bindingRaw := range bindingsMap {
		binding, err := parseRegistryBinding(bindingRaw, workspaceKey)
		if err != nil {
			return GlobalRegistry{}, err
		}
		registry.Bindings[workspaceKey] = binding
	}
	return registry, nil
}

func parseRegistryBinding(bindingRaw json.RawMessage, workspaceKey string) (RegistryBinding, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(bindingRaw, &raw); err != nil {
		return RegistryBinding{}, fmt.Errorf("无效的 Apifox 绑定记录: %s 必须是 JSON 对象。", workspaceKey)
	}

	projectIDRaw, ok := raw["projectId"]
	if !ok {
		return RegistryBinding{}, fmt.Errorf("无效的 Apifox 绑定记录: %s 缺少 projectId。", workspaceKey)
	}
	var projectID string
	if err := json.Unmarshal(projectIDRaw, &projectID); err != nil {
		return RegistryBinding{}, fmt.Errorf("无效的 Apifox 绑定记录: %s 缺少 projectId。", workspaceKey)
	}

	binding := RegistryBinding{
		ProjectID: projectID,
		ModuleIDs: []int{},
	}

	if authRaw, ok := raw["authKey"]; ok && string(authRaw) != "null" {
		var authKey string
		if err := json.Unmarshal(authRaw, &authKey); err != nil {
			return RegistryBinding{}, fmt.Errorf("无效的 Apifox 绑定记录: %s 的 authKey 必须是字符串。", workspaceKey)
		}
		binding.AuthKey = authKey
	}

	if nameRaw, ok := raw["projectName"]; ok && string(nameRaw) != "null" {
		var projectName string
		if err := json.Unmarshal(nameRaw, &projectName); err != nil {
			return RegistryBinding{}, fmt.Errorf("无效的 Apifox 绑定记录: %s 的 projectName 必须是字符串。", workspaceKey)
		}
		binding.ProjectName = projectName
	}

	if moduleRaw, ok := raw["moduleIds"]; ok && string(moduleRaw) != "null" {
		var moduleIDs []int
		if err := json.Unmarshal(moduleRaw, &moduleIDs); err != nil {
			return RegistryBinding{}, fmt.Errorf("无效的 Apifox 绑定记录: %s 的 moduleIds 必须是数组。", workspaceKey)
		}
		for _, moduleID := range moduleIDs {
			if moduleID <= 0 {
				return RegistryBinding{}, fmt.Errorf("无效的 Apifox 绑定记录: %s 的 moduleIds 必须是正整数数组。", workspaceKey)
			}
		}
		binding.ModuleIDs = moduleIDs
	}

	return binding, nil
}

// WriteGlobalRegistry atomically writes the registry with mode 0600 and best-effort parent 0700.
func WriteGlobalRegistry(homeDir string, registry GlobalRegistry) (string, error) {
	if registry.Bindings == nil {
		registry.Bindings = map[string]RegistryBinding{}
	}
	registry.SchemaVersion = SchemaVersion

	registryPath := GetGlobalRegistryPath(homeDir)
	parentDir := filepath.Dir(registryPath)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		return "", fmt.Errorf("写入 Apifox 全局注册表失败: %s: %v", registryPath, err)
	}
	// Best-effort tighten parent permissions (ignore errors on platforms that do not support it).
	_ = os.Chmod(parentDir, 0o700)

	payload, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("写入 Apifox 全局注册表失败: %s: %v", registryPath, err)
	}
	payload = append(payload, '\n')

	tempFile, err := os.CreateTemp(parentDir, ".apifox-api-*.tmp")
	if err != nil {
		return "", fmt.Errorf("写入 Apifox 全局注册表失败: %s: %v", registryPath, err)
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(payload); err != nil {
		_ = tempFile.Close()
		return "", fmt.Errorf("写入 Apifox 全局注册表失败: %s: %v", registryPath, err)
	}
	if err := tempFile.Chmod(0o600); err != nil {
		// Best-effort; continue.
		_ = err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return "", fmt.Errorf("写入 Apifox 全局注册表失败: %s: %v", registryPath, err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("写入 Apifox 全局注册表失败: %s: %v", registryPath, err)
	}

	if err := os.Rename(tempPath, registryPath); err != nil {
		return "", fmt.Errorf("写入 Apifox 全局注册表失败: %s: %v", registryPath, err)
	}
	cleanupTemp = false
	_ = os.Chmod(registryPath, 0o600)
	return registryPath, nil
}
