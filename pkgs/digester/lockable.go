package digester

import (
	"time"

	"github.com/robdavid/img-pin/pkgs/images"
)

type ImageDigester interface {
	GetDigest(image *images.Image, options ...images.ImageOption) (created time.Time, err error)
	VerifyDigest(image *images.Image, options ...images.ImageOption) (err error)
}

type NonLockingImageDigester struct{}

func (NonLockingImageDigester) GetDigest(image *images.Image, options ...images.ImageOption) (created time.Time, err error) {
	return image.GetDigest(options...)
}

func (NonLockingImageDigester) VerifyDigest(image *images.Image, options ...images.ImageOption) (err error) {
	return image.VerifyDigest(options...)
}
