package images_test

import (
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImage(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	img, err := images.Parse("ubuntu:24.04")
	require.NoError(err)
	assert.Equal("docker.io", img.Registry)
	assert.Equal("library/ubuntu", img.Repository)
	assert.Equal("24.04", img.Tag)
	assert.Empty(img.Digest)
	assert.Equal("docker.io/library/ubuntu:24.04", img.String())
	img, err = images.Parse("hashicorp/vault:1.13.3@sha256:5eba321fbeb624163a45c1aee5379caf6ec16fe6f644cc89f203a209eafba5eb")
	require.NoError(err)
	assert.Equal("docker.io", img.Registry)
	assert.Equal("hashicorp/vault", img.Repository)
	assert.Equal("1.13.3", img.Tag)
	assert.Equal("sha256:5eba321fbeb624163a45c1aee5379caf6ec16fe6f644cc89f203a209eafba5eb", img.Digest)
	assert.Equal("docker.io/hashicorp/vault:1.13.3@sha256:5eba321fbeb624163a45c1aee5379caf6ec16fe6f644cc89f203a209eafba5eb", img.String())
	img, err = images.Parse("myregistry.com/myrepo/myimage:1.0")
	require.NoError(err)
	assert.Equal("myregistry.com", img.Registry)
	assert.Equal("myrepo/myimage", img.Repository)
	assert.Equal("1.0", img.Tag)
	assert.Empty(img.Digest)
	assert.Equal("myregistry.com/myrepo/myimage:1.0", img.String())
	img, err = images.Parse("myregistry.com/myrepo/myimage@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(err)
	assert.Equal("myregistry.com", img.Registry)
	assert.Equal("myrepo/myimage", img.Repository)
	assert.Empty(img.Tag)
	assert.Equal("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", img.Digest)
	assert.Equal("myregistry.com/myrepo/myimage@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", img.String())
}

func TestSetDigestNoTag(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	img, err := images.Parse("ubuntu:24.04")
	require.NoError(err)
	assert.Empty(img.Digest)
	created, err := img.GetDigest()
	require.NoError(err)
	assert.False(created.IsZero())
	assert.NotEmpty(img.Digest)
	assert.Regexp(`^docker\.io/library/ubuntu\@sha256:[a-z0-9]+$`, img.String())
}

func TestSetDigestWithTag(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	img, err := images.Parse("ubuntu:24.04")
	require.NoError(err)
	assert.Empty(img.Digest)
	created, err := img.GetDigest(images.IncludeTag)
	require.NoError(err)
	assert.False(created.IsZero())
	assert.NotEmpty(img.Digest)
	assert.Regexp(`^docker\.io/library/ubuntu:24.04\@sha256:[a-z0-9]+$`, img.String())
}

func TestVerifyParsedImage(t *testing.T) {
	test.ReportErr(t)
	assert := assert.New(t)
	err := Try(images.Parse("hashicorp/vault:" + vaultTag + "@" + vaultDigest)).VerifyDigest()
	assert.NoError(err)
	err = Try(images.Parse("hashicorp/vault:" + vaultTag + "@" + vaultDigest)).VerifyDigest(images.ExpectDigest(vaultDigest))
	assert.NoError(err)
	err = Try(images.Parse("hashicorp/vault:" + vaultTag + "@" + vaultDigest)).VerifyDigest(images.ExpectDigest("sha256:deadbeef"))
	assert.ErrorIs(err, images.ErrDigestMismatch)
	err = Try(images.Parse("hashicorp/vault:" + vaultTag)).VerifyDigest(images.ExpectDigest(vaultDigest))
	assert.NoError(err)
	err = Try(images.Parse("hashicorp/vault:" + vaultTag)).VerifyDigest(images.ExpectDigest("sha256:deadbeef"))
	assert.ErrorIs(err, images.ErrDigestMismatch)
	err = Try(images.Parse("hashicorp/vault")).VerifyDigest()
	assert.ErrorIs(err, images.ErrNoDigest)
	err = Try(images.Parse("hashicorp/vault:" + vaultTag + "@" + oldVaultDigest)).VerifyDigest()
	assert.ErrorIs(err, images.ErrDigestMismatch)
	assert.ErrorIs(err, images.ErrTagDrift)
	assert.ErrorContains(err, "hashicorp/vault:"+vaultTag+"@"+oldVaultDigest)
}

func TestUpdateParsedDigest(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	image := Try(images.Parse("hashicorp/vault:" + vaultTag + "@" + oldVaultDigest))
	err := image.UpdateDigest(images.IncludeTag)
	require.NoError(err)
	newDigest, _, err := images.GetDigest(image.String())
	require.NoError(err)
	assert.Equal(vaultDigest, newDigest)
	image2 := image.Clone()
	err = image2.UpdateDigest(images.IncludeTag)
	require.NoError(err)
	assert.Equal(*image, *image2)
}
