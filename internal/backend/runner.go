package backend

import (
	"fmt"
	"os/exec"
)

// CommandRunner executes the taskwarrior CLI with the mount process
// environment so TASKDATA-scoped databases are addressed correctly.
type CommandRunner interface {
	Run(args []string) ([]byte, error)
}

type execRunner struct {
	exe string
	env []string
}

func NewRunner(exe string, env []string) CommandRunner {
	return &execRunner{exe: exe, env: env}
}

func (r *execRunner) Run(args []string) ([]byte, error) {
	cmd := exec.Command(r.exe, args...)
	cmd.Env = r.env
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.Error); ok {
			return nil, fmt.Errorf("required program %q not found on PATH", r.exe)
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s %v failed: %s", r.exe, args, ee.Stderr)
		}
		return nil, err
	}
	return out, nil
}
