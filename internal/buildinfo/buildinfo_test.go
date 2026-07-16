package buildinfo

import "testing"

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
