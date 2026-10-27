package view

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const DefaultTimeout = 5 * time.Second

// ShellRunner executes command blocks with /bin/sh in a fixed environment.
type ShellRunner struct {
	env     []string
	timeout time.Duration
}

func NewShellRunner(env []string, timeout time.Duration) *ShellRunner {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &ShellRunner{env: env, timeout: timeout}
}

// CommandError reports a failed command block with the stderr it produced.
type CommandError struct {
	Command  string
	Stderr   string
	ExitCode int
	Err      error
}

func (e *CommandError) Error() string {
	message := fmt.Sprintf("command %q failed", e.Command)
	if e.ExitCode >= 0 {
		message += fmt.Sprintf(" (exit %d)", e.ExitCode)
	}
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		return message + ": " + stderr
	}
	if e.Err != nil {
		return message + ": " + e.Err.Error()
	}
	return message
}

func (e *CommandError) Unwrap() error { return e.Err }

func (r *ShellRunner) Run(ctx context.Context, command string) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "/bin/sh", "-c", command)
	cmd.Env = r.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}

	commandErr := &CommandError{Command: command, Stderr: stderr.String(), ExitCode: -1, Err: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		commandErr.ExitCode = exitErr.ExitCode()
	}
	if runCtx.Err() == context.DeadlineExceeded {
		commandErr.Err = fmt.Errorf("timed out after %s", r.timeout)
		commandErr.ExitCode = -1
	}
	return nil, commandErr
}
