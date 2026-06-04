package run_test

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/robdavid/img-pin/pkgs/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSuccess(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	out, err := run.Run("echo", "hello")
	require.NoError(err)
	assert.Equal([]byte("hello\n"), out)
}

func TestRunMultipleArgs(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	out, err := run.Run("echo", "hello", "world")
	require.NoError(err)
	assert.Equal([]byte("hello world\n"), out)
}

func TestRunExitErrorCapturesStderr(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	out, err := run.Run("sh", "-c", "echo hello from stderr >&2; echo hello from stdout; exit 1")
	require.Error(err)
	assert.Equal([]byte("hello from stdout\n"), out)
	var runErr *run.RunError
	require.ErrorAs(err, &runErr)
	assert.Equal([]byte("hello from stderr\n"), runErr.Stderr)
	var exitErr *exec.ExitError
	require.ErrorAs(err, &exitErr)
}

func TestRunNoArgs(t *testing.T) {
	assert := assert.New(t)
	out, err := run.Run()
	assert.Error(err)
	assert.Empty(out)
	assert.Contains(err.Error(), "no command")
}

func TestRunCommandNotFound(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	out, err := run.Run("nonexistent_command_xyz")
	require.Error(err)
	assert.Empty(out)
	var runErr *run.RunError
	require.ErrorAs(err, &runErr)
	assert.Empty(runErr.Stderr)
	var pathErr *exec.Error
	require.ErrorAs(err, &pathErr)
	assert.Equal("nonexistent_command_xyz", pathErr.Name)
}

func TestRunSuccessNoOutput(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	out, err := run.Run("true")
	require.NoError(err)
	assert.Empty(out)
}

func TestRunExitErrorNoStdout(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	out, err := run.Run("sh", "-c", "exit 42")
	require.Error(err)
	assert.Empty(out)
	var exitErr *exec.ExitError
	require.ErrorAs(err, &exitErr)
	assert.Equal(42, exitErr.ExitCode())
}

var errSentinel = errors.New("something went wrong")

func TestRunErrorImplementsError(t *testing.T) {
	assert := assert.New(t)
	err := &run.RunError{Stderr: []byte("stderr content"), Err: errSentinel}
	assert.Equal("command failed: stderr content, something went wrong", err.Error())
	assert.ErrorIs(err, errSentinel)
}

func TestRunErrorUnwrap(t *testing.T) {
	assert := assert.New(t)
	base := errors.New("base error")
	err := &run.RunError{Err: base}
	assert.True(errors.Is(err, base))
}

func TestRunStdoutOnError(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	out, err := run.Run("sh", "-c", "echo stdout data; echo stderr data >&2; exit 1")
	require.Error(err)
	assert.Equal([]byte("stdout data\n"), out)
}
