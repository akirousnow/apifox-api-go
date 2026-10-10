// Package buildinfo resolves version metadata for the CLI.
//
// Resolution order (first non-empty wins):
//
//  1. Link-time injection via ldflags (release pipeline, scripts/release.sh):
//     -X github.com/akirousnow/apifox-api-go/internal/buildinfo.Version=0.1.4
//     -X github.com/akirousnow/apifox-api-go/internal/buildinfo.Commit=abc1234
//  2. Go build info (debug.ReadBuildInfo):
//     - `go install module@vX.Y.Z` embeds Main.Version="vX.Y.Z".
//     - `go build` inside a git checkout embeds vcs.revision.
//  3. fallbackVersion (mirrors the VERSION file of the current release) and
//     "unknown" for the commit.
//
// This matters because `go install pkg@latest` does not run scripts/release.sh,
// so without step 2 such binaries would always report "dev (commit unknown)".
package buildinfo

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
)

// fallbackVersion mirrors the VERSION file of the current release. Bump it in
// the same commit that bumps VERSION. It is only used when neither ldflags nor
// Go build info provide a module version (e.g. plain `go build` in a repo).
const fallbackVersion = "0.1.4"

// Version is the semver of this build. Empty means "resolve at runtime".
var Version = ""

// Commit is the source control revision of this build. Empty means "resolve at
// runtime".
var Commit = ""

// resolve returns the effective version and commit.
func resolve() (version, commit string) {
	version, commit = Version, Commit
	if version == "" {
		version = moduleVersion()
	}
	if version == "" {
		version = fallbackVersion
	}
	if commit == "" {
		commit = vcsRevision()
	}
	if commit == "" {
		commit = "unknown"
	}
	return version, commit
}

// pseudoVersionPattern matches Go pseudo-versions such as
// "v0.1.1-0.20260824101034-a0d42c8a5eb3" or "v0.0.0-20260824101034-a0d42c8a5eb3"
// (optionally suffixed with "+dirty"). These identify development builds whose
// version should come from fallbackVersion instead of the raw pseudo-version.
var pseudoVersionPattern = regexp.MustCompile(`-(?:0\.)?\d{14}-[0-9a-f]{12}`)

// isPseudoVersion reports whether v is a Go pseudo-version (development build).
func isPseudoVersion(v string) bool {
	return pseudoVersionPattern.MatchString(v)
}

// moduleVersion returns the main module version from Go build info, with the
// "v" prefix stripped. Returns "" for development builds ("(devel)" or
// pseudo-versions reported by the go tool for VCS builds).
func moduleVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	v := strings.TrimSpace(bi.Main.Version)
	if v == "" || v == "(devel)" || v == "devel" || isPseudoVersion(v) {
		return ""
	}
	return strings.TrimPrefix(v, "v")
}

// vcsRevision returns the short VCS revision embedded by the go tool when
// building inside a git checkout. Returns "" when absent (module download
// builds, -buildvcs=false, or non-VSC checkouts).
func vcsRevision() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range bi.Settings {
		if setting.Key != "vcs.revision" || setting.Value == "" {
			continue
		}
		return shortRevision(setting.Value)
	}
	return ""
}

// shortRevision truncates a full VCS revision to the conventional 7-char form.
func shortRevision(revision string) string {
	if len(revision) > 7 {
		return revision[:7]
	}
	return revision
}

// VersionString returns the effective semver of this build.
func VersionString() string {
	version, _ := resolve()
	return version
}

// Format returns the stable version payload for -v/--version and the version
// command. Shape: "apifox-api <semver> (commit <sha>)"
func Format() string {
	version, commit := resolve()
	return fmt.Sprintf("apifox-api %s (commit %s)", version, commit)
}
