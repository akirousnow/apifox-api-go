package cli

import (
	"context"
	"fmt"
)

// Run executes the CLI and returns a process exit code.
// Errors and cancel messages are written to dependencies.Streams.Err only.
// Success payloads are written by commands to Streams.Out.
func Run(ctx context.Context, dependencies Dependencies, args []string) int {
	err := Execute(ctx, dependencies, args)
	if err == nil {
		return ExitSuccess
	}

	errWriter := dependencies.Streams.Err
	if errWriter == nil {
		return ExitFailure
	}

	if IsCancel(err) {
		fmt.Fprintln(errWriter, "interrupted")
		return ExitFailure
	}

	fmt.Fprintln(errWriter, err.Error())
	return ExitFailure
}
