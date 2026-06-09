package yaml

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/img-pin/pkgs/files"
	"go.yaml.in/yaml/v4"
)

var ErrNotDocNode = errors.New("not a document node")

// GetTop takes a document node and returns the expected singleton top level node.
func GetTop(docNode *yaml.Node) (node *yaml.Node, err error) {
	if docNode.Kind != yaml.DocumentNode {
		err = ErrNotDocNode
	} else if len(docNode.Content) != 1 {
		err = fmt.Errorf("%w: was expecting a single child but found %d", ErrNotDocNode, len(docNode.Content))
	} else {
		node = docNode.Content[0]
	}
	return
}

func DocKind(docNode *yaml.Node) (result string, err error) {
	match, _ := MatchOne(docNode, PathOf("kind"))
	if match != nil {
		err = match.Decode(&result)
	}
	return
}

func ReadDocs(filePath string) ([]*yaml.Node, error) {
	var reader io.ReadCloser
	var err error
	if filePath == "-" {
		reader = os.Stdin
	} else {
		if reader, err = os.Open(filePath); err != nil {
			return nil, err
		}
		defer reader.Close()
	}
	return StreamDocsIn(reader)
}

func StreamDocsIn(reader io.Reader) ([]*yaml.Node, error) {
	var docs []*yaml.Node
	var err error
	decoder := yaml.NewDecoder(reader)
	for {
		var docNode yaml.Node
		err = decoder.Decode(&docNode)
		if err == io.EOF {
			break // Reached end of file, no more documents
		} else if err != nil {
			return docs, err
		}
		docs = append(docs, &docNode)
	}
	return docs, nil
}

func StreamDocsOut(writer io.Writer, docs ...*yaml.Node) error {
	encoder := yaml.NewEncoder(writer)
	encoder.SetIndent(2)
	for _, doc := range docs {
		err := encoder.Encode(doc)
		if err != nil {
			return err
		}
	}
	return nil
}

func PatchDocsOut(original string, writer io.Writer, docs ...*yaml.Node) error {
	var output strings.Builder
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	for _, doc := range docs {
		err := encoder.Encode(doc)
		if err != nil {
			return err
		}
	}
	target := output.String()
	patched, err := files.PatchNonWhitespace(original, target)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer, patched)
	return err
}

func WriteDocsOut(filename string, docs ...*yaml.Node) (err error) {
	defer Handle(func(e error) {
		err = fmt.Errorf("%s: %w", filename, e)
	})
	if filename == "-" {
		return StreamDocsOut(os.Stdout, docs...)
	}
	output := Try(files.OpenForOverwrite(filename))
	defer output.Close()
	orig := Try(os.Open(filename))
	defer orig.Close()
	Check(SyncWrite(orig, output, docs...))
	Check(orig.Close())
	output.AllowOverwrite(true)
	return
}
