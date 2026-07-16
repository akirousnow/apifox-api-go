package cli

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestExitCode(t *testing.T) {
	t.Parallel()
	if got := ExitCode(nil); got != ExitSuccess {
		t.Fatalf("ExitCode(nil) = %d, want %d", got, ExitSuccess)
	}
	if got := ExitCode(errors.New("boom")); got != ExitFailure {
		t.Fatalf("ExitCode(err) = %d, want %d", got, ExitFailure)
	}
	if got := ExitCode(fmt.Errorf("wrap: %w", context.Canceled)); got != ExitFailure {
		t.Fatalf("ExitCode(cancel) = %d, want %d", got, ExitFailure)
	}
}

func TestIsCancel(t *testing.T) {
	t.Parallel()
	if IsCancel(nil) {
		t.Fatal("IsCancel(nil) should be false")
	}
	if IsCancel(errors.New("nope")) {
		t.Fatal("IsCancel(plain) should be false")
	}
	if !IsCancel(context.Canceled) {
		t.Fatal("IsCancel(context.Canceled) should be true")
	}
	if !IsCancel(fmt.Errorf("wrap: %w", context.Canceled)) {
		t.Fatal("IsCancel(wrapped cancel) should be true")
	}
}
