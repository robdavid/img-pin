package digester_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	eh "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/images"
	imghelpers "github.com/robdavid/img-pin/pkgs/images/test/helpers"
	"github.com/robdavid/img-pin/pkgs/internal/test/helpers"
	"github.com/robdavid/img-pin/pkgs/k8s/k3s"
	_ "github.com/robdavid/img-pin/pkgs/k8s/k3s"
	"github.com/robdavid/img-pin/pkgs/k8s/kube"
	_ "github.com/robdavid/img-pin/pkgs/k8s/workload"
	"github.com/robdavid/img-pin/pkgs/lock"
	runhelpers "github.com/robdavid/img-pin/pkgs/run/test/helpers"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestDigest(t *testing.T) {
	type ass = *assert.Assertions
	type req = *require.Assertions
	type testFn = func(t *testing.T, assert ass, require req)
	runner := func(mode runhelpers.ScriptMode, kubeVersion string, testFn testFn) func(t *testing.T) {
		return func(t *testing.T) {
			defer test.ReportErr(t)
			k3s.HelmBinary.Unset()
			kube.KubeVersion.Unset()
			os.Setenv(kube.KubeVersion.Env, kubeVersion)
			defer os.Unsetenv(kube.KubeVersion.Env)
			images.MockDigest(t, imghelpers.CommonMockDigestFunc)
			runhelpers.Script(t, runhelpers.ScriptOpts{
				OutputFile: "tests/run-*.json",
				ArgCompare: runhelpers.ArgsCompare,
				Mode:       mode,
			})
			testFn(t, assert.New(t), require.New(t))
		}
	}

	run := func(testFn testFn) func(*testing.T) { return runner(runhelpers.ScriptModeAuto, "", testFn) }
	runKube := func(kubeVersion string, testFn testFn) func(*testing.T) {
		return runner(runhelpers.ScriptModeAuto, kubeVersion, testFn)
	}
	// runCapture := func(testFn testFn) func(*testing.T) { return runner(false, true, testFn) }

	t.Run("test K3S chart modification", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/harbor.yaml")
		eh.Check(digester.CreateDigests(tempFile))
		content := test.Result(os.ReadFile(tempFile)).Must(t)
		re := regexp.MustCompile(`v2\.11\.1\@sha256:[a-z0-9]{64}`)
		matches := re.FindAll(content, -1)
		assert.Equal(10, len(matches))
		fmt.Printf("content:\n%s\n", content)
	}))

	t.Run("test K3S chart modification with patch update method", run(func(t *testing.T, assert ass, require req) {
		tempDir := helpers.CopyToTempDir(t, "tests/harbor.yaml", "tests/harbor.lock.yaml")
		images.MockDigest(t, imghelpers.CommonMockDigestFunc)
		eh.Check(digester.CreateDigests(tempDir.First(),
			digester.UpdateMethod(types.UpdatePatch), digester.UseLockFile))
		content := test.Result(os.ReadFile(tempDir.First())).Must(t)
		re := regexp.MustCompile(`v2\.11\.1\@sha256:[a-z0-9]{64}`)
		matches := re.FindAll(content, -1)
		assert.Equal(10, len(matches))
		fmt.Printf("content:\n%s\n", content)
	}))

	t.Run("test K3S chart verfication failure", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/harbor.yaml")
		err := digester.VerifyDigests(tempFile)
		assert.ErrorIs(err, images.ErrNoDigest)
		fmt.Printf("%v\n", err)
	}))

	testExpansion := func(t *testing.T, assert ass, require req) {
		tempDir := helpers.CopyToTempDir(t, "tests/harbor.yaml", "tests/harbor.lock.yaml")
		dig := test.Result(digester.DigestKube(tempDir.First(), digester.UseLockFile)).Must(t)
		assert.Greater(len(dig.Resources), 30)
		var buffer bytes.Buffer
		eh.Check(digester.WriteCombinedDigests([]*digester.Digester{dig}, &buffer))
		re := regexp.MustCompile(`image: docker\.io/goharbor/.*:v2\.11\.1\@sha256:[a-z0-9]{64}`)
		matches := re.FindAll(buffer.Bytes(), -1)
		assert.Equal(9, len(matches))
		reImg := regexp.MustCompile(`image:`)
		matches = reImg.FindAll(buffer.Bytes(), -1)
		assert.Equal(9, len(matches))
	}

	t.Run("test K3S chart expansion", run(testExpansion))
	t.Run("test K3S chart expansion with kubeconfig", runKube("1.36", testExpansion))

	t.Run("test K3S chart expansion with resource list", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/akri.yaml")
		dig := test.Result(digester.DigestKube(tempFile)).Must(t)
		digester.WriteCombinedDigests([]*digester.Digester{dig}, os.Stdout)
		assert.Greater(len(dig.Resources), 19)
	}))

	t.Run("test K3S chart modify gatekeeper", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/opag.yaml")
		test.Result0(digester.CreateDigests(tempFile)).Must(t)
		content := test.Result(os.ReadFile(tempFile)).Must(t)
		buf := bytes.NewBuffer(content)
		docs := test.Result(yu.StreamDocsIn(buf)).Must(t)
		require.Equal(5, len(docs))
		image := yu.Get[string](docs[4], "spec", "template", "spec", "containers", 0, "image")
		assert.Contains(image.GetOr(""), "@sha256:")
	}))

	t.Run("test digest V1 schema", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/dex.yaml")
		err := digester.CreateDigests(tempFile, digester.ImageOptions(images.MinimumAge(time.Hour*24)))
		assert.ErrorIs(err, images.ErrSchemaV1)
		fmt.Println(err)
	}))

	t.Run("test skipping digest V1 schema", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/dex.yaml")
		err := digester.CreateDigests(tempFile, digester.ImageOptions(images.MinimumAge(time.Hour*24)), digester.SkipV1Schema)
		assert.NoError(err)
		fmt.Println(err)
	}))

	t.Run("test load digester from stream", run(func(t *testing.T, assert ass, require req) {
		var output bytes.Buffer
		fh, err := os.Open("tests/opag.yaml")
		require.NoError(err)
		defer fh.Close()
		dig := digester.NewDigester(digester.ImageOptions(images.IncludeTag))
		err = dig.LoadStream(fh)
		require.NoError(err)
		require.NoError(dig.CreateDigests())
		require.NoError(dig.Write(&output))
		assert.Contains(output.String(), "docker.io/openpolicyagent/gatekeeper:dev@sha256:c64a643dd665db62c43aa089432eb2e74b13364c616fc12ca524baead6ccc332")
	}))
}

func TestDigestLockUpdate(t *testing.T) {

	type ass = *assert.Assertions
	type req = *require.Assertions
	type testFn = func(t *testing.T, assert ass, require req, iteration int, digests images.DigestFunc)
	runner := func(mode runhelpers.ScriptMode, kubeVersion string, testFn testFn) func(t *testing.T) {
		return func(t *testing.T) {
			defer test.ReportErr(t)
			k3s.HelmBinary.Unset()
			kube.KubeVersion.Unset()
			os.Setenv(kube.KubeVersion.Env, kubeVersion)
			defer os.Unsetenv(kube.KubeVersion.Env)
			runhelpers.Script(t, runhelpers.ScriptOpts{
				OutputFile: "tests/run-*.json",
				ArgCompare: runhelpers.ArgsCompare,
				Mode:       mode,
			})
			images.MockDigest(t, imghelpers.CommonMockDigestFunc)
			testFn(t, assert.New(t), require.New(t), 1, imghelpers.CommonMockDigestFunc)
			images.MockDigest(t, imghelpers.CommonMockDigest2Func)
			testFn(t, assert.New(t), require.New(t), 2, imghelpers.CommonMockDigest2Func)
		}
	}
	run := func(testFn testFn) func(*testing.T) { return runner(runhelpers.ScriptModeCapture, "", testFn) }

	t.Run("test lock file image update", run(func(t *testing.T, assert ass, require req, iteration int, digests images.DigestFunc) {
		tempDir := helpers.CopyToTempDir(t, "tests/akri.yaml", "tests/akri.lock.yaml")
		yamlFile := filepath.Join(tempDir.Dir, "akri.yaml")
		lockFile := filepath.Join(tempDir.Dir, "akri.lock.yaml")
		var dig *digester.Digester
		var buffer bytes.Buffer
		var lockData lock.LockData
		switch iteration {
		case 1:
			dig = test.Result(digester.DigestKube(yamlFile, digester.UseLockFile, digester.LockFileName(lockFile))).Must(t)
		case 2:
			dig = test.Result(digester.DigestKube(yamlFile, digester.UseLockFile, digester.LockFileName(lockFile), digester.UpdateAllLocks)).Must(t)
		}
		dig.Write(&buffer)
		lockText := test.Result(os.ReadFile(lockFile)).Must(t)
		test.Check(t, yaml.Unmarshal(lockText, &lockData))
		ki := slices.FindUsingRef(lockData.Images, func(ld *lock.ImageData) bool {
			return ld.Source.Registry == "docker.io" && ld.Source.Repository == "bitnami/kubectl" && ld.Source.Tag == "latest"
		})
		require.GreaterOrEqual(ki, 0)
		digest, ok := lockData.Images[ki].Digest.RefOK()
		require.True(ok)
		expected := test.Result(imghelpers.LookupDigest(digests, "docker.io/bitnami/kubectl:latest")).Must(t)
		assert.Equal(expected, digest.Digest)
	}))

}
