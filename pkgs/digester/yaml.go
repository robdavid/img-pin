package digester

import (
	"fmt"
	"log/slog"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/ferrors"
	"github.com/robdavid/img-pin/pkgs/images"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
)

// YAMLDigester encompases some logic around patching multiple images
// at specified paths in a YAML document.
type YAMLDigester struct {
	imageDigester ImageDigester
}

func MakeYamlDigester(id ImageDigester) YAMLDigester { return YAMLDigester{id} }

func (yd *YAMLDigester) DigestTarget(target yu.PathAndNode, options skipping.ImageOptions) (err error) {
	var imageName string
	var img *images.Image
	defer Handle(func(e error) {
		if img != nil {
			if err = skipping.SkipError(options, e, img); err == nil {
				return
			}
		} else {
			err = e
		}
		if imageName == "" {
			err = fmt.Errorf("%s: %w", target.Path, err)
		} else {
			err = fmt.Errorf("%s: %q: %w", target.Path, imageName, err)
		}
	})
	Check(target.Node.Decode(&imageName))
	img = Try(images.Parse(imageName))
	Try(yd.imageDigester.GetDigest(img, slices.Affix(options.ImageOptions(), images.SkipTime)...))
	newImageName := img.String()
	slog.Debug("digest target at {{.path}}, {{.before}} => {{.after}}", "path", target.Path, "before", imageName, "after", newImageName)
	Check(target.Node.Encode(newImageName))
	return
}

func (yd *YAMLDigester) VerifyTarget(target yu.PathAndNode, options skipping.ImageOptions) (err error) {
	var imageName string
	var img *images.Image
	defer Handle(func(e error) {
		if img != nil {
			if err = skipping.SkipError(options, e, img); err == nil {
				return
			}
		} else {
			err = e
		}
		if imageName == "" {
			err = fmt.Errorf("%s: %w", target.Path, err)
		} else {
			err = fmt.Errorf("%s: %q: %w", target.Path, imageName, err)
		}
	})
	Check(target.Node.Decode(&imageName))
	slog.Debug("verifying target at {{.path}} == {{.image}}", "path", target.Path, "image", imageName)
	img = Try(images.Parse(imageName))
	Check(yd.imageDigester.VerifyDigest(img, options.ImageOptions()...))
	return
}

func (yd *YAMLDigester) processTargets(targets []yu.PathAndNode, options skipping.ImageOptions, processor func(yu.PathAndNode, skipping.ImageOptions) error) (err error) {
	for _, target := range targets {
		if perr := processor(target, options); perr != nil {
			err = ferrors.Join(err, perr)
		}
	}
	return
}

func (yd *YAMLDigester) DigestTargets(targets []yu.PathAndNode, options skipping.ImageOptions) error {
	return yd.processTargets(targets, options, yd.DigestTarget)
}

func (yd *YAMLDigester) VerifyTargets(targets []yu.PathAndNode, options skipping.ImageOptions) error {
	return yd.processTargets(targets, options, yd.VerifyTarget)
}
