// Package snapshot loads OpenAPI Snapshot cache envelopes offline.
// Cache path and envelope identity match the TypeScript fetcher layout so
// existing .cache/apifox-api/*.openapi.json files are reusable.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strconv"
)

const (
	// OpenAPIExportAPIVersion is the X-Apifox-Api-Version used for export-openapi.
	OpenAPIExportAPIVersion = "2024-03-28"
	// OpenAPIExportLocale is the locale query parameter for export-openapi.
	OpenAPIExportLocale = "zh-CN"
	// DefaultCacheTTLMs is 24 hours, matching APIFOX_MCP_OPENAPI_TTL_MS default.
	DefaultCacheTTLMs = 24 * 60 * 60 * 1000
	// EnvOpenAPICacheTTLMs overrides the snapshot freshness window.
	EnvOpenAPICacheTTLMs = "APIFOX_MCP_OPENAPI_TTL_MS"
	// CacheSubdir is relative to the workspace binding root.
	CacheSubdir = ".cache/apifox-api"
)

// exportParamsHashInput is the exact JSON.stringify payload TypeScript hashes.
// Key order must match TS object insertion order (not Go map alphabetical order).
const exportParamsHashInput = `{"apiVersion":"2024-03-28","locale":"zh-CN","body":{"scope":{"type":"ALL"},"options":{"includeApifoxExtensionProperties":false,"addFoldersToTags":false},"oasVersion":"3.1","exportFormat":"JSON"}}`

var unsafeFilePartPattern = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// CacheIdentity holds the fields that scope a snapshot cache file.
type CacheIdentity struct {
	ProjectID       string
	AuthFingerprint string
	// ModuleID is nil for the default module.
	ModuleID *int
}

// SafeFilePart replaces characters that are unsafe in filenames with underscores.
func SafeFilePart(value string) string {
	return unsafeFilePartPattern.ReplaceAllString(value, "_")
}

// ModuleIDFilePart returns "default" when moduleID is nil, else "m{id}".
func ModuleIDFilePart(moduleID *int) string {
	if moduleID == nil {
		return "default"
	}
	return "m" + strconv.Itoa(*moduleID)
}

// ExportParamsHash is sha256(JSON.stringify({apiVersion,locale,body})).hex[:16].
func ExportParamsHash() string {
	sum := sha256.Sum256([]byte(exportParamsHashInput))
	return hex.EncodeToString(sum[:])[:16]
}

// GetOpenAPICacheDir returns {workspaceDir}/.cache/apifox-api.
func GetOpenAPICacheDir(workspaceDir string) string {
	return filepath.Join(workspaceDir, ".cache", "apifox-api")
}

// GetOpenAPICachePath builds the TS-compatible cache filename:
// {safeProjectId}.{modulePart}.{authFingerprint}.{exportParamsHash}.openapi.json
func GetOpenAPICachePath(identity CacheIdentity, cacheDir string) string {
	if cacheDir == "" {
		cacheDir = "."
	}
	filename := SafeFilePart(identity.ProjectID) + "." +
		ModuleIDFilePart(identity.ModuleID) + "." +
		identity.AuthFingerprint + "." +
		ExportParamsHash() + "." +
		"openapi.json"
	return filepath.Join(cacheDir, filename)
}

// GetOpenAPICachePathForWorkspace builds the full path under the workspace cache dir.
func GetOpenAPICachePathForWorkspace(workspaceDir string, identity CacheIdentity) string {
	return GetOpenAPICachePath(identity, GetOpenAPICacheDir(workspaceDir))
}
