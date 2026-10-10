package buildinfo

import (
	"strings"
	"testing"
)

func TestFormatDefault(t *testing.T) {
	t.Parallel()

	originalVersion := Version
	originalCommit := Commit
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
	})

	Version = "dev"
	Commit = "unknown"

	got := Format()
	want := "apifox-api dev (commit unknown)"
	if got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}

func TestFormatInjectedValues(t *testing.T) {
	t.Parallel()

	originalVersion := Version
	originalCommit := Commit
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
	})

	Version = "0.1.0"
	Commit = "deadbeef"

	got := Format()
	want := "apifox-api 0.1.0 (commit deadbeef)"
	if got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}

// TestFormatExplicitDevPreserved ensures an explicit "dev"/"unknown" pair set
// by tests or tooling is reported verbatim, not re-resolved.
func TestFormatExplicitDevPreserved(t *testing.T) {
	t.Parallel()

	originalVersion := Version
	originalCommit := Commit
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
	})

	Version = "dev"
	Commit = "unknown"

	if got, want := Format(), "apifox-api dev (commit unknown)"; got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}

// TestFormatUnsetFallsBack verifies that empty ldflags vars resolve via Go
// build info / fallbackVersion instead of rendering empty. The commit part is
// environment-dependent (vcs.revision only exists inside git builds), so only
// the stable prefix/suffix is asserted.
func TestFormatUnsetFallsBack(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
	})

	Version = ""
	Commit = ""

	got := Format()
	if !strings.HasPrefix(got, "apifox-api "+fallbackVersion+" (commit ") {
		t.Fatalf("Format() = %q, want prefix %q", got, "apifox-api "+fallbackVersion+" (commit ")
	}
	if !strings.HasSuffix(got, ")") {
		t.Fatalf("Format() = %q, want closing paren suffix", got)
	}
	if got == "apifox-api "+fallbackVersion+" (commit )" {
		t.Fatalf("Format() = %q, commit must not be empty", got)
	}
}

func TestModuleVersionStripsPrefixAndSkipsDevel(t *testing.T) {
	t.Parallel()

	// moduleVersion is a thin wrapper over debug.ReadBuildInfo; assert its
	// normalization contract through the exported VersionString path instead.
	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })

	Version = ""
	// In a `go test` binary Main.Version is empty or "(devel)", so the
	// effective version must be the fallback, never "(devel)" or empty.
	if got := VersionString(); got == "" || got == "(devel)" || strings.HasPrefix(got, "v") {
		t.Fatalf("VersionString() = %q, want non-empty, non-devel, non-v-prefixed fallback", got)
	}
}

func TestIsPseudoVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    string
		want bool
	}{
		{name: "empty", v: "", want: false},
		{name: "plain semver", v: "0.1.4", want: false},
		{name: "v-prefixed semver", v: "v0.1.4", want: false},
		{name: "devel", v: "(devel)", want: false},
		{name: "pseudo no base tag", v: "v0.0.0-20260824101034-a0d42c8a5eb3", want: true},
		{name: "pseudo with base tag", v: "v0.1.1-0.20260824101034-a0d42c8a5eb3", want: true},
		{name: "pseudo dirty", v: "v0.1.1-0.20260824101034-a0d42c8a5eb3+dirty", want: true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := isPseudoVersion(testCase.v); got != testCase.want {
				t.Fatalf("isPseudoVersion(%q) = %v, want %v", testCase.v, got, testCase.want)
			}
		})
	}
}

func TestShortRevision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rev  string
		want string
	}{
		{name: "empty", rev: "", want: ""},
		{name: "short kept", rev: "abc1234", want: "abc1234"},
		{name: "long truncated", rev: "abcdef1234567890abcdef1234567890abcdef12", want: "abcdef1"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := shortRevision(testCase.rev); got != testCase.want {
				t.Fatalf("shortRevision(%q) = %q, want %q", testCase.rev, got, testCase.want)
			}
		})
	}
}
