package helpers

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
)

func CopyToTemp(t *testing.T, filename string) string {
	defer Handle(func(e error) {
		t.Skipf("Cannot create temp file for %s: %s", filename, e)
	})
	src := Try(os.Open(filename))
	defer src.Close()
	dir := filepath.Dir(filename)
	name := filepath.Base(filename)
	ext := filepath.Ext(name)
	dst := Try(os.CreateTemp(dir, name[:len(name)-len(ext)]+"-*.tmp"+ext))
	defer dst.Close()
	Try(io.Copy(dst, src))
	return dst.Name()
}
