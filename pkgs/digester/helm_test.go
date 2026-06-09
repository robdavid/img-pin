package digester_test

import (
	"os"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

func TestValuesSearch(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	var values yaml.Node
	valuesText := Try(os.ReadFile("tests/test_values.yaml"))
	Check(yaml.Unmarshal(valuesText, &values))
	hp := digester.HelmProcessor{}
	hp.AddValuesSpecificImages(&values)
	assert.Equal(6, len(hp.Images))
	actual := slices.Map(hp.Images,
		func(i digester.ImageDetails) string { return i.String() })
	assert.Contains(actual, "docker.io/ubuntu:20.04")
	assert.Contains(actual, "ubuntu:22.04")
	assert.Contains(actual, "public.ecr.aws/docker/library/vault:1.13.3")
	assert.Contains(actual, "docker.io/busybox:1.0")
	assert.Contains(actual, "docker.io/apline:3.22.4")
	assert.Contains(actual, "nginx:1.30-apline")
}
