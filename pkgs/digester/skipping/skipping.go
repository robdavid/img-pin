package skipping

import (
	"errors"
	"fmt"
	"os"

	"github.com/robdavid/img-pin/pkgs/images"
)

type SkipOptions interface {
	// SkipOnPolicy returns true if the policy skip error should be honored
	SkipOnPolicy(image *images.Image) bool

	// SkipV1Schema returns true if images with V1 schemas should be skipped.
	SkipV1Schema(image *images.Image) bool

	// SkipNoDigest returns true if the image having no digest should be skipped,
	// i.e. if a digest is required but there is a good reason why it can be skipped
	SkipNoDigest(image *images.Image) bool

	SkipNotFound(image *images.Image) bool
}

type ExtendedSkipOptions interface {
	SkipOptions

	// NoteImageSkipped is called when an image is skipped to allow information
	// about skipped images to be recorded.
	NoteImageSkipped(image *images.Image)

	ShouldSkip(image *images.Image) bool
}

type ImageOptions interface {
	images.OptionsHolder
	SkipOptions
}

// SkipError handles skipping logic based on the provided options, error and
// image name. If the image is to be skipped, nil is returned, otherwise
// the original error (or possibly a decorated error). The function may
// emit a warning message to stderr.
func SkipError(skipOptions SkipOptions, err error, image *images.Image) (unSkippedErr error) {
	//var imageString string
	var ok bool
	var extendSkip ExtendedSkipOptions
	noteSkipped := func() {
		if extendSkip != nil {
			extendSkip.NoteImageSkipped(image)
		}
	}
	// if imageString, ok = image.(string); !ok {
	// 	if stringer, ok := image.(fmt.Stringer); ok {
	// 		imageString = stringer.String()
	// 	} else {
	// 		panic("image name is not string-like")
	// 	}
	// }
	if extendSkip, ok = skipOptions.(ExtendedSkipOptions); ok {
		if extendSkip.ShouldSkip(image) {
			return nil
		}
	}

	if errors.Is(err, images.ErrSkipImage) && skipOptions.SkipOnPolicy(image) {
		noteSkipped()
		return nil
	}
	if errors.Is(err, images.ErrSchemaV1) && skipOptions.SkipV1Schema(image) {
		fmt.Fprintf(os.Stderr, "%s: %q: skipping V1 schema image - %s\n", os.Args[0], image, err)
		noteSkipped()
		return nil
	}
	if errors.Is(err, images.ErrImageNotFound) && skipOptions.SkipV1Schema(image) {
		fmt.Fprintf(os.Stderr, "%s: %q: skipping not found image - %s\n", os.Args[0], image, err)
		noteSkipped()
		return nil
	}
	return err
}

func IsSchemaV1(image string) bool {
	_, _, _, err := images.Digest(image)
	return errors.Is(err, images.ErrSchemaV1)
}
