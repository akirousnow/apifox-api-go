package cli

import (
	"context"
	"errors"
)

// Process exit codes for the apifox-api CLI (agent-safe contract).
const (
	// ExitSuccess means the command completed and any payload is on stdout.
	ExitSuccess = 0
	// ExitFailure covers user/input/validation errors, remote hard failures,
	// missing binding/cache, and cooperative cancel (Ctrl-C / context cancel).
	ExitFailure = 1
)

// ExitCode maps a command error to a stable process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	return ExitFailure
}

// IsCancel reports whether err is context cancellation (Ctrl-C policy).
func IsCancel(err error) bool {
	return err != nil && errors.Is(err, context.Canceled)
}
