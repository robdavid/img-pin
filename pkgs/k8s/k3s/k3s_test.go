package k3s_test

import (
	"fmt"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/robdavid/img-pin/pkgs/k8s/k3s"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noSkipOptions []images.ImageOption

// SkipOnPolicy returns true if the policy skip error should be honored
func (s noSkipOptions) SkipOnPolicy(image *images.Image) bool {
	return false
}

// SkipV1Schema returns true if images with V1 schemas should be skipped.
func (s noSkipOptions) SkipV1Schema(image *images.Image) bool {
	return false
}

// SkipNoDigest returns true if the image having no digest should be skipped,
// i.e. if a digest is required but there is a good reason why it can be skipped
func (s noSkipOptions) SkipNoDigest(image *images.Image) bool {
	return false
}

func (s noSkipOptions) SkipNotFound(image *images.Image) bool {
	return false
}

func (n noSkipOptions) ImageOptions() []images.ImageOption {
	return n
}

func TestLoadingFullValues(t *testing.T) {
	defer test.ReportErr(t)
	require := require.New(t)
	docs := Try(yu.ReadDocs("tests/harbor.yaml"))
	require.Equal(2, len(docs))
	deployment := k3s.HelmChartDeployment{}
	deployment.Load(docs[1])
	helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
	Check(helmProc.LoadDefaultImages())
	require.Equal(10, len(helmProc.Images))
	image := helmProc.Images[0].Repository.Value + ":" + helmProc.Images[0].Tag.Value
	fmt.Println(image, helmProc.Images[0].Repository.Path)
	digested, _, _ := Try3(images.Digest(image, images.IncludeTag, images.SkipTime))
	fmt.Println(digested)
}

func TestLoadingChartValues(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	require := require.New(t)
	docs := Try(yu.ReadDocs("tests/harbor.yaml"))
	require.Equal(2, len(docs))
	deployment := k3s.HelmChartDeployment{}
	deployment.Load(docs[1])
	value := yu.Get[string](Try(deployment.Values()), "expose", "ingress", "hosts", "core")
	assert.Equal(opt.Value("harbor.domain"), value)
}

func TestLoadingOverrides(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	require := require.New(t)
	docs := Try(yu.ReadDocs("tests/harbor.yaml"))
	require.Equal(2, len(docs))
	deployment := k3s.HelmChartDeployment{}
	deployment.Load(docs[1])
	helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
	Check(helmProc.LoadDefaultImages())
	Check(helmProc.ResolveOverrides())
	assert.Equal(10, len(helmProc.DeploymentImages))
	img := Try(images.Parse(helmProc.DeploymentImages[0].String()))
	assert.Equal("v2.11.2", img.Tag)
}

func TestHelmUpdate(t *testing.T) {
	defer test.ReportErr(t)
	// assert := assert.New(t)
	require := require.New(t)
	docs := Try(yu.ReadDocs("tests/harbor.yaml"))
	require.Equal(2, len(docs))
	deployment := k3s.HelmChartDeployment{}
	deployment.Load(docs[1])
	helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
	Check(helmProc.Digest())
	deployment.Save()
}

func TestValueSet(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	require := require.New(t)
	docs := Try(yu.ReadDocs("tests/nginx-ingress.yaml"))
	require.Equal(1, len(docs))
	deployment := k3s.HelmChartDeployment{}
	deployment.Load(docs[0])
	helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
	Check(helmProc.Digest())
	Try(deployment.Save())
	resources := Try(deployment.Render())
	for _, resource := range resources {
		if kind, ok := yu.Get[string](resource, "kind").GetOK(); ok && kind == "Service" {
			type obj = map[string]any
			type lst = []any
			var service obj
			Check(resource.Decode(&service))
			port0 := service["spec"].(obj)["ports"].(lst)[0].(obj)["port"]
			port1 := service["spec"].(obj)["ports"].(lst)[1].(obj)["port"]
			assert.Equal(8080, port0)
			assert.Equal(8443, port1)
			break
		}
	}
}
