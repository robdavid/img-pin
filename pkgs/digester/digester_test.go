package digester_test

import (
	"bytes"
	"fmt"
	"os"
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
	fmt.Printf("content:\n%s\n", content)
}

func TestDigestK3SPatch(t *testing.T) {
	defer test.ReportErr(t)
	tempFile := helpers.CopyToTemp(t, "tests/harbor.yaml")
	defer os.Remove(tempFile)
	eh.Check(digester.CreateDigests(tempFile,
		digester.UpdateMethod(types.UpdatePatch)))
	content := eh.Try(os.ReadFile(tempFile))
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
