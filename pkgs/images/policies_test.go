package images_test

import (
	"testing"
	"time"

	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShortAgePolicy(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	img, err := images.Parse("ubuntu:24.04")
	assert.NoError(err)
	created, err := img.GetDigest(images.AddPolicy(images.DefaultMinAgePolicy(365 * 24 * time.Hour)))
	require.Error(err)
	assert.ErrorIs(err, images.ErrImageTooRecent)
	assert.True(time.Since(created) < 365*24*time.Hour)
}

func TestUpdateGroup(t *testing.T) {
	assert := assert.New(t)
	img, err := images.Parse("ubuntu:24.04")
	assert.NoError(err)
	ip := images.ImageParts{Registry: "public.ecr.aws", Group: "library"}
	ip.UpdateImage(img)
	assert.Equal("public.ecr.aws", img.Registry)
	assert.Equal("library/ubuntu", img.Repository)
	assert.Equal("24.04", img.Tag)
}
