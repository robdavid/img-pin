package yaml_test

import (
	"testing"

	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const testText = `
nest:
  normalString: "Hello  "
  normalMultiline: "Hello  \nWorld\n"

  literalWithTrailing: |
    Hello
    World
`

func TestTrimMultiline(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	var root yaml.Node
	err := yaml.Unmarshal([]byte(testText), &root)
	require.NoError(err)
	assert.Equal("Hello  ", yu.Get[string](&root, "nest", "normalString").GetOr(""))
	assert.Equal("Hello  \nWorld\n", yu.Get[string](&root, "nest", "normalMultiline").GetOr(""))
	assert.Equal("Hello  \nWorld\n", yu.Get[string](&root, "nest", "literalWithTrailing").GetOr(""))
	yu.TrimMultiline(&root)
	assert.Equal("Hello  ", yu.Get[string](&root, "nest", "normalString").GetOr(""))
	assert.Equal("Hello  \nWorld\n", yu.Get[string](&root, "nest", "normalMultiline").GetOr(""))
	assert.Equal("Hello\nWorld\n", yu.Get[string](&root, "nest", "literalWithTrailing").GetOr(""))
}
