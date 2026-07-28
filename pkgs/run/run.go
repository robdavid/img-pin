package run

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
)

// RunError represents an error executing a command. It contains the underlying error, and
// any standard error output.
type RunError struct {
	Stderr []byte // Stderr contains any command output standard error.
	Err    error  // Err is the underlying error.
}

func (e *RunError) Error() string {
	return fmt.Sprintf("command failed: %s, %s", e.Stderr, e.Err)
}

func (e *RunError) Unwrap() error {
	return e.Err
}

// RunFunc is the type signature of the [Run] function.
type RunFunc = func(args ...string) ([]byte, error)

var runDispatch RunFunc = run

// Run executes a system command with the command and arguments in args. It returns the output
// of the command or an error. If there is an error it is of type [RunError] which captures
// the underlying error and any stderr output.
func Run(args ...string) ([]byte, error) {
	return runDispatch(args...)
}

func run(args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("run: no command")
	}
	cmd := exec.Command(args[0], args[1:]...)
	slog.Debug("running '{{.cmd}}' with {{.args}}", "cmd", args[0], "args", args[1:])
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), &RunError{Stderr: stderr.Bytes(), Err: err}
	}
	return stdout.Bytes(), nil
}
