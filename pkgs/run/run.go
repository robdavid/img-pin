package run

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
)

type RunError struct {
	Stderr []byte
	Err    error
}

func (e *RunError) Error() string {
	return fmt.Sprintf("command failed: %s, %s", e.Stderr, e.Err)
}

func (e *RunError) Unwrap() error {
	return e.Err
}

func Run(args ...string) ([]byte, error) {
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
