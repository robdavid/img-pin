package digester_test

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	eh "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/images"
	imghelpers "github.com/robdavid/img-pin/pkgs/images/test/helpers"
	"github.com/robdavid/img-pin/pkgs/internal/test/helpers"
	"github.com/robdavid/img-pin/pkgs/k8s/k3s"
	_ "github.com/robdavid/img-pin/pkgs/k8s/k3s"
	_ "github.com/robdavid/img-pin/pkgs/k8s/workload"
	runhelpers "github.com/robdavid/img-pin/pkgs/run/test/helpers"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanSetup() {}

func TestDigest(t *testing.T) {
	type ass = *assert.Assertions
	type req = *require.Assertions
	type testFn = func(t *testing.T, assert ass, require req)
	runner := func(auto bool, capture bool, testFn testFn) func(t *testing.T) {
		return func(t *testing.T) {
			defer test.ReportErr(t)
			k3s.UnsetHelmBinary()
			k3s.UnsetHelmBinaryEnv()
			images.MockDigest(t, imghelpers.CommonMockDigestFunc)
			const fileTemplate = "tests/run-*.json"
			if auto {
				runhelpers.CaptureOrRunScript(t, fileTemplate, runhelpers.ArgsCompare)
			} else if capture {
				runhelpers.Capture(t, fileTemplate)
			} else {
				runhelpers.ScriptTmpArgs(t, fileTemplate)
			}

			testFn(t, assert.New(t), require.New(t))
		}
	}

	run := func(testFn testFn) func(*testing.T) { return runner(true, false, testFn) }
	// runCapture := func(testFn testFn) func(*testing.T) { return runner(false, true, testFn) }

	t.Run("test K3S chart modification", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/harbor.yaml")
		eh.Check(digester.CreateDigests(tempFile))
		content := eh.Try(os.ReadFile(tempFile))
		re := regexp.MustCompile(`v2\.11\.1\@sha256:[a-z0-9]{64}`)
		matches := re.FindAll(content, -1)
		assert.Equal(10, len(matches))
		fmt.Printf("content:\n%s\n", content)
	}))

	t.Run("test K3S chart modification with patch update method", run(func(t *testing.T, assert ass, require req) {
		tempDir := helpers.CopyToTempDir(t, "tests/harbor.yaml", "tests/harbor.lock.yaml")
		images.MockDigest(t, imghelpers.CommonMockDigestFunc)
		eh.Check(digester.CreateDigests(tempDir.First(),
			digester.UpdateMethod(types.UpdatePatch), digester.UseLockfile))
		content := eh.Try(os.ReadFile(tempDir.First()))
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

	t.Run("test K3S chart expansion", run(func(t *testing.T, assert ass, require req) {
		tempDir := helpers.CopyToTempDir(t, "tests/harbor.yaml", "tests/harbor.lock.yaml")
		dig := eh.Try(digester.DigestKube(tempDir.First(), digester.UseLockfile))
		assert.Greater(len(dig.Resources), 30)
		var buffer bytes.Buffer
		eh.Check(digester.WriteCombinedDigests([]*digester.Digester{dig}, &buffer))
		re := regexp.MustCompile(`image: docker\.io/goharbor/.*:v2\.11\.1\@sha256:[a-z0-9]{64}`)
		matches := re.FindAll(buffer.Bytes(), -1)
		assert.Equal(9, len(matches))
		reImg := regexp.MustCompile(`image:`)
		matches = reImg.FindAll(buffer.Bytes(), -1)
		assert.Equal(9, len(matches))
	}))

	t.Run("test K3S chart expansion with resource list", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/akri.yaml")
		dig := eh.Try(digester.DigestKube(tempFile))
		digester.WriteCombinedDigests([]*digester.Digester{dig}, os.Stdout)
		assert.Greater(len(dig.Resources), 19)
	}))

	t.Run("test K3S chart modify gatekeeper", run(func(t *testing.T, assert ass, require req) {
		tempFile := helpers.CopyToTemp(t, "tests/opag.yaml")
		eh.Check(digester.CreateDigests(tempFile))
		content := eh.Try(os.ReadFile(tempFile))
		buf := bytes.NewBuffer(content)
		docs := eh.Try(yu.StreamDocsIn(buf))
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
}
