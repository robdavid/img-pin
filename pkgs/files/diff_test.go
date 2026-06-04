package files_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/img-pin/pkgs/files"
	"github.com/stretchr/testify/assert"
)

func TestPatchingFile(t *testing.T) {
	defer test.ReportErr(t)
	fmt.Println(Try(os.Getwd()))
	original := string(Try(os.ReadFile("tests/original.txt")))
	updated := string(Try(os.ReadFile("tests/updated.txt")))
	expected := string(Try(os.ReadFile("tests/expected.txt")))
	actual := Try(files.PatchNonWhitespace(original, updated))
	assert.Equal(t, expected, actual)
}
