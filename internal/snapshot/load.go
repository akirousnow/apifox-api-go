package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/akirousnow/apifox-api-go/internal/customdoc"
	"github.com/akirousnow/apifox-api-go/internal/openapi"
)

// LoadResult is a loaded OpenAPI Snapshot (local cache and/or remote refresh).
type LoadResult struct {
	Data      json.RawMessage
	CachePath string
	Stale     bool
	Warning   string
	ModuleID  *int
	Refreshed bool
}

// LoadOptions controls snapshot loading (offline or online with stale fallback).
type LoadOptions struct {
	WorkspaceDir    string
	ProjectID       string
	AuthFingerprint string
	// CustomSource replaces the Apifox exporter with a user-provided file or URL.
	CustomSource string
	ModuleID     *int
	Env          map[string]string
	// NowMs overrides the clock for tests (milliseconds since epoch).
	NowMs int64

	// ForceRefresh skips the fresh-cache short circuit and always hits remote.
	ForceRefresh bool
	// AllowStaleOnError falls back to a matching stale snapshot on transient remote failure.
	// When nil, defaults to true (search/get). Explicit refresh sets false.
	AllowStaleOnError *bool
	// AuthKey is the Bearer token used for remote export (never logged).
	AuthKey string
	// FetchFunc overrides the default HTTP export client (tests inject httptest).
	FetchFunc FetchFunc
	// Context cancels in-flight export; nil uses context.Background().
	Context context.Context
	// OfflineOnly never performs network I/O (used by LoadModuleSnapshotOffline).
	OfflineOnly bool
}

// CacheTTLMs reads APIFOX_MCP_OPENAPI_TTL_MS or returns the 24h default.
func CacheTTLMs(env map[string]string) (int64, error) {
	raw := ""
	if env != nil {
		raw = strings.TrimSpace(env[EnvOpenAPICacheTTLMs])
	}
	if raw == "" {
		return DefaultCacheTTLMs, nil
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("APIFOX_MCP_OPENAPI_TTL_MS 必须是非负数字。")
	}
	return int64(parsed), nil
}

// MatchesIdentity reports whether the envelope belongs to the current binding/export params.
func MatchesIdentity(envelope Envelope, identity CacheIdentity) bool {
	if envelope.ProjectID != identity.ProjectID {
		return false
	}
	if envelope.AuthFingerprint != identity.AuthFingerprint {
		return false
	}
	if envelope.ExportAPIVersion != OpenAPIExportAPIVersion {
		return false
	}
	if envelope.ExportParamsHash != ExportParamsHash() {
		return false
	}
	if identity.ModuleID == nil {
		return envelope.ModuleID == nil
	}
	if envelope.ModuleID == nil {
		return false
	}
	return *envelope.ModuleID == *identity.ModuleID
}

// IsFresh reports whether the envelope is within the TTL window (strict less-than).
func IsFresh(envelope Envelope, nowMs int64, ttlMs int64) bool {
	return nowMs-envelope.Timestamp < ttlMs
}

func allowStaleOnError(options LoadOptions) bool {
	if options.AllowStaleOnError == nil {
		return true
	}
	return *options.AllowStaleOnError
}

func moduleIDFilePartForError(moduleID *int) string {
	return ModuleIDFilePart(moduleID)
}

// LoadModuleSnapshotOffline loads a matching cache file without network I/O.
// Fresh hits return stale=false. Matching stale hits return data with a warning
// so offline search remains usable until remote refresh lands.
func LoadModuleSnapshotOffline(options LoadOptions) (LoadResult, error) {
	options.OfflineOnly = true
	options.ForceRefresh = false
	return LoadModuleSnapshot(options)
}

// LoadModuleSnapshot resolves a module snapshot with optional remote refresh and stale fallback.
// Fresh (non-stale) cache hits do not perform network I/O.
func LoadModuleSnapshot(options LoadOptions) (LoadResult, error) {
	identity := CacheIdentity{
		ProjectID:       options.ProjectID,
		AuthFingerprint: options.AuthFingerprint,
		ModuleID:        options.ModuleID,
	}
	cachePath := GetOpenAPICachePathForWorkspace(options.WorkspaceDir, identity)

	ttlMs, err := CacheTTLMs(options.Env)
	if err != nil {
		return LoadResult{}, err
	}

	nowMs := options.NowMs
	if nowMs == 0 {
		nowMs = time.Now().UnixMilli()
	}

	matchedEnvelope, matchedOK, readErr := readMatchingEnvelope(cachePath, identity)
	if readErr != nil {
		return LoadResult{}, readErr
	}

	// Fresh cache hit: no network I/O.
	if !options.ForceRefresh && matchedOK && IsFresh(matchedEnvelope, nowMs, ttlMs) {
		data, normalizeErr := openapi.NormalizeDocument(matchedEnvelope.Data)
		if normalizeErr != nil {
			return LoadResult{}, fmt.Errorf("无法解析 OpenAPI 快照: %w", normalizeErr)
		}
		return LoadResult{
			Data:      data,
			CachePath: cachePath,
			Stale:     false,
			ModuleID:  options.ModuleID,
			Refreshed: false,
		}, nil
	}

	// Offline-only path: never fetch; return stale matching data or a clear error.
	if options.OfflineOnly {
		if !matchedOK {
			return LoadResult{}, fmt.Errorf(
				"未找到当前 module 的 OpenAPI 快照缓存: %s。请先运行 `apifox-api refresh`（或带 auth 的 init）生成缓存。",
				cachePath,
			)
		}
		if IsFresh(matchedEnvelope, nowMs, ttlMs) {
			data, normalizeErr := openapi.NormalizeDocument(matchedEnvelope.Data)
			if normalizeErr != nil {
				return LoadResult{}, fmt.Errorf("无法解析 OpenAPI 快照: %w", normalizeErr)
			}
			return LoadResult{
				Data:      data,
				CachePath: cachePath,
				Stale:     false,
				ModuleID:  options.ModuleID,
			}, nil
		}
		data, normalizeErr := openapi.NormalizeDocument(matchedEnvelope.Data)
		if normalizeErr != nil {
			return LoadResult{}, fmt.Errorf("无法解析 OpenAPI 快照: %w", normalizeErr)
		}
		return LoadResult{
			Data:      data,
			CachePath: cachePath,
			Stale:     true,
			Warning:   "OpenAPI 快照已过期，已使用本地缓存（离线模式；可稍后运行 apifox-api refresh）。",
			ModuleID:  options.ModuleID,
		}, nil
	}

	// Remote refresh requires Auth Key.
	if strings.TrimSpace(options.AuthKey) == "" && strings.TrimSpace(options.CustomSource) == "" {
		return LoadResult{}, fmt.Errorf(
			"刷新 OpenAPI 快照失败 (moduleId=%s): 缺少 Auth Key，无法从远程拉取 OpenAPI 快照。",
			moduleIDFilePartForError(options.ModuleID),
		)
	}

	// Remote refresh path.
	fetchFunc := options.FetchFunc
	if fetchFunc == nil {
		if strings.TrimSpace(options.CustomSource) != "" {
			fetchFunc = func(ctx context.Context, _ string, _ string, _ *int) (json.RawMessage, error) {
				loaded, loadErr := customdoc.Load(ctx, options.CustomSource, options.WorkspaceDir)
				return json.RawMessage(loaded.Data), loadErr
			}
		} else {
			client := NewDefaultHTTPClient()
			fetchFunc = func(ctx context.Context, projectID string, authKey string, moduleID *int) (json.RawMessage, error) {
				return FetchOpenAPIExport(ctx, client, projectID, authKey, moduleID)
			}
		}
	}

	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}

	data, fetchErr := fetchFunc(ctx, options.ProjectID, options.AuthKey, options.ModuleID)
	if fetchErr == nil {
		if strings.TrimSpace(options.CustomSource) != "" {
			data, fetchErr = openapi.NormalizeCustomDocument(data)
		} else {
			data, fetchErr = openapi.NormalizeDocument(data)
		}
	}
	if fetchErr == nil {
		envelope := Envelope{
			Timestamp:        nowMs,
			ProjectID:        options.ProjectID,
			ModuleID:         options.ModuleID,
			AuthFingerprint:  options.AuthFingerprint,
			ExportAPIVersion: OpenAPIExportAPIVersion,
			ExportParamsHash: ExportParamsHash(),
			Data:             data,
		}
		if writeErr := WriteEnvelope(cachePath, envelope); writeErr != nil {
			return LoadResult{}, fmt.Errorf(
				"刷新 OpenAPI 快照失败 (moduleId=%s): %w",
				moduleIDFilePartForError(options.ModuleID),
				writeErr,
			)
		}
		return LoadResult{
			Data:      data,
			CachePath: cachePath,
			Stale:     false,
			ModuleID:  options.ModuleID,
			Refreshed: true,
		}, nil
	}

	// Stale fallback for search/get on transient (and by TS parity: any) remote failure
	// when a matching cache exists and AllowStaleOnError is enabled.
	// Transient is defined narrowly (timeout / connection / 5xx); auth/4xx hard-fail.
	if !options.ForceRefresh && allowStaleOnError(options) && matchedOK && IsTransientExportError(fetchErr) {
		data, normalizeErr := openapi.NormalizeDocument(matchedEnvelope.Data)
		if normalizeErr != nil {
			return LoadResult{}, fmt.Errorf("无法解析 OpenAPI 快照: %w", normalizeErr)
		}
		return LoadResult{
			Data:      data,
			CachePath: cachePath,
			Stale:     true,
			Warning:   fmt.Sprintf("OpenAPI 快照刷新失败，已使用本地过期缓存: %s", fetchErr.Error()),
			ModuleID:  options.ModuleID,
			Refreshed: false,
		}, nil
	}

	return LoadResult{}, fmt.Errorf(
		"刷新 OpenAPI 快照失败 (moduleId=%s): %s",
		moduleIDFilePartForError(options.ModuleID),
		fetchErr.Error(),
	)
}

func readMatchingEnvelope(cachePath string, identity CacheIdentity) (Envelope, bool, error) {
	envelope, err := ReadEnvelope(cachePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return Envelope{}, false, nil
		}
		// Invalid JSON / unreadable: treat as no match for online path identity checks,
		// but surface identity-unrelated read errors only when file exists and is corrupt.
		// Match TS: readCacheEntry returns undefined on any parse failure.
		return Envelope{}, false, nil
	}
	if !MatchesIdentity(envelope, identity) {
		return Envelope{}, false, nil
	}
	return envelope, true, nil
}

// LoadAllModuleSnapshots loads every bound module snapshot.
// Empty moduleIDs means the default module (moduleId nil) is loaded once.
// On partial failure the error names the failing module; completed module caches remain valid.
func LoadAllModuleSnapshots(
	workspaceDir string,
	projectID string,
	authKey string,
	authFingerprint string,
	moduleIDs []int,
	options LoadOptions,
) ([]LoadResult, error) {
	moduleIDsToLoad := make([]*int, 0, max(1, len(moduleIDs)))
	if len(moduleIDs) == 0 {
		moduleIDsToLoad = append(moduleIDsToLoad, nil)
	} else {
		for _, moduleID := range moduleIDs {
			moduleIDCopy := moduleID
			moduleIDsToLoad = append(moduleIDsToLoad, &moduleIDCopy)
		}
	}

	results := make([]LoadResult, 0, len(moduleIDsToLoad))
	for _, moduleID := range moduleIDsToLoad {
		loadOptions := options
		loadOptions.WorkspaceDir = workspaceDir
		loadOptions.ProjectID = projectID
		loadOptions.AuthKey = authKey
		loadOptions.AuthFingerprint = authFingerprint
		loadOptions.ModuleID = moduleID
		result, err := LoadModuleSnapshot(loadOptions)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}
