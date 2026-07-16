package binding

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// NormaliseWorkspaceKey turns a workspace directory into the registry map key:
// absolute path → realpath when possible → forward slashes → lower-case on Windows.
func NormaliseWorkspaceKey(workspaceDir string) (string, error) {
	resolved, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", err
	}
	realPath, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		realPath = resolved
	}
	return slashAndLower(realPath), nil
}

// NormaliseAncestorKey normalises an ancestor path without requiring the path to exist.
func NormaliseAncestorKey(ancestorPath string) string {
	resolved, err := filepath.Abs(ancestorPath)
	if err != nil {
		resolved = ancestorPath
	}
	return slashAndLower(resolved)
}

func slashAndLower(workspaceDir string) string {
	withSlashes := strings.ReplaceAll(workspaceDir, "\\", "/")
	if runtime.GOOS == "windows" {
		return strings.ToLower(withSlashes)
	}
	return withSlashes
}

// AncestorKeys returns workspaceKey and each parent key up to and including homeKey
// (or the filesystem root). Match order is exact-first (cwd before parents).
func AncestorKeys(workspaceKey string, homeKey string) []string {
	keys := make([]string, 0, 8)
	current := workspaceKey
	for {
		keys = append(keys, current)
		if current == homeKey {
			break
		}
		parent := path.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return keys
}
