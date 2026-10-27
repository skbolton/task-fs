package view

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestShellRunnerSuccess(t *testing.T) {
	runner := NewShellRunner(nil, time.Second)
	out, err := runner.Run(context.Background(), `printf '2026-11-01\n'`)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if string(out) != "2026-11-01\n" {
		t.Errorf("output = %q, want %q", out, "2026-11-01\n")
	}
}

func TestShellRunnerInheritsEnvironment(t *testing.T) {
	runner := NewShellRunner([]string{"TASKFS_TEST_VALUE=hello"}, time.Second)
	out, err := runner.Run(context.Background(), `printf %s "$TASKFS_TEST_VALUE"`)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if string(out) != "hello" {
		t.Errorf("output = %q, want %q", out, "hello")
	}
}

func TestShellRunnerFailureIncludesStderr(t *testing.T) {
	runner := NewShellRunner(nil, time.Second)
	_, err := runner.Run(context.Background(), `printf 'boom\n' >&2; exit 7`)
	if err == nil {
		t.Fatal("Run() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %q, want stderr", err)
	}
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("error = %T, want *CommandError", err)
	}
	if commandErr.ExitCode != 7 {
		t.Errorf("exit code = %d, want 7", commandErr.ExitCode)
	}
	if !strings.Contains(commandErr.Stderr, "boom") {
		t.Errorf("stderr = %q, want boom", commandErr.Stderr)
	}
}

func TestShellRunnerTimeout(t *testing.T) {
	runner := NewShellRunner(nil, 20*time.Millisecond)
	start := time.Now()
	_, err := runner.Run(context.Background(), `sleep 1`)
	if err == nil {
		t.Fatal("Run() succeeded, want timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want timeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Run() took %s, want prompt timeout", elapsed)
	}
}
