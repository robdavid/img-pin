package dockerfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robdavid/img-pin/pkgs/dockerfile"
	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanForImages(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	input := `FROM ubuntu:24.04
RUN echo "Hello, world!"
FROM alpine:3.18`
	occurrences, err := dockerfile.ScanForImages(strings.NewReader(input), true)
	require.NoError(err)
	require.Len(occurrences, 2)
	assert.Equal("ubuntu:24.04", occurrences[0].Image)
	assert.Equal(1, occurrences[0].Line)
	assert.Equal("alpine:3.18", occurrences[1].Image)
	assert.Equal(3, occurrences[1].Line)
}

func TestLockImages(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	input := `FROM ubuntu:24.04
RUN echo "Hello, world!"
FROM alpine:3.18`
	var output strings.Builder
	count, _, _, err := dockerfile.LockImages(strings.NewReader(input), &output)
	require.NoError(err)
	assert.Equal(2, count)
	lines := strings.Split(output.String(), "\n")
	assert.Len(lines, 4) // 3 lines + trailing newline
	assert.Regexp(`^FROM docker\.io/library/ubuntu@sha256:[0-9a-fA-F]+$`, lines[0])
	assert.Equal("RUN echo \"Hello, world!\"", lines[1])
	assert.Regexp(`^FROM docker\.io/library/alpine@sha256:[0-9a-fA-F]+$`, lines[2])
}

func TestLockImagesSyntaxVariations(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	input := `FROM --platform=linux/amd64 ubuntu:24.04
RUN echo "Hello, world!"
FROM alpine:3.18 AS builder`
	var output strings.Builder
	count, _, _, err := dockerfile.LockImages(strings.NewReader(input), &output)
	require.NoError(err)
	assert.Equal(2, count)
	lines := strings.Split(output.String(), "\n")
	assert.Len(lines, 4) // 3 lines + trailing newline
	assert.Regexp(`^FROM --platform=linux/amd64 docker\.io/library/ubuntu@sha256:[0-9a-fA-F]+$`, lines[0])
	assert.Equal("RUN echo \"Hello, world!\"", lines[1])
	assert.Regexp(`^FROM docker\.io/library/alpine@sha256:[0-9a-fA-F]+ AS builder$`, lines[2])
}

func TestPatch(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	content := "FROM ubuntu:24.04\nRUN echo \"Hello, world!\"\nFROM alpine:3.18\n"
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	require.NoError(os.WriteFile(dockerfilePath, []byte(content), 0644))

	count, _, _, err := dockerfile.Patch(dockerfilePath)
	require.NoError(err)
	assert.Equal(2, count)

	patched, err := os.ReadFile(dockerfilePath)
	require.NoError(err)

	lines := strings.Split(string(patched), "\n")
	require.Len(lines, 4) // 3 lines + trailing newline
	assert.Regexp(`^FROM docker\.io/library/ubuntu@sha256:[0-9a-fA-F]+$`, lines[0])
	assert.Equal("RUN echo \"Hello, world!\"", lines[1])
	assert.Regexp(`^FROM docker\.io/library/alpine@sha256:[0-9a-fA-F]+$`, lines[2])
	assert.Empty(lines[3])
}

func TestPatchWithFailedVerifyOnly(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	content := "FROM ubuntu:24.04\nRUN echo \"Hello, world!\"\nFROM alpine:3.18\n"
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	require.NoError(os.WriteFile(dockerfilePath, []byte(content), 0644))

	count, _, _, err := dockerfile.Patch(dockerfilePath, dockerfile.VerifyOnly)
	require.Error(err)
	assert.Equal(0, count)
	assert.ErrorIs(err, images.ErrNoDigest)
	assert.ErrorContains(err, "ubuntu:24.04\": no image digest found: at line 1")
	assert.ErrorContains(err, "alpine:3.18\": no image digest found: at line 3")

	patched, err := os.ReadFile(dockerfilePath)
	require.NoError(err)

	lines := strings.Split(string(patched), "\n")
	require.Len(lines, 4) // 3 lines + trailing newline
	assert.Equal(strings.Split(content, "\n"), lines)
}

func TestPatchWithSuccessfulVerifyOnly(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	content := "FROM ubuntu:24.04\nRUN echo \"Hello, world!\"\nFROM alpine:3.18\n"
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	require.NoError(os.WriteFile(dockerfilePath, []byte(content), 0644))

	count, _, _, err := dockerfile.Patch(dockerfilePath)
	require.NoError(err)
	assert.Equal(2, count)

	count, verified, total, err := dockerfile.Patch(dockerfilePath, dockerfile.VerifyOnly)
	require.NoError(err)
	assert.Equal(0, count)
	assert.Equal(2, verified)
	assert.Equal(2, total)

	patched, err := os.ReadFile(dockerfilePath)
	require.NoError(err)

	lines := strings.Split(string(patched), "\n")
	require.Len(lines, 4) // 3 lines + trailing newline
	assert.Regexp(`^FROM docker\.io/library/ubuntu@sha256:[0-9a-fA-F]+$`, lines[0])
	assert.Equal("RUN echo \"Hello, world!\"", lines[1])
	assert.Regexp(`^FROM docker\.io/library/alpine@sha256:[0-9a-fA-F]+$`, lines[2])
	assert.Empty(lines[3])
}

func TestV1SchemaDigestNoSkip(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	const content = `FROM quay.io/dexidp/dex:v2.14.0
CMD "dex"
`
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	require.NoError(os.WriteFile(dockerfilePath, []byte(content), 0644))
	_, _, _, err := dockerfile.Patch(dockerfilePath, dockerfile.ImageOptions(images.MinimumAge(time.Hour)))
	assert.ErrorIs(err, images.ErrSchemaV1)
}

func TestV1SchemaDigestSkip(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	const content = `FROM quay.io/dexidp/dex:v2.14.0
CMD "dex"
`
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	require.NoError(os.WriteFile(dockerfilePath, []byte(content), 0644))
	_, _, _, err := dockerfile.Patch(dockerfilePath, dockerfile.ImageOptions(images.MinimumAge(time.Hour)), dockerfile.SkipV1Schema)
	assert.NoError(err)
}
