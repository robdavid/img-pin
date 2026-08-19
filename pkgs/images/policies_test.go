package images_test

import (
	"strings"
	"testing"
	"time"

	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/robdavid/img-pin/pkgs/images/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShortAgePolicy(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	images.MockDigest(t, helpers.CommonMockDigestFunc)
	img, err := images.Parse("ubuntu:24.04")
	assert.NoError(err)
	created, err := img.GetDigest(images.AddPolicy(images.DefaultMinAgePolicy(365 * 24 * time.Hour)))
	require.Error(err)
	assert.ErrorIs(err, images.ErrImageTooRecent)
	assert.True(time.Since(created) < 365*24*time.Hour)
}

func TestUpdateGroup(t *testing.T) {
	assert := assert.New(t)
	images.MockDigest(t, helpers.CommonMockDigestFunc)
	img, err := images.Parse("ubuntu:24.04")
	assert.NoError(err)
	ip := images.ImageParts{Registry: "public.ecr.aws", Group: "library"}
	ip.UpdateImage(img)
	assert.Equal("public.ecr.aws", img.Registry)
	assert.Equal("library/ubuntu", img.Repository)
	assert.Equal("24.04", img.Tag)
}

func TestDefaultAgeMappingPolicy(t *testing.T) {
	mapping := map[images.ImageParts]time.Duration{
		{Registry: "docker.io"}:                                   time.Minute,
		{Registry: "docker.io", Group: "library"}:                 time.Minute * 2,
		{Registry: "docker.io", Group: "library", Name: "debian"}: time.Minute * 3,
	}
	testTable := []struct {
		image string
		age   opt.Val[time.Duration]
	}{
		{"docker.io/ubuntu/ubuntu:24.04", opt.Value(time.Minute)},
		{"docker.io/library/fedora:44", opt.Value(time.Minute * 2)},
		{"docker.io/library/debian:13.6", opt.Value(time.Minute * 3)},
		{"public.ecr.aws/docker/library/centos:centos7.9.2009", opt.Empty[time.Duration]()},
	}
	for _, testEntry := range testTable {
		t.Run(strings.ReplaceAll(testEntry.image, "/", "_"), func(t *testing.T) {
			assert := assert.New(t)
			img, err := images.Parse(testEntry.image)
			assert.NoError(err)
			policy := images.DefaultAgeMapperPolicy(mapping)
			polCtx := images.PolicyContext{Image: img, Options: new([]images.ImageOption)}
			policy(&polCtx)
			opts := images.BuildOptions(*polCtx.Options)
			assert.Equal(testEntry.age, opts.MinAge)
		})
	}

}
