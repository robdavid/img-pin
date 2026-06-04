package yaml_test

import (
	"os"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestSimpleStylesCompare(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	styles := Try(os.ReadFile("tests/string-styles.yaml"))
	styles2 := Try(os.ReadFile("tests/string-styles2.yaml"))
	var node1, node2 yaml.Node
	Check(yaml.Unmarshal(styles, &node1))
	Check(yaml.Unmarshal(styles2, &node2))
	assert.True(yu.EqualData(&node1, &node1))
	assert.False(yu.EqualData(&node1, &node2))
}

func TestFoldedStylesRoundTrip(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	styles := Try(os.ReadFile("tests/string-styles.yaml"))
	var node1, node2 yaml.Node
	Check(yaml.Unmarshal(styles, &node1))
	output := Try(yaml.Marshal(&node1))
	Check(yaml.Unmarshal(output, &node2))
	assert.True(yu.EqualData(&node1, &node2))
}
