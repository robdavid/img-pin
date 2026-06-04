package ferrors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/robdavid/img-pin/pkgs/ferrors"
)

var (
	err1 = errors.New("Error 1")
	err2 = errors.New("Error 2")
	err3 = errors.New("Error 3")
	err4 = errors.New("Error 4")
	err5 = errors.New("Error 5")
	err6 = errors.New("Error 6")
)

func TestJoin1(t *testing.T) {
	e := fmt.Errorf("%w: %w", err1, ferrors.Join(err2))
	fmt.Printf("%s: %v", "error is", e)
}

func TestDepth1(t *testing.T) {
	e := fmt.Errorf("%w: %w", err1, ferrors.Join(err2, err3))
	fmt.Printf("%s: %v", "error is", e)
}

func TestDepth2(t *testing.T) {
	j1 := ferrors.Join(err2, err3)
	j2 := ferrors.Join(err4, err5)
	e := fmt.Errorf("%w: %w", err1, ferrors.Join(j1, j2))
	fmt.Printf("%s: %v", "error is", e)
}

func TestDoubleWrap(t *testing.T) {
	j1 := ferrors.Join(err2, err3)
	j2 := ferrors.Join(err4, err5)
	e := fmt.Errorf("%w: %w %w", err1, j1, j2)
	fmt.Printf("%s: %v", "error is", e)
}

func TestDepth2Wrapped(t *testing.T) {
	j1 := ferrors.Join(err2, err3)
	j2 := ferrors.Join(err4, err5)
	e1 := fmt.Errorf("j1 is %w", j1)
	e2 := fmt.Errorf("j2 is %w", j2)
	e := fmt.Errorf("%w: %w", err1, ferrors.Join(e1, e2))
	fmt.Printf("%s: %v", "error is", e)
}

func TestDepth2WDoublerapped(t *testing.T) {
	j1 := ferrors.Join(err2, err3)
	j2 := ferrors.Join(err4, err5)
	e1 := fmt.Errorf("j1 is %w", j1)
	e2 := fmt.Errorf("j2 is %w", j2)
	e := fmt.Errorf("%w: %w %w", err1, e1, e2)
	fmt.Printf("%s: %v", "error is", e)
}
