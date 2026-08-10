package helpers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/robdavid/img-pin/pkgs/run"
)

// CaptureWrap wraps the run function, returning a function that unwraps it.
// It delegates to the underlying function whilst capturing the commands and their results and placing the results in crs.
func CaptureWrap(crs *CommandResponses) (cleanup func()) {
	wrapper := func(wrapped run.RunFunc) run.RunFunc {
		return func(args ...string) ([]byte, error) {
			var err error
			var response []byte
			cr := CommandResponse{CmdAndArgs: args}
			if response, err = wrapped(args...); err != nil {
				cr.Error = &CommandError{Text: err.Error()}
				var runError *run.RunError
				if errors.As(err, &runError) {
					cr.Error.Stderr = string(runError.Stderr)
				}
			} else {
				cr.Response = string(response)
			}
			crs.Append(cr)
			return response, err
		}
	}
	cleanup = run.WrapRun(wrapper)
	return
}

type CaptureState struct {
	Responses  CommandResponses
	OutputFile string
}

func (cs *CaptureState) WrapRun() (cleanup func()) { return CaptureWrap(&cs.Responses) }

func (cs *CaptureState) WriteOutput() error {
	dir := filepath.Dir(cs.OutputFile)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	output, err := os.Create(cs.OutputFile)
	if err != nil {
		return err
	}
	defer output.Close()
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cs.Responses)
}

func CaptureMain(outputFile string) (cleanup func()) {
	cs := CaptureState{OutputFile: outputFile}
	unwrap := cs.WrapRun()
	cleanup = func() {
		unwrap()
		if len(cs.Responses) > 0 {
			if err := cs.WriteOutput(); err != nil {
				panic(err)
			}
		}
	}
	return
}

func mockFile(t Testable, template string) string {
	return strings.Replace(template, "*", t.Name(), 1)
}

func Capture(t Testable, outputFile string) {
	t.Helper()
	t.Cleanup(CaptureMain(mockFile(t, outputFile)))
}
