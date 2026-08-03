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
	name := filepath.Base(filename)
	ext := filepath.Ext(name)
	dst := Try(os.CreateTemp("", name[:len(name)-len(ext)]+"-*.tmp"+ext))
	defer dst.Close()
	t.Cleanup(func() { os.Remove(dst.Name()) })
	Try(io.Copy(dst, src))
	return dst.Name()
}

type TempDir struct {
	Dir   string
	Files []string
}

func (td *TempDir) Delete() error { return os.RemoveAll(td.Dir) }
func (fd *TempDir) First() string { return fd.Files[0] }

// CopyToTempDir creates a temporary directory and copies the provided files to that directory
// a [TempDir] object is returned that contains details about the directory and files. It adds
// a cleanup handler to t to remove the temporary directory after the test.
func CopyToTempDir(t *testing.T, filenames ...string) (tmpDir TempDir) {
	t.Helper()
	defer Handle(func(e error) {
		t.Skipf("Cannot create temp file for %v: %s", filenames, e)
	})
	if len(filenames) == 0 {
		t.Fatal("No file names supplied to CopyToTemp")
	}
	tmpDir.Dir = Try(os.MkdirTemp("", "tmp-*"))
	t.Cleanup(func() { tmpDir.Delete() })
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
