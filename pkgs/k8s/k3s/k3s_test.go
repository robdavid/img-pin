package k3s_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/images"
	imghelpers "github.com/robdavid/img-pin/pkgs/images/test/helpers"
	"github.com/robdavid/img-pin/pkgs/k8s/k3s"
	runhelpers "github.com/robdavid/img-pin/pkgs/run/test/helpers"
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

func TestDigest(t *testing.T) {
	type ass = *assert.Assertions
	type req = *require.Assertions
	type testFn = func(t *testing.T, assert ass, require req)
	runner := func(mode runhelpers.ScriptMode, testFn testFn) func(t *testing.T) {
		return func(t *testing.T) {
			defer test.ReportErr(t)
			k3s.HelmBinary.Unset()
			images.MockDigest(t, imghelpers.CommonMockDigestFunc)
			runhelpers.Script(t, runhelpers.ScriptOpts{
				OutputFile: "tests/run-*.json",
				ArgCompare: runhelpers.ArgsCompare,
				Mode:       mode,
			})
			testFn(t, assert.New(t), require.New(t))
		}
	}

	run := func(testFn testFn) func(*testing.T) { return runner(runhelpers.ScriptModeAuto, testFn) }
	runEmpty := func(testFn testFn) func(*testing.T) { return runner(runhelpers.ScriptModeEmpty, testFn) }

	t.Run("loading full values", run(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("loading chart values", runEmpty(func(t *testing.T, assert ass, require req) {
		images.MockDigest(t, imghelpers.CommonMockDigestFunc)
		docs := Try(yu.ReadDocs("tests/harbor.yaml"))
		require.Equal(2, len(docs))
		deployment := k3s.HelmChartDeployment{}
		deployment.Load(docs[1])
		value := yu.Get[string](Try(deployment.Values()), "expose", "ingress", "hosts", "core")
		assert.Equal(opt.Value("harbor.domain"), value)
	}))

	t.Run("loading overrides", run(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("test helm update", run(func(t *testing.T, assert ass, require req) {
		docs := Try(yu.ReadDocs("tests/harbor.yaml"))
		require.Equal(2, len(docs))
		deployment := k3s.HelmChartDeployment{}
		deployment.Load(docs[1])
		helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
		Check(helmProc.Digest())
		deployment.Save()
	}))

	t.Run("test helm value set", run(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("helm crds", runEmpty(func(t *testing.T, assert ass, require req) {
		docs := Try(yu.ReadDocs("tests/nginx-ingress.yaml"))
		require.Equal(1, len(docs))
		deployment := k3s.HelmChartDeployment{}
		deployment.Load(docs[0])
		helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
		crds := Try(helmProc.CRDs())
		assert.Equal(12, len(crds))
		for i, crd := range crds {
			assert.Equal("CustomResourceDefinition", yu.Get[string](crd, "kind").GetOr(""), "doc %n is not a CRD", i)
		}
	}))

	t.Run("helm version", run(func(t *testing.T, assert ass, require req) {
		assert.True(k3s.HelmVersionAtLeast("v2"), "unexpected helm version %q", k3s.HelmBinary.Version())
		assert.False(k3s.HelmVersionAtLeast("v99"), "unexpected helm version %q", k3s.HelmBinary.Version())
	}))
}

func TestHelmBinary(t *testing.T) {

	const envName = "IMG_PIN_HELM_TEST"

	defer k3s.HelmBinary.Unset()

	t.Run("with default binary", func(t *testing.T) {
		k3s.HelmBinary.SetEnv(envName)
		os.Unsetenv(envName)
		for range 2 {
			assert.Equal(t, "helm", k3s.HelmBinary.Value())
		}
	})

	t.Run("with specific binary", func(t *testing.T) {
		k3s.HelmBinary.SetEnv(envName)
		os.Setenv(envName, "helm4")
		defer os.Unsetenv(envName)
		for range 2 {
			assert.Equal(t, "helm4", k3s.HelmBinary.Value())
		}
	})

}
