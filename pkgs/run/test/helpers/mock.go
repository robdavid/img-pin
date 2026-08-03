package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"

	"github.com/robdavid/img-pin/pkgs/run"
)

var ErrNoMatch = errors.New("unexpected command")

type CommandError struct {
	Text   string
	Stderr string
}

func (ce *CommandError) IsNil() bool { return ce == nil }

type CommandResponse struct {
	CmdAndArgs []string      `json:"cmdAndArgs"`
	Response   string        `json:"response"`
	Error      *CommandError `json:"error,omitempty"`
}

type CommandResponses []CommandResponse

func (crs *CommandResponses) Append(cr CommandResponse) { *crs = append(*crs, cr) }

type Testable interface {
	Cleanup(func())
	Name() string
	Fatal(args ...any)
	Helper()
}

type ArgEqual = func(args1, args2 []string, index int) bool

type MockState struct {
	Responses CommandResponses
	Expected  int
	EqualFunc ArgEqual
}

func (ms *MockState) Run(args ...string) ([]byte, error) {
	next := ms.Responses[ms.Expected]
	if len(args) != len(next.CmdAndArgs) {
		return nil, fmt.Errorf("%w %v; expected %v", ErrNoMatch, args, next.CmdAndArgs)
	}
	for i := range args {
		if (ms.EqualFunc == nil && args[i] != next.CmdAndArgs[i]) || !ms.EqualFunc(args, next.CmdAndArgs, i) {
			return nil, fmt.Errorf("%w %v; expected %v - mismatch at position %d", ErrNoMatch, args, next.CmdAndArgs, i)
		}
	}
	ms.Expected++
	if !next.Error.IsNil() {
		return nil, &run.RunError{Stderr: []byte(next.Error.Stderr), Err: fmt.Errorf("%s", next.Error.Text)}
	}
	return []byte(next.Response), nil
}

func (ms *MockState) LoadResponses(filename string) error {
	input, err := os.Open(filename)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(input)
	return decoder.Decode(&ms.Responses)
}

var reTempFile = regexp.MustCompile(`^.*/(.*)-[0-9]+\.(json|yaml)$`)

func ArgsCompare(args1, args2 []string, index int) bool {
	left := args1[index]
	right := args2[index]
	if index > 0 {
		if lmatch := reTempFile.FindStringSubmatch(left); lmatch != nil {
			if rmatch := reTempFile.FindStringSubmatch(right); rmatch != nil {
				if slices.Equal(lmatch[1:], rmatch[1:]) {
					return true
				}
			}
		}
	}
	return left == right
}

type ScriptMode int

const (
	ScriptModeRun ScriptMode = iota
	ScriptModeCapture
	ScriptModeEmpty
	ScriptModeAuto
)

type ScriptOpts struct {
	Mode       ScriptMode
	ArgCompare ArgEqual
	OutputFile string
}

func Script(t Testable, opts ScriptOpts) {
	t.Helper()
	file := mockFile(t, opts.OutputFile)
	mode := opts.Mode
	var cleanup func()
	for {
		ms := MockState{EqualFunc: opts.ArgCompare}
		switch mode {
		case ScriptModeAuto:
			_, err := os.Stat(file)
			if err == nil {
				mode = ScriptModeRun
			} else if errors.Is(err, os.ErrNotExist) {
				mode = ScriptModeCapture
			} else {
				panic(err)
			}
			continue
		case ScriptModeRun:
			fname := mockFile(t, opts.OutputFile)
			if err := ms.LoadResponses(fname); err != nil {
				t.Fatal(err)
			}
			fallthrough
		case ScriptModeEmpty:
			cleanup = run.SetRun(ms.Run)
		case ScriptModeCapture:
			cleanup = CaptureMain(file)
		}
		break
	}
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
}
