//go:build unix

package snapshot

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// lockCachePath acquires an exclusive flock on a sibling .lock file for cachePath.
// Returns an unlock function. On systems without flock support this is a no-op.
func lockCachePath(cachePath string) (func(), error) {
	lockPath := cachePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		_ = lockFile.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		_ = lockFile.Close()
	}, nil
}
