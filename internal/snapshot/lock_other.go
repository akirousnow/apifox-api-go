//go:build !unix

package snapshot

// lockCachePath is a no-op on non-unix platforms; atomic rename still protects
// against torn writes within a single process.
func lockCachePath(cachePath string) (func(), error) {
	return func() {}, nil
}
