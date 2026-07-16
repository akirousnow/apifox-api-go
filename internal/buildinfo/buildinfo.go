// Package buildinfo holds version metadata injected at link time via ldflags.
package buildinfo

import "fmt"

// Version is the semver (or development label) of this build.
// Override at link time:
//
//	-X github.com/akirousnow/apifox-api-go/internal/buildinfo.Version=0.1.0
var Version = "dev"

// Commit is the source control revision of this build.
// Override at link time:
//
//	-X github.com/akirousnow/apifox-api-go/internal/buildinfo.Commit=abc1234
var Commit = "unknown"

// Format returns the stable version payload for --version and the version command.
// Shape: "apifox-api <semver> (commit <sha>)"
func Format() string {
	return fmt.Sprintf("apifox-api %s (commit %s)", Version, Commit)
}
