// Package binding implements Project Binding registry I/O, path identity,
// auth resolution, and related domain types for the apifox-api CLI.
package binding

// RegistryBinding is one workspace → Apifox project mapping stored in the global registry.
type RegistryBinding struct {
	ProjectID   string  `json:"projectId"`
	AuthKey     string  `json:"authKey,omitempty"`
	ModuleIDs   []int   `json:"moduleIds"`
	ProjectName string  `json:"projectName,omitempty"`
}

// GlobalRegistry is schema v1 of ~/.apifox-api.json.
type GlobalRegistry struct {
	SchemaVersion int                        `json:"schemaVersion"`
	AuthKey       string                     `json:"authKey,omitempty"`
	Bindings      map[string]RegistryBinding `json:"bindings"`
}

// ResolvedBinding is a runtime-resolved Project Binding with a usable auth key.
type ResolvedBinding struct {
	ProjectID       string
	AuthKey         string
	AuthFingerprint string
	ModuleIDs       []int
	ProjectName     string
	WorkspaceDir    string
	RegistryPath    string
	StoredAuthKey   string
	Source          string
}

// UpsertResult is returned after writing or overwriting a Project Binding.
type UpsertResult struct {
	RegistryPath    string
	WorkspaceKey    string
	PreviousBinding *RegistryBinding
}

// InitAuthKeyResolution separates keys that may be persisted into a binding
// from keys that may only be used for prefetch (e.g. Global Auth Key).
type InitAuthKeyResolution struct {
	PrefetchAuthKey     string
	PersistAuthKey      string
	PrefetchFingerprint string
	PrefetchSource      string
}

// LegacyBinding is the old workspace-local MCP binding file shape.
type LegacyBinding struct {
	ProjectID   string
	ProjectName string
}

const (
	// GlobalRegistryFileName is the registry file under the user home directory.
	GlobalRegistryFileName = ".apifox-api.json"
	// LegacyBindingFileName is the old per-workspace MCP binding file.
	LegacyBindingFileName = ".apifox-mcp.json"
	// CurrentModuleFileName is written under the binding root for multi-module init.
	CurrentModuleFileName = ".current-module"
	// SchemaVersion is the only supported registry schema.
	SchemaVersion = 1
	// EnvAuthKey is the environment variable that overrides stored tokens at runtime.
	EnvAuthKey = "APIFOX_AUTH_KEY"
)
