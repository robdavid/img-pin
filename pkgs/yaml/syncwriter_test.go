package yaml_test

import (
	"io"
	"strings"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
)

var singleDocument = `
one: 1 # This is one
two:
  three: 23 # two followed by three is 23 right?

billion: 1000000000

pets:
  - dog
`

var twoDocuments = `
description: "This is doc 1"
one: 1 # This is one
two:
  three: 23 # two followed by three is 23 right?

billion: 1000000000

pets:
  - dog

---

description: "This is doc 2"

# Not sure what this is for.
# Not for Star Wars
captains:
  - Picard
  - Janeway
  - Luca
  - Saru
`

var threeDocuments = `
description: "This is doc 1"
one: 1 # This is one
two:
  three: 23 # two followed by three is 23 right?

billion: 1000000000

pets:
  - dog

---

description: "This is doc 2"

# Not sure what this is for.
# Not for Star Wars
captains:
  - Picard
  - Janeway
  - Luca
  - Saru
---
description: "Oh, OK, this time Star Wars"

sith:
  - Vader
  - Sidius
  - Maul
  - Doku

# What's this doing here?
---

`

func TestSingleDocument(t *testing.T) {
	defer test.ReportErr(t)
	original := strings.NewReader(singleDocument)
	var output strings.Builder
	nodes := Try(yu.StreamDocsIn(original))
	assert.Equal(t, 1, len(nodes))
	original.Seek(0, io.SeekStart)
	yu.SyncWrite(original, &output, nodes...)
	assert.Equal(t, singleDocument, output.String())
}

func TestSingleDocumentModified(t *testing.T) {
	defer test.ReportErr(t)
	original := strings.NewReader(singleDocument)
	var output strings.Builder
	nodes := Try(yu.StreamDocsIn(original))
	yu.Put(nodes[0], 32, "two", "three")
	assert.Equal(t, 1, len(nodes))
	original.Seek(0, io.SeekStart)
	yu.SyncWrite(original, &output, nodes...)
	assert.Equal(t, singleDocument, strings.Replace(output.String(), "three: 32", "three: 23", 1))
}

func TestTwoDocuments(t *testing.T) {
	defer test.ReportErr(t)
	original := strings.NewReader(twoDocuments)
	var output strings.Builder
	nodes := Try(yu.StreamDocsIn(original))
	assert.Equal(t, 2, len(nodes))
	original.Seek(0, io.SeekStart)
	yu.SyncWrite(original, &output, nodes...)
	assert.Equal(t, twoDocuments, output.String())
}

func TestThreeDocuments(t *testing.T) {
	defer test.ReportErr(t)
	original := strings.NewReader(threeDocuments)
	var output strings.Builder
	nodes := Try(yu.StreamDocsIn(original))
	assert.Equal(t, 4, len(nodes)) // Forth empty doc
	original.Seek(0, io.SeekStart)
	yu.SyncWrite(original, &output, nodes...)
	assert.Equal(t, threeDocuments, output.String())
}
