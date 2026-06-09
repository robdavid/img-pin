package lock_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/robdavid/img-pin/pkgs/lock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

const testLockYaml = `images:
- source:
    repository: library/ubuntu
    tag: latest
  digest:
    repository: library/ubuntu
    tag: latest
    digest: sha256:deadbeef
  created: 2026-05-26T12:00:00Z
`

func TestLoadSuccess(t *testing.T) {
	defer test.ReportErr(t)

	require := require.New(t)
	assert := assert.New(t)

	dir := t.TempDir()
	fpath := filepath.Join(dir, "lock.yaml")
	err := os.WriteFile(fpath, []byte(testLockYaml), 0644)
	require.NoError(err)

	lf := lock.Lockfile{Filename: fpath}
	err = lf.Load()
	require.NoError(err)
	require.NotNil(lf.Index)
	assert.Equal(1, len(lf.Locks.Images))

	img := lf.Locks.Images[0]
	assert.Equal("library/ubuntu:latest", img.Source.String())
	assert.Equal("sha256:deadbeef", img.Digest.Try().Digest)
	assert.Equal(2026, img.Created.Year())
	assert.Equal(time.May, img.Created.Month())
	assert.Equal(26, img.Created.Day())
	assert.Equal(12, img.Created.Hour())
	assert.Equal("UTC", img.Created.Location().String())
}

func TestLoadMissingFile(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dir := t.TempDir()
	fpath := filepath.Join(dir, "nonexistent.yaml")

	lf := lock.Lockfile{Filename: fpath}
	err := lf.Load()
	require.Error(err)
	assert.True(os.IsNotExist(err))
}

func TestLoadCreateIfMissing(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dir := t.TempDir()
	fpath := filepath.Join(dir, "new.yaml")

	lf := lock.Lockfile{Filename: fpath, CreateIfMissing: true}
	err := lf.Load()
	require.NoError(err)

	_, err = os.Stat(fpath)
	require.NoError(err)

	bytes, err := os.ReadFile(fpath)
	require.NoError(err)
	assert.Equal("images: []\n", string(bytes))
}

func TestTimeRoundTrip(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	now := time.Date(2026, time.May, 26, 12, 0, 0, 0, time.UTC)
	created := lock.Time{Time: now}
	val, err := created.MarshalYAML()
	require.NoError(err)

	var node yaml.Node
	err = node.Encode(val)
	require.NoError(err)

	var decoded lock.Time
	err = decoded.UnmarshalYAML(&node)
	require.NoError(err)
	assert.True(created.Equal(decoded.Time))
}

func TestLoadInvalidYaml(t *testing.T) {
	require := require.New(t)

	dir := t.TempDir()
	fpath := filepath.Join(dir, "bad.yaml")
	err := os.WriteFile(fpath, []byte(":: invalid yaml :: {"), 0644)
	require.NoError(err)

	lf := lock.Lockfile{Filename: fpath}
	err = lf.Load()
	require.Error(err)
}

func TestLockOneImage(t *testing.T) {
	const imageName = "docker.io/goharbor/harbor-portal:v2.11.1"
	assert := assert.New(t)
	require := require.New(t)
	lf := lock.Make()
	lf.Locking = true
	img, err := images.Parse(imageName)
	require.NoError(err)
	counters := make(map[string]int)
	_, err = lf.GetDigest(img, images.RequestCount(counters), images.IncludeTag)
	require.NoError(err)
	assert.NotEmpty(img.Digest)
	assert.True(strings.HasPrefix(img.String(), imageName))
	reqs := counters["index.docker.io"]
	assert.NotZero(reqs)
	digest := img.Digest
	img2, err := images.Parse(imageName)
	_, err = lf.GetDigest(img2, images.RequestCount(counters), images.IncludeTag)
	require.NoError(err)
	assert.Equal(digest, img2.Digest)
	assert.Equal(reqs, counters["index.docker.io"])
	assert.True(strings.HasPrefix(img2.String(), imageName))
}

func TestDigestLockAndVerification(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// Use a consistent image for testing digest calculation flow
	const imageName = "docker.io/goharbor/harbor-portal:v2.11.1"
	const nextImageName = "docker.io/goharbor/harbor-portal:v2.11.2"

	img, err := images.Parse(imageName)
	require.NoError(err)

	var created1, created2 time.Time

	// --- Phase 1: Lock and calculate initial digest (Expected: ~3 requests) ---
	lf := lock.Make()
	counters := make(map[string]int)
	lf.Locking = true

	// First call, requiring external lookup to establish the digest
	created1, err = lf.GetDigest(img, images.RequestCount(counters), images.IncludeTag)
	require.NoError(err)

	digestA := img.Digest

	initialRequests := counters["index.docker.io"]
	// We assert at least 3 requests were made for the initial lock resolution.
	assert.GreaterOrEqual(initialRequests, 3, "Expected at least 3 initial requests for digest resolution when locked")

	// --- Phase 2: Unlock and verify (Expected: No new external requests) ---

	lf.Locking = false // Turn off locking mode
	counters = make(map[string]int)

	img2, err := images.Parse(imageName)
	require.NoError(err)

	// Simulate recalculation/verification call using GetDigest again, relying on cached data.
	created2, err = lf.GetDigest(img2, images.RequestCount(counters), images.IncludeTag)
	require.NoError(err)

	digestB := img2.Digest

	assert.Equal(digestA, digestB)

	// Assert that no new external requests were made since we are unlocked and the digest should be available in memory.
	assert.Zero(counters["index.docker.io"], "Expected zero additional registry requests when unlocking mode and verifying cached digest")

	assert.True(created1.Equal(created2), "%s != %s", created1, created2)

	img3, err := images.Parse(imageName)
	require.NoError(err)

	img2.Digest = ""
	// Simulate recalculation/verification call using GetDigest again, relying on cached data.
	_, err = lf.GetDigest(img2, images.RequestCount(counters), images.IncludeTag)
	require.NoError(err)

	err = lf.VerifyDigest(img3)
	require.NoError(err)

	// Assert that no new external requests were made since we are unlocked and the digest should be available in memory.
	assert.Zero(counters["index.docker.io"], "Expected zero additional registry requests when unlocking mode and verifying cached digest")

	// Try calculating a has on an previously unseen image
	img, err = images.Parse(nextImageName)
	require.NoError(err)

	// Simulate recalculation/verification call using GetDigest again, relying on cached data.
	_, err = lf.GetDigest(img, images.RequestCount(counters), images.IncludeTag)
	require.ErrorIs(err, lock.ErrImageNoLock)

}

// TestLockPredigested asserts behavior when locking an image name that already
// has a digest.
func TestLockPredigested(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	const imageName = "goharbor/harbor-portal:v2.11.2@sha256:24498a84d9fb814e38f8c9d48b83738af79d8c028d49e25137316b968bbd76cc"
	var img1, img2, img3 *images.Image
	var err error
	var created1, created2 time.Time

	lf := lock.Make()
	counters := make(map[string]int)

	// Start by locking a digested file
	lf.Locking = true

	img1, err = images.Parse(imageName)
	require.NoError(err)
	created1, err = lf.GetDigest(img1, images.RequestCount(counters), images.IncludeTag, images.SkipTime)

	// The digests build time should be calculated regardless
	require.NoError(err)
	assert.NotZero(created1)
	requests := counters["index.docker.io"]

	// Look up again without creating new locks
	lf.Locking = false

	img2, err = images.Parse(imageName)
	require.NoError(err)
	created2, err = lf.GetDigest(img2, images.RequestCount(counters), images.IncludeTag, images.SkipTime)

	// We should get the same result, with no new requests
	assert.Equal(img2, img1)
	assert.True(created2.Equal(created1), "%s != %s", created2, created1)

	assert.Equal(requests, counters["index.docker.io"])

	// Verification should now success
	img3, err = images.Parse(imageName)
	require.NoError(err)
	err = lf.VerifyDigest(img3)
	require.NoError(err)
}

func TestGetDigest_SkippedByPolicy(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	lf := lock.Make()
	lf.Locking = true

	// Image that will be skipped by policy
	const imageName = "docker.io/ubuntu:24.04"
	img, err := images.Parse(imageName)
	require.NoError(err)

	// Use SkipPolicy to skip this image
	policy := images.SkipPolicy(images.ImageParts{
		Registry: "docker.io",
		// Group:    "skipped-image",
		Name: "ubuntu",
		Tag:  "24.04",
	})

	// Call GetDigest with the skip policy.
	// The GetDigest implementation in lock.go calls image.GetDigest (via Digest in images.go),
	// which applies policies.
	// Since it's skipped, Digest should return ErrSkipImage.
	// lock.GetDigest should catch this error and create a lock entry with no digest.
	_, err = lf.GetDigest(img, images.AddPolicy(policy))
	assert.ErrorIs(err, images.ErrSkipImage)

	// Verify it's in the index
	assert.True(lf.Index[img.String()] != nil)

	// Now unlock and check it's found and returns ErrSkipImage
	lf.Locking = false
	_, err = lf.GetDigest(img)
	assert.ErrorIs(err, images.ErrSkipImage)

	out, err := yaml.Marshal(&lf.Locks)
	require.NoError(err)
	yaml := string(out)
	assert.Contains(yaml, "source:")
	assert.NotContains(yaml, "digest:")
	buf := bytes.NewBuffer(out)
	io.Copy(os.Stdout, buf)
}
