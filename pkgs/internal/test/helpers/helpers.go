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

type TempDir struct {
	Dir   string
	Files []string
}

func (td *TempDir) Delete() error { return os.RemoveAll(td.Dir) }
func (fd *TempDir) First() string { return fd.Files[0] }

func CopyToTempDir(t *testing.T, filenames ...string) (tmpDir TempDir) {
	t.Helper()
	defer Handle(func(e error) {
		t.Skipf("Cannot create temp file for %v: %s", filenames, e)
	})
	if len(filenames) == 0 {
		t.Fatal("No file names supplied to CopyToTemp")
	}
	dir := filepath.Dir(filenames[0])
	tmpDir.Dir = Try(os.MkdirTemp(dir, "tmp-*"))
	for _, filename := range filenames {
		src := Try(os.Open(filename))
		defer src.Close()
		name := filepath.Base(filename)
		path := filepath.Join(tmpDir.Dir, name)
		dst := Try(os.Create(path))
		defer dst.Close()
		Try(io.Copy(dst, src))
		tmpDir.Files = append(tmpDir.Files, path)
	}
	return
}
