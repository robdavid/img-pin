package helpers

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/robdavid/img-pin/pkgs/images"
)

type MockDigest struct {
	Sources []string
	Digest  string
	Created time.Time
	Err     error
}

var reDigest = regexp.MustCompile(`[0-9a-z]{64}`)
var reImage = regexp.MustCompile(`(:?[a-z0-9\.-]+/)?[a-z0-9-]+/[a-z0-9-]+:[0-9a-z\.\+]+`)

func (md *MockDigest) validate() {
	if md.Digest != "" && !reDigest.MatchString(md.Digest) {
		panic(fmt.Errorf("bad digest string %q (does not match %s)", md.Digest, reDigest.String()))
	}
	for _, source := range md.Sources {
		if !reImage.MatchString(source) {
			panic(fmt.Errorf("bad image string %q (does not match %s)", source, reImage.String()))
		}
	}
}

func MakeMockDigest(digest string, created time.Time, sources ...string) MockDigest {
	md := MockDigest{
		Sources: sources,
		Digest:  digest,
		Created: created,
	}
	md.validate()
	return md
}

func MakeMockDigestErr(err error, sources ...string) MockDigest {
	md := MockDigest{
		Sources: sources,
		Err:     err,
	}
	md.validate()
	return md
}

const sha256Prefix = "sha256:"

func MockDigestImage(mocks []MockDigest) images.DigestFunc {
	digestMap := make(map[string]*MockDigest)
	for m := range mocks {
		mock := &mocks[m]
		for _, s := range mock.Sources {
			digest := mock.Digest
			digestMap[s] = mock
			digestMap[s+"@"+sha256Prefix+digest] = mock
			if t := strings.LastIndex(s, ":"); t >= 0 {
				digestMap[s[:t]+"@"+sha256Prefix+digest] = mock
			}
		}
	}
	return func(image string, opts *images.ImageOptions) (digested string, digest string, created time.Time, err error) {
		defer func() {
			if err != nil {
				err = fmt.Errorf("%q: %w", image, err)
			}
		}()
		if mockDigest, ok := digestMap[image]; !ok {
			err = images.ErrImageNotFound
		} else if mockDigest.Err != nil {
			err = mockDigest.Err
		} else {
			digest = mockDigest.Digest
			if !strings.HasPrefix(digest, sha256Prefix) {
				digest = sha256Prefix + digest
			}
			image := mockDigest.Sources[0]
			created = mockDigest.Created
			if pos := strings.LastIndex(image, ":"); pos >= 0 {
				if opts.RejectLatest && image[pos+1:] == "latest" {
					err = images.ErrLatestTag
				} else if opts.IncludeTag {
					digested = image + "@" + digest
				} else {
					digested = image[:pos] + "@" + digest
				}
			} else if opts.RejectLatest {
				err = images.ErrLatestTag
			} else if opts.RequireTag {
				err = images.ErrNoTag
			} else if opts.IncludeTag {
				digested = image + ":latest@" + digest
			} else {
				digested = image + "@" + digest
			}
			if opts.RequestCounters != nil && !strings.Contains(image, digest) {
				parts := strings.Split(mockDigest.Sources[0], "/")
				registry := parts[0]
				if registry == "docker.io" {
					registry = "index.docker.io"
				}
				opts.RequestCounters[registry] += 3
			}
			if min, ok := opts.MinAge.GetOK(); ok && time.Since(created) < min {
				err = images.ErrImageTooRecent
			}
		}

		return
	}
}
