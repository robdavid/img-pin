package yaml

import (
	"bufio"
	"io"
	"strings"

	. "github.com/robdavid/genutil-go/errors/handler"
	"gopkg.in/yaml.v3"
)

// SyncWrite is an attempt to produce output YAML with blank lines in the same
// places as the original (or something close). Don't try this at home. As it
// writes out the new file, it simultaneously reads the original and compares
// line by line. When they are in sync, and then a blank line is encountered in
// the the original, a corresponding ng blank line is written to the output.
// Otherwise, lines are written from the YAML encoder as usual. Comments are
// preserved because YAMLv3 AST trees have that information.
func SyncWrite(original io.Reader, output io.Writer, modified ...*yaml.Node) (err error) {
	defer Catch(&err)
	yin, yout := io.Pipe()
	encoder := yaml.NewEncoder(yout)
	encoder.SetIndent(2)
	outErr := make(chan error, 1)
	go func() {
		defer yout.Close()
		for _, node := range modified {
			if err := encoder.Encode(node); err != nil {
				outErr <- err
				return
			}
		}
		outErr <- nil
	}()
	// Make sure we don't block the go routine from ending by making sure
	// we read the channel whatever.
	// Note this is *exploiting* the fact that IfElse doesn't short circuit.
	// defer func() { err = functions.IfElse(err == nil, <-outErr, err) }()
	defer yin.Close()
	orig := bufio.NewScanner(original)
	modi := bufio.NewScanner(yin)

	oEOF := !orig.Scan()
	mEOF := !modi.Scan()
	synced := true
	for !mEOF {
		if oEOF {
			Check(orig.Err())
			writeBytesLn(output, modi.Bytes())
			mEOF = !modi.Scan()
		} else {
			oText := strings.TrimSpace(orig.Text())
			mText := strings.TrimSpace(modi.Text())
			if oText == mText {
				// In Sync, write, move forward both
				synced = true
				writeBytesLn(output, modi.Bytes())
				mEOF = !modi.Scan()
				oEOF = !orig.Scan()
			} else if oText == "" && synced {
				// Newline in original
				// Move forward in original
				// Newline will be added in modified (below)
				writeBytesLn(output, nil)
				oEOF = !orig.Scan()
			} else {
				// We are out of sync, maybe the value changed only
				oCol := strings.Index(oText, ":")
				mCol := strings.Index(mText, ":")
				if (oCol > 0 && mCol > 0 && oText[:oCol] == mText[:mCol]) ||
					(strings.HasPrefix(oText, "- ") && strings.HasPrefix(mText, "- ")) {
					oEOF = !orig.Scan()
				} else {
					synced = false
				}
				writeBytesLn(output, modi.Bytes())
				mEOF = !modi.Scan()
			}
		}
	}
	Check(modi.Err())
	Check(<-outErr)
	return
}

var newLineBytes []byte = []byte{'\n'}

func writeBytesLn(output io.Writer, b []byte) {
	Try(output.Write(b))
	Try(output.Write(newLineBytes))
}
