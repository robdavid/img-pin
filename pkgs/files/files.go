package files

import (
	"os"
	"path/filepath"
)

type SafeOverwriter struct {
	filename  string
	overwrite bool
	fh        *os.File
}

// OpenForOverwrite notionally opens a file for writing. In reality,
// it actually opens a temporary file in the same directory. Once
// [SafeOverwriter.AllowOverwrite] is called with a value of [true],
// closing the *[SafeOverwriter] handle will cause the named file to be
// replaced by the temporary file. If the file is closed without calling
// this method, it is simply deleted.
func OpenForOverwrite(filename string) (*SafeOverwriter, error) {
	var err error
	var so SafeOverwriter
	var abs string
	so.filename = filename
	if abs, err = filepath.Abs(filename); err != nil {
		return nil, err
	}
	parent := filepath.Dir(abs)
	so.fh, err = os.CreateTemp(parent, filepath.Base(abs)+".*.tmp")
	return &so, nil
}

func (so *SafeOverwriter) Write(bytes []byte) (int, error) {
	n, err := so.fh.Write(bytes)
	if err != nil {
		so.overwrite = false
	}
	return n, err
}

// AllowOverwrite is used to indicate whether it is considered safe to
// replace the named file with the new content.
func (so *SafeOverwriter) AllowOverwrite(overwrite bool) {
	so.overwrite = overwrite
}

// Overwrite closes the file and causes the named file to be replaced by the
// new content.
func (so *SafeOverwriter) Overwrite() error {
	so.overwrite = true
	return so.Close()
}

// Close closes the output file and depending on whether [SafeOverwriter.AllowOverwrite]
// has been enabled it is either deleted (overwrite not allowed), or it is renamed to
// replace the named file.
func (so *SafeOverwriter) Close() (err error) {
	if so.fh != nil {
		if err = so.fh.Close(); err != nil || !so.overwrite {
			err = os.Remove(so.fh.Name())
		} else {
			err = os.Rename(so.fh.Name(), so.filename)
		}
		so.fh = nil
	}
	return err
}
