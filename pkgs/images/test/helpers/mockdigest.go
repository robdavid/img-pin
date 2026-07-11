package helpers

import (
	"fmt"
	"strings"
	"time"

	"github.com/robdavid/img-pin/pkgs/images"
)

type MockDigest struct {
	Sources []string
	Digest  string
	Created time.Time
}

func MakeMockDigest(digest string, created time.Time, sources ...string) MockDigest {
	return MockDigest{
		Sources: sources,
		Digest:  digest,
		Created: created,
	}
}

func MockDigestImage(mocks []MockDigest) images.DigestFunc {
	digestMap := make(map[string]*MockDigest)
	for m := range mocks {
		mock := &mocks[m]
		for _, s := range mock.Sources {
			digestMap[s] = mock
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
		} else {
			const prefix = "sha256:"
			digest = mockDigest.Digest
			if !strings.HasPrefix(digest, prefix) {
				digest = prefix + digest
			}
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
			if min, ok := opts.MinAge.GetOK(); ok && time.Since(created) < min {
				err = images.ErrImageTooRecent
			}
		}

		return
	}
}
