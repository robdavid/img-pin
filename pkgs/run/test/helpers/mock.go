package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

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

var reTempFile = regexp.MustCompile(`^/tmp/(.*)-[0-9]+.(json|yaml)$`)

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

func Script(t Testable, filename string, compare ArgEqual) {
	ms := MockState{EqualFunc: compare}
	fname := strings.Replace(filename, "*", t.Name(), 1)
	if err := ms.LoadResponses(fname); err != nil {
		t.Fatal(err)
	}
	cleanup := run.SetRun(ms.Run)
	t.Cleanup(cleanup)
}

func ScriptTmpArgs(t Testable, filename string) {
	Script(t, filename, ArgsCompare)
}
