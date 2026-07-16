package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Envelope is the on-disk OpenAPI Snapshot cache wrapper (schema shared with TypeScript).
type Envelope struct {
	Timestamp        int64           `json:"timestamp"`
	ProjectID        string          `json:"projectId"`
	ModuleID         *int            `json:"moduleId,omitempty"`
	AuthFingerprint  string          `json:"authFingerprint"`
	ExportAPIVersion string          `json:"exportApiVersion"`
	ExportParamsHash string          `json:"exportParamsHash"`
	Data             json.RawMessage `json:"data"`
}

// ReadEnvelope loads a cache file. Missing/unreadable/invalid JSON returns an error.
func ReadEnvelope(cachePath string) (Envelope, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return Envelope{}, err
	}
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("无效的 OpenAPI 快照缓存: %s 不是有效 JSON。", cachePath)
	}
	return envelope, nil
}

// WriteEnvelope atomically writes a pretty-printed envelope via temp file + rename,
// under a cross-process lock when available. Interrupted or concurrent writers leave
// either the previous complete file or the new complete file — never truncated/half JSON.
func WriteEnvelope(cachePath string, envelope Envelope) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	unlock, err := lockCachePath(cachePath)
	if err != nil {
		return err
	}
	defer unlock()

	directory := filepath.Dir(cachePath)
	tempFile, err := os.CreateTemp(directory, ".openapi-*.tmp")
	if err != nil {
		return err
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
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tempPath, cachePath); err != nil {
		return err
	}
	cleanupTemp = false
	return nil
}
