package images_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageWithTag(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	digested, digest, created, err := images.Digest("ubuntu:24.04", images.IncludeTag)
	require.NoError(err)
	assert.False(created.IsZero())
	assert.Regexp(`^sha256:[0-9a-fA-F]+$`, digest)
	assert.Equal("index.docker.io/library/ubuntu:24.04@"+digest, digested)
	reDigested, reDigest, created, err := images.Digest(digested)
	require.NoError(err)
	assert.Equal(digest, reDigest)
	assert.Equal(digested, reDigested)
	assert.False(created.IsZero())
}

func TestImageWithoutTag(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	digested, digest, created, err := images.Digest("ubuntu:24.04")
	require.NoError(err)
	assert.False(created.IsZero())
	assert.Regexp(`^sha256:[0-9a-fA-F]+$`, digest)
	assert.Equal("index.docker.io/library/ubuntu@"+digest, digested)
	reDigested, reDigest, created, err := images.Digest(digested)
	require.NoError(err)
	assert.Equal(digest, reDigest)
	assert.Equal(digested, reDigested)
	assert.False(created.IsZero())
}

func TestImageWithoutTagRejectLatest(t *testing.T) {
	require := require.New(t)
	_, _, _, err := images.Digest("ubuntu", images.RejectLatest)
	require.ErrorIs(err, images.ErrLatestTag)
}

func TestImageRequiringDigest(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	digested, digest, created, err := images.Digest("ubuntu:24.04", images.RequireDigest)
	require.ErrorIs(err, images.ErrNoDigest)
	assert.Empty(digested)
	assert.Empty(digest)
	assert.False(created.IsZero())
	digested, digest, created, err = images.Digest("ubuntu:24.04", images.RequireDigest, images.SkipTime)
	require.ErrorIs(err, images.ErrNoDigest)
	assert.Empty(digested)
	assert.Empty(digest)
	assert.True(created.IsZero())
}

func TestGetDigest(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	digest, noDigest, err := images.GetDigest("ubuntu:24.04")
	require.ErrorIs(err, images.ErrNoDigest)
	assert.Empty(digest)
	assert.Equal("ubuntu:24.04", noDigest)
	digest, noDigest, err = images.GetDigest("ubuntu@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(err)
	assert.Equal("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", digest)
	assert.Equal("ubuntu", noDigest)
	digest, noDigest, err = images.GetDigest("ubuntu:24.04@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(err)
	assert.Equal("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", digest)
	assert.Equal("ubuntu:24.04", noDigest)
}

func TestGetTag(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tag, noTag, err := images.GetTag("ubuntu:24.04")
	require.NoError(err)
	assert.Equal("24.04", tag)
	assert.Equal("ubuntu", noTag)
	tag, noTag, err = images.GetTag("ubuntu")
	require.NoError(err)
	assert.Empty(tag)
	assert.Equal("ubuntu", noTag)
	tag, noTag, err = images.GetTag("ubuntu@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(err)
	assert.Empty(tag)
	assert.Equal("ubuntu", noTag)
	tag, noTag, err = images.GetTag("ubuntu:24.04@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(err)
	assert.Equal("24.04", tag)
	assert.Equal("ubuntu", noTag)
	tag, noTag, err = images.GetTag("ubuntu:latest")
	require.NoError(err)
	assert.Equal("latest", tag)
	assert.Equal("ubuntu", noTag)
}

func TestLockImage(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	digested, err := images.LockImage("ubuntu:24.04")
	require.NoError(err)
	assert.Regexp(`^index.docker.io/library/ubuntu@sha256:[0-9a-fA-F]+$`, digested)
	redigested, err := images.LockImage(digested)
	require.NoError(err)
	assert.Equal(digested, redigested)
}

func TestLockImageMinAge(t *testing.T) {
	assert := assert.New(t)
	_, err := images.LockImage("ubuntu:24.04", images.MinimumAge(time.Hour*24*365*100))
	assert.ErrorIs(err, images.ErrImageTooRecent)
}

const vaultTag = "1.13.3"
const oldVaultTag = "1.13.2"
const vaultDigest = "sha256:5eba321fbeb624163a45c1aee5379caf6ec16fe6f644cc89f203a209eafba5eb"
const oldVaultDigest = "sha256:c186e9bff2db0bd61dad70e7733cbfa5a0f8ddee3a6a061f3753060689aa81ab"

func TestVerifyImage(t *testing.T) {
	assert := assert.New(t)
	err := images.VerifyImage("hashicorp/vault:"+vaultTag+"@"+vaultDigest, "")
	assert.NoError(err)
	err = images.VerifyImage("hashicorp/vault:"+vaultTag+"@"+vaultDigest, vaultDigest)
	assert.NoError(err)
	err = images.VerifyImage("hashicorp/vault:"+vaultTag+"@"+vaultDigest, "sha256:deadbeef")
	assert.ErrorIs(err, images.ErrDigestMismatch)
	err = images.VerifyImage("hashicorp/vault:"+vaultTag, vaultDigest)
	assert.NoError(err)
	err = images.VerifyImage("hashicorp/vault:"+vaultTag, "sha256:deadbeef")
	assert.ErrorIs(err, images.ErrDigestMismatch)
	err = images.VerifyImage("hashicorp/vault", "")
	assert.ErrorIs(err, images.ErrNoDigest)
	err = images.VerifyImage("hashicorp/vault@"+vaultDigest, "")
}

func TestVerifyHashDrift(t *testing.T) {
	assert := assert.New(t)
	err := images.VerifyImage("hashicorp/vault:"+oldVaultTag+"@"+vaultDigest, "")
	assert.ErrorIs(err, images.ErrDigestMismatch)
	assert.ErrorIs(err, images.ErrTagDrift)
}

func TestVerifyAge(t *testing.T) {
	assert := assert.New(t)
	err := images.VerifyImage("hashicorp/vault:"+vaultTag+"@"+vaultDigest, "")
	assert.NoError(err)
	err = images.VerifyImage("hashicorp/vault:"+vaultTag+"@"+vaultDigest, "", images.MinimumAge(time.Hour*24*365*100))
	assert.ErrorIs(err, images.ErrImageTooRecent)
}

func TestVerifyAgeNoTag(t *testing.T) {
	const image = "hashicorp/vault@" + vaultDigest
	assert := assert.New(t)
	err := images.VerifyImage(image, "")
	assert.NoError(err)
	err = images.VerifyImage(image, "", images.MinimumAge(time.Hour*24*365*100))
	assert.ErrorIs(err, images.ErrImageTooRecent)
}

func TestUpdateDigest(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	updated, err := images.UpdateImage("hashicorp/vault:"+vaultTag+"@"+oldVaultDigest, images.IncludeTag)
	require.NoError(err)
	newDigest, _, err := images.GetDigest(updated)
	require.NoError(err)
	assert.Equal(vaultDigest, newDigest)
	updated2, err := images.UpdateImage(updated, images.IncludeTag)
	require.NoError(err)
	assert.Equal(updated, updated2)
}

func dumpErrs(err error) {
	fmt.Printf("%T: %s\n", err, err)
	if uerr, ok := err.(interface{ Unwrap() error }); ok {
		dumpErrs(uerr.Unwrap())
	} else if uerrs, ok := err.(interface{ Unwrap() []error }); ok {
		for _, uerr := range uerrs.Unwrap() {
			dumpErrs(uerr)
		}
	}
}

// https://github.com/google/go-containerregistry/issues/377
func TestSchemaV1(t *testing.T) {
	const v1Image = "quay.io/dexidp/dex:v2.14.0"
	_, _, _, err := images.Digest(v1Image)
	assert.ErrorIs(t, err, images.ErrSchemaV1)
}
