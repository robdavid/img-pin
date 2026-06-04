package ferrors

import (
	"fmt"
	"strings"
)

type joined []error

func (j joined) Error() string {

	var output strings.Builder
	for _, e := range j {
		fmt.Fprintf(&output, "\n  %s", strings.ReplaceAll(e.Error(), "\n", "\n  "))
	}
	return output.String()
}

func (j joined) Unwrap() []error { return j }

// Join creates an error that wraps the errors provided. Errors that are
// themselves the result of calling [Join] are coalesced into a single object.
// If all inputs are nil, the result is nil. If only one input is non-nil, that
// value is returned. Otherwise, errors are wrapped as described. When two or
// more wrapped errors are converted to a string, each error is formatted with
// leading line feeds and indentation (two spaces), with indentation stacking
// when wrapped errors are, in turn, wrapping other [Join]ed errors.
func Join(errs ...error) error {
	var acc error = nil
	for _, err := range errs {
		if err == nil {
			continue
		} else if acc == nil {
			acc = err
		} else if j, ok := acc.(joined); ok {
			if k, ok := err.(joined); ok {
				acc = append(j, k...)
			} else {
				acc = append(j, err)
			}
		} else if k, ok := err.(joined); ok {
			acc = append(joined{acc}, k...)
		} else {
			acc = joined{acc, err}
		}
	}
	return acc
}

// Split splits out the and error created by [Join] into its constituent errors.
// If the error was not created by [Join], the original error is returned
// as the only element.
func Split(err error) []error {
	if e, ok := err.(joined); ok {
		return e
	} else {
		return []error{err}
	}
}
