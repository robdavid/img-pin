package helpers

import (
	"encoding/json"
	"os"

	"github.com/robdavid/img-pin/pkgs/run"
)

type CommandError string

func (ce *CommandError) IsNil() bool { return *ce == "" }

type CommandResponse struct {
	CmdAndArgs []string     `json:"cmdAndArgs"`
	Response   string       `json:"response"`
	Error      CommandError `json:"error"`
}

type CommandResponses []CommandResponse

func (crs *CommandResponses) Append(cr CommandResponse) { *crs = append(*crs, cr) }

// CaptureWrap wraps the run function, returning a function that unwraps it.
// It delegates to the underlying function whilst capturing the commands and their results and placing the results in crs.
func CaptureWrap(crs *CommandResponses) (cleanup func()) {
	wrapper := func(wrapped run.RunFunc) run.RunFunc {
		return func(args ...string) ([]byte, error) {
			var err error
			var response []byte
			cr := CommandResponse{CmdAndArgs: args}
			if response, err = wrapped(args...); err != nil {
				cr.Error = CommandError(err.Error())
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
	output, err := os.Create(cs.OutputFile)
	if err != nil {
		return err
	}
	defer output.Close()
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cs.Responses)
}

func CaptureMainTo(outputFile string) (cleanup func()) {
	cs := CaptureState{OutputFile: outputFile}
	unwrap := cs.WrapRun()
	cleanup = func() {
		unwrap()
		if err := cs.WriteOutput(); err != nil {
			panic(err)
		}
	}
	return
}
