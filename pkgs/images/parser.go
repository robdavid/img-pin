package images

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/functions"
	"github.com/robdavid/genutil-go/slices"
)

var ErrSkipImage = errors.New("image is skipped")
var ErrPolicyNotFound = errors.New("policy name not found")

type Image struct {
	Registry   string `yaml:"registry,omitempty"`
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag,omitempty"`
	Digest     string `yaml:"digest,omitempty"`
}

type PolicyContext struct {
	Image         *Image
	FallbackImage *Image
	Options       *[]ImageOption
	// Skip indicates that an image should be skipped for a scan, eg. local images
}

type Policy = func(*PolicyContext) error

// AddPolicy returns an image option that adds the provided policy object to the list of policies
func AddPolicy(p Policy) ImageOption {
	return func(o *imageOptions) {
		o.policies = append(o.policies, p)
	}
}

func AddNamedPolicy(name string) ImageOption {
	p := policyRegistry[name]
	return func(o *imageOptions) {
		if p == nil {
			Raise(fmt.Errorf("%s: %w", name, ErrPolicyNotFound))
		} else {
			o.policies = append(o.policies, p)
		}
	}
}

func ClearPolicies(o *imageOptions) {
	o.policies = nil
}

func PolicyList(ps ...Policy) Policy {
	return func(pol *PolicyContext) error {
		for _, p := range ps {
			if err := p(pol); err != nil {
				return err
			}
		}
		return nil
	}
}

func Parse(image string) (img *Image, err error) {
	var ref name.Reference
	img = new(Image)
	image = strings.TrimSpace(image)
	if ref, err = name.ParseReference(image); err != nil {
		return
	}
	img.Registry = ref.Context().RegistryStr()
	img.Repository = ref.Context().RepositoryStr()
	if img.Registry == "index.docker.io" {
		img.Registry = "docker.io"
	}
	if tagRef, ok := ref.(name.Tag); ok {
		img.Tag = tagRef.Identifier()
	} else if digestRef, ok := ref.(name.Digest); ok {
		img.Digest = digestRef.Identifier()
		if strings.HasSuffix(image, "@"+img.Digest) {
			img.Tag, _, err = GetTag(image[:len(image)-len(img.Digest)-1])
			if err != nil {
				return
			}
		} else {
			err = fmt.Errorf("%q: %w: digest suffix not found", image, ErrImageName)
			return
		}
	} else {
		err = fmt.Errorf("%q: %w: unable to parse tag or digest", image, ErrImageName)
	}
	return
}

func (img *Image) String() string {
	var output strings.Builder
	if img.Registry != "" {
		output.WriteString(img.Registry)
		output.WriteByte('/')
	}
	output.WriteString(img.Repository)
	if img.Tag != "" {
		output.WriteByte(':')
		output.WriteString(img.Tag)
	}
	if img.Digest != "" {
		output.WriteByte('@')
		output.WriteString(img.Digest)
	}
	return output.String()
}

func (img *Image) Clone() *Image {
	copy := *img
	return &copy
}

func (img *Image) GroupAndName() (group string, name string) {
	if idx := strings.LastIndex(img.Repository, "/"); idx != -1 {
		return img.Repository[:idx], img.Repository[idx+1:]
	}
	return "", img.Repository
}

func (pol *PolicyContext) ApplyPolicies(policies ...Policy) error {
	for _, policy := range policies {
		if err := policy(pol); err != nil {
			return fmt.Errorf("%q: %w", pol.Image, err)
		}
	}
	return nil
}

func zeroF[T any]() T {
	var zero T
	return zero
}

func ApplyPolicies(image *Image, options ...ImageOption) (pol *PolicyContext, err error) {
	var opts imageOptions
	defer Catch(&err)
	opts.load(options)
	pol = &PolicyContext{Image: image, Options: &options}
	err = pol.ApplyPolicies(opts.policies...)
	return
}

func (img *Image) getOrCheckDigest(options []ImageOption, refreshDigest bool) (
	digested string, digest string, created time.Time, resolvedOptions *imageOptions, err error) {
	var opts imageOptions
	resolvedOptions = &opts
	opts.load(options)
	pol := PolicyContext{Image: img, Options: &options}
	if err = pol.ApplyPolicies(opts.policies...); err != nil {
		return
	}
	if refreshDigest && img.Digest == "" && opts.expectedDigest == "" {
		err = fmt.Errorf("%q: %w", img, ErrNoDigest)
		return
	} else if !refreshDigest && img.Digest != "" && opts.skipTime {
		// Nothing to do
		return
	}

	for i, im := range []*Image{img, pol.FallbackImage} {
		if im == nil {
			continue
		}
		taggedImg := *im
		if refreshDigest && taggedImg.Tag != "" {
			// Only remove the digest if we also have a tag, otherwise we won't be
			// able to resolve the image.
			taggedImg.Digest = ""
		}
		if digested, digest, created, err = Digest(taggedImg.String(), options...); err != nil {
			if opts.skipAuth && errors.Is(err, ErrUnauthorized) {
				err = fmt.Errorf("%q: %w", img, ErrSkipImage)
			}
			if i == 0 && errors.Is(err, ErrImageNotFound) {
				continue
			}
			return
		}
		break
	}
	return
}

func (img *Image) GetDigest(options ...ImageOption) (created time.Time, err error) {
	var digested string
	defer Catch(&err)
	haveDigest := img.Digest != "" // Note we have a digest, but give policy a chance to raise errors
	digested, _, created, _, err = img.getOrCheckDigest(options, false)
	if haveDigest || err != nil {
		return
	}
	var img2 *Image
	if img2, err = Parse(digested); err != nil {
		return
	}
	*img = *img2
	return
}

func (img *Image) VerifyDigest(options ...ImageOption) (err error) {
	var digest string
	var opts *imageOptions
	defer Catch(&err)

	_, digest, _, opts, err = img.getOrCheckDigest(slices.Affix(options, SkipTime), true)
	if err != nil {
		return
	}

	if (opts.expectedDigest != "" && digest != opts.expectedDigest) || (opts.expectedDigest == "" && digest != img.Digest) {
		err = ErrDigestMismatch
		if img.Tag != "" && img.Digest != "" && digest != img.Digest {
			err = fmt.Errorf("%w (%w)", err, ErrTagDrift)
		}
		expectedDigest := functions.IfElse(opts.expectedDigest != "", opts.expectedDigest, img.Digest)
		err = fmt.Errorf("%q: %w: %q != %q", img, err, digest, expectedDigest)
	}
	return
}

func (img *Image) UpdateDigest(options ...ImageOption) (err error) {
	defer Catch(&err)
	_, img.Digest, _, _, err = img.getOrCheckDigest(slices.Affix(options, SkipTime), true)
	if err != nil {
		return
	}
	return
}
