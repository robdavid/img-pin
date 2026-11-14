package run

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
)

type RunOptions struct {
	Args       []string
	WorkingDir string
}

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
type RunFunc = func(options RunOptions) ([]byte, error)

var runDispatch RunFunc = run

// RunInDirFunc is the type signature of the [Run] function.
type RunInDirFunc = func(workingDir string, args ...string) ([]byte, error)

// Run executes a system command with the command and arguments in args. It returns the output
// of the command or an error. If there is an error it is of type [RunError] which captures
// the underlying error and any stderr output.
func Run(args ...string) ([]byte, error) {
	return runDispatch(RunOptions{
		Args: args,
	})
}

func run(options RunOptions) ([]byte, error) {
	if len(options.Args) == 0 {
		return nil, fmt.Errorf("run: no command")
	}
	cmd := exec.Command(options.Args[0], options.Args[1:]...)
	workingDirectory := "."
	if options.WorkingDir != "" {
		workingDirectory = options.WorkingDir
		cmd.Dir = workingDirectory
	}
	slog.Debug("running} '{{.cmd}}' with {{.args}}", "cmd", options.Args[0], "args", options.Args[1:], "WorkingDir", workingDirectory)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), &RunError{Stderr: stderr.Bytes(), Err: err}
	}
	return stdout.Bytes(), nil
}

// RunInDir executes a system command with the command and arguments in args in the specified working directory.
//
//	It returns the output of the command or an error. If there is an error it is of type [RunError] which captures
//
// the underlying error and any stderr output.
func RunInDir(workingDir string, args ...string) ([]byte, error) {
	return runDispatch(RunOptions{
		Args:       args,
		WorkingDir: workingDir,
	})
}
