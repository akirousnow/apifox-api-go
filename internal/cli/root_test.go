package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/akirousnow/apifox-api-go/internal/buildinfo"
	"github.com/akirousnow/apifox-api-go/internal/cli"
)

func TestVersionFlagAndSubcommandDefault(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		args []string
	}{
		{name: "long_flag", args: []string{"--version"}},
		{name: "subcommand", args: []string{"version"}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			err := cli.Execute(
				context.Background(),
				cli.Dependencies{
					Streams: cli.Streams{
						In:  strings.NewReader(""),
						Out: &stdout,
						Err: &stderr,
					},
				},
				testCase.args,
			)
			if err != nil {
				t.Fatalf("Execute(%v) error: %v\nstderr=%q", testCase.args, err, stderr.String())
			}

			got := strings.TrimSpace(stdout.String())
			want := buildinfo.Format()
			if got != want {
				t.Fatalf("stdout = %q, want %q\nstderr=%q", got, want, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr should be empty, got %q", stderr.String())
			}
		})
	}
}

func TestVersionReflectsBuildinfoOverrides(t *testing.T) {
	// buildinfo package vars are process-global; do not parallelize mutations.
	originalVersion := buildinfo.Version
	originalCommit := buildinfo.Commit
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
		buildinfo.Commit = originalCommit
	})

	buildinfo.Version = "0.1.0-test"
	buildinfo.Commit = "cafebabe"

	var stdout bytes.Buffer
	err := cli.Execute(
		context.Background(),
		cli.Dependencies{
			Streams: cli.Streams{
				In:  strings.NewReader(""),
				Out: &stdout,
				Err: &bytes.Buffer{},
			},
		},
		[]string{"version"},
	)
	if err != nil {
		t.Fatalf("Execute(version) error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "apifox-api 0.1.0-test (commit cafebabe)"
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
