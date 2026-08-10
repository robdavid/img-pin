package images_test

import (
	"testing"
	"time"

	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImages(t *testing.T) {
	type ass = *assert.Assertions
	type req = *require.Assertions
	type testFn = func(t *testing.T, assert ass, require req)
	runner := func(test testFn) func(t *testing.T) {
		return func(t *testing.T) {
			if testing.Short() {
				t.Skip("Skipping integration test; --short flag given")
			} else {
				test(t, assert.New(t), require.New(t))
			}
		}
	}

	t.Run("image with tag", runner(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("image without tag", runner(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("image without tag and reject latest", runner(func(t *testing.T, assert ass, require req) {
		_, _, _, err := images.Digest("ubuntu", images.RejectLatest)
		require.ErrorIs(err, images.ErrLatestTag)
	}))

	t.Run("image requiring digest", runner(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("get digest", runner(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("get tag", runner(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("lock image", runner(func(t *testing.T, assert ass, require req) {
		digested, err := images.LockImage("ubuntu:24.04")
		require.NoError(err)
		assert.Regexp(`^index.docker.io/library/ubuntu@sha256:[0-9a-fA-F]+$`, digested)
		redigested, err := images.LockImage(digested)
		require.NoError(err)
		assert.Equal(digested, redigested)
	}))

	t.Run("lock image with minium age", runner(func(t *testing.T, assert ass, require req) {
		_, err := images.LockImage("ubuntu:24.04", images.MinimumAge(time.Hour*24*365*100))
		assert.ErrorIs(err, images.ErrImageTooRecent)
	}))

	t.Run("verify image", runner(func(t *testing.T, assert ass, require req) {
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
	}))

	t.Run("verify hash drift", runner(func(t *testing.T, assert ass, require req) {
		err := images.VerifyImage("hashicorp/vault:"+oldVaultTag+"@"+vaultDigest, "")
		assert.ErrorIs(err, images.ErrDigestMismatch)
		assert.ErrorIs(err, images.ErrTagDrift)
	}))

	t.Run("get digest", runner(func(t *testing.T, assert ass, require req) {
		err := images.VerifyImage("hashicorp/vault:"+vaultTag+"@"+vaultDigest, "")
		assert.NoError(err)
		err = images.VerifyImage("hashicorp/vault:"+vaultTag+"@"+vaultDigest, "", images.MinimumAge(time.Hour*24*365*100))
		assert.ErrorIs(err, images.ErrImageTooRecent)
	}))

	t.Run("verify age with no tag", runner(func(t *testing.T, assert ass, require req) {
		const image = "hashicorp/vault@" + vaultDigest
		err := images.VerifyImage(image, "")
		assert.NoError(err)
		err = images.VerifyImage(image, "", images.MinimumAge(time.Hour*24*365*100))
		assert.ErrorIs(err, images.ErrImageTooRecent)
	}))

	t.Run("update digest", runner(func(t *testing.T, assert ass, require req) {
		updated, err := images.UpdateImage("hashicorp/vault:"+vaultTag+"@"+oldVaultDigest, images.IncludeTag)
		require.NoError(err)
		newDigest, _, err := images.GetDigest(updated)
		require.NoError(err)
		assert.Equal(vaultDigest, newDigest)
		updated2, err := images.UpdateImage(updated, images.IncludeTag)
		require.NoError(err)
		assert.Equal(updated, updated2)
	}))

	// https://github.com/google/go-containerregistry/issues/377
	t.Run("schema version 1", runner(func(t *testing.T, assert ass, require req) {
		const v1Image = "quay.io/dexidp/dex:v2.14.0"
		_, _, _, err := images.Digest(v1Image)
		assert.ErrorIs(err, images.ErrSchemaV1)
	}))

}
