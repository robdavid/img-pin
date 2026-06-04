package files

import (
	"errors"
	"strings"

	"github.com/robdavid/genutil-go/functions"
	"github.com/robdavid/genutil-go/slices"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var ErrCannotApplyAllPatches = errors.New("not all patches applied")

// PatchNonWhitespace turns the src text document into the dst document whilst preserving
// newline whitespace in the original. It does this, using https://github.com/sergi/go-diff
// to create a diff between the two files, removing diffs that are just whitespace containing
// newlines, and applying the set of diffs as a patch to the source document.
func PatchNonWhitespace(src, dst string) (result string, err error) {
	var applied []bool
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(src, dst, true)

	meaningful := slices.Filter(diffs, func(d diffmatchpatch.Diff) bool {
		return !(d.Type == diffmatchpatch.DiffDelete && strings.TrimSpace(d.Text) == "" && strings.Contains(d.Text, "\n"))
	})

	meaningful = dmp.DiffCleanupMerge(meaningful)
	text1 := dmp.DiffText1(meaningful)
	patches := dmp.PatchMake(text1, meaningful)
	result, applied = dmp.PatchApply(patches, src)
	if !slices.All(applied, functions.Id) {
		err = ErrCannotApplyAllPatches
	}
	return
}
