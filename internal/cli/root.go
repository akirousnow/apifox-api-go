// Package cli wires Cobra commands as a thin adapter over domain packages.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/akirousnow/apifox-api-go/internal/buildinfo"
	"github.com/akirousnow/apifox-api-go/internal/snapshot"
)

// Streams holds process I/O for the CLI. Tests inject buffers instead of os streams.
type Streams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Dependencies are process-level inputs assembled in main and passed into commands.
type Dependencies struct {
	Streams Streams
	// CWD is the process working directory used for Project Binding resolution.
	CWD string
	// HomeDir is the user home directory that holds ~/.apifox-api.json.
	HomeDir string
	// Env is a snapshot of environment variables (e.g. APIFOX_AUTH_KEY).
	Env map[string]string
	// FetchFunc optionally overrides remote OpenAPI export (tests inject fakes).
	// Production leaves this nil so the default HTTP client is used.
	FetchFunc snapshot.FetchFunc
}

// NewRoot constructs the root command tree. Callers must not share a single
// package-level rootCmd; each Execute gets a fresh tree for test isolation.
func NewRoot(dependencies Dependencies) *cobra.Command {
	rootCommand := &cobra.Command{
		Use:           "apifox-api",
		Short:         "Apifox OpenAPI CLI for Project Binding, search, and TypeScript generation",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCommand.SetIn(dependencies.Streams.In)
	rootCommand.SetOut(dependencies.Streams.Out)
	rootCommand.SetErr(dependencies.Streams.Err)

	rootCommand.Version = buildinfo.Version
	rootCommand.SetVersionTemplate(buildinfo.Format() + "\n")

	rootCommand.AddCommand(newVersionCommand(dependencies))
	rootCommand.AddCommand(newInitCommand(dependencies))
	rootCommand.AddCommand(newConfigCommand(dependencies))
	rootCommand.AddCommand(newModuleCommand(dependencies))
	rootCommand.AddCommand(newSearchCommand(dependencies))
	rootCommand.AddCommand(newSearchFieldsCommand(dependencies))
	rootCommand.AddCommand(newGetCommand(dependencies))
	rootCommand.AddCommand(newRefreshCommand(dependencies))

	return rootCommand
}

func newVersionCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the apifox-api version and commit",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(command.OutOrStdout(), buildinfo.Format())
			return err
		},
	}
}

// Execute runs the CLI with the given args under ctx. Args should not include the program name.
func Execute(ctx context.Context, dependencies Dependencies, args []string) error {
	rootCommand := NewRoot(dependencies)
	rootCommand.SetArgs(args)
	return rootCommand.ExecuteContext(ctx)
}
