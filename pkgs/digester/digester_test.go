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
	"github.com/robdavid/img-pin/pkgs/internal/test/helpers"
	_ "github.com/robdavid/img-pin/pkgs/k8s/k3s"
	_ "github.com/robdavid/img-pin/pkgs/k8s/workload"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDigestK3S(t *testing.T) {
	defer test.ReportErr(t)
	tempFile := helpers.CopyToTemp(t, "tests/harbor.yaml")
	defer os.Remove(tempFile)
	eh.Check(digester.CreateDigests(tempFile))
	content := eh.Try(os.ReadFile(tempFile))
	re := regexp.MustCompile(`v2\.11\.1\@sha256:[a-z0-9]{64}`)
	matches := re.FindAll(content, -1)
	assert.Equal(t, 10, len(matches))
	fmt.Printf("content:\n%s\n", content)
}

func TestDigestK3SPatch(t *testing.T) {
	defer test.ReportErr(t)
	tempDir := helpers.CopyToTempDir(t, "tests/harbor.yaml", "tests/harbor.lock.yaml")
	defer tempDir.Delete()
	eh.Check(digester.CreateDigests(tempDir.First(),
		digester.UpdateMethod(types.UpdatePatch), digester.UseLockfile))
	content := eh.Try(os.ReadFile(tempDir.First()))
	re := regexp.MustCompile(`v2\.11\.1\@sha256:[a-z0-9]{64}`)
	matches := re.FindAll(content, -1)
	assert.Equal(t, 10, len(matches))
	fmt.Printf("content:\n%s\n", content)
}

func TestDigestVerifyK3S(t *testing.T) {
	defer test.ReportErr(t)
	tempFile := helpers.CopyToTemp(t, "tests/harbor.yaml")
	defer os.Remove(tempFile)
	err := digester.VerifyDigests(tempFile)
	assert.ErrorIs(t, err, images.ErrNoDigest)
	fmt.Printf("%v\n", err)
}

func TestDigestKube(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	tempDir := helpers.CopyToTempDir(t, "tests/harbor.yaml", "tests/harbor.lock.yaml")
	defer tempDir.Delete()
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
}

func TestDigestKubeList(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	tempFile := helpers.CopyToTemp(t, "tests/akri.yaml")
	defer os.Remove(tempFile)
	dig := eh.Try(digester.DigestKube(tempFile, digester.UseLockfile))
	digester.WriteCombinedDigests([]*digester.Digester{dig}, os.Stdout)
	assert.Greater(len(dig.Resources), 19)
}

func TestDigestK8S(t *testing.T) {
	defer test.ReportErr(t)
	require := require.New(t)
	assert := assert.New(t)
	tempFile := helpers.CopyToTemp(t, "tests/opag.yaml")
	defer os.Remove(tempFile)
	eh.Check(digester.CreateDigests(tempFile))
	content := eh.Try(os.ReadFile(tempFile))
	buf := bytes.NewBuffer(content)
	docs := eh.Try(yu.StreamDocsIn(buf))
	require.Equal(5, len(docs))
	image := yu.Get[string](docs[4], "spec", "template", "spec", "containers", 0, "image")
	assert.Contains(image.GetOr(""), "@sha256:")
}

func TestDigestSchemaV1(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	tempFile := helpers.CopyToTemp(t, "tests/dex.yaml")
	defer os.Remove(tempFile)
	err := digester.CreateDigests(tempFile, digester.ImageOptions(images.MinimumAge(time.Hour*24)))
	assert.ErrorIs(err, images.ErrSchemaV1)
	fmt.Println(err)
}

func TestDigestSchemaV1Skipped(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	tempFile := helpers.CopyToTemp(t, "tests/dex.yaml")
	defer os.Remove(tempFile)
	err := digester.CreateDigests(tempFile, digester.ImageOptions(images.MinimumAge(time.Hour*24)), digester.SkipV1Schema)
	assert.NoError(err)
	fmt.Println(err)
}
