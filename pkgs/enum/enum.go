package enum

import (
	"errors"
	"fmt"
	"strings"
)

var ErrNotValid = errors.New("not a valid enum value")

type StringableInt interface {
	String() string
	~int
}

// FromString converts a string to an enum type (that is a type based on an int
// that implements [fmt.Stringer]).
func FromString[T StringableInt](name string) (result T, err error) {
	for {
		if str := result.String(); str == name {
			return
		} else {
			suffix := fmt.Sprintf("(%d)", result)
			if strings.HasSuffix(str, suffix) {
				return -1, fmt.Errorf("%w %q for %T (valid choices are %v)",
					ErrNotValid, name, result, strings.Join(AllStrings[T](), ", "))
			}
		}
		result++
	}
}

// AllStrings returns a list of strings for a countable enum type (that is a
// type based on an int that implements [fmt.Stringer]).
func AllStrings[T StringableInt]() (result []string) {
	var value T
	for {
		str := value.String()
		suffix := fmt.Sprintf("(%d)", value)
		if strings.HasSuffix(str, suffix) {
			return
		}
		result = append(result, str)
		value++
	}
}
