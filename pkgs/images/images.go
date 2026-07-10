package images

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/ferrors"
)

// ErrImageTooRecent is returned when an image is too recent to be considered stable.
var ErrImageTooRecent = errors.New("image age is less than required minimum")

// ErrNoTag is returned when an image reference does not include a tag, but one is required.
var ErrNoTag = errors.New("no image tag tag")

// ErrNoDigest is returned when an image reference does not include a digest, but one is required.
var ErrNoDigest = errors.New("no image digest found")

// ErrUnauthorized is returned when the registry returns a 401 or 403 status code.
var ErrUnauthorized = errors.New("not authorized for docker registry")

// ErrTooManyRequests is returned when the registry returns a 429 status code.
var ErrTooManyRequests = errors.New("too many requests for docker registry")

// ErrLatestTag is returned when the image reference includes the :latest tag or has no tag at all (implying :latest)
// but that is not allowed.
var ErrLatestTag = errors.New("latest tag is not allowed")

// ErrDigestMismatch is returned when an image digest does not match the expected digest.
var ErrDigestMismatch = errors.New("image digest does not match expected digest")

var ErrTagDrift = errors.New("tag's hash has drifted")

// ErrNoImage is returned when an image name cannot be parsed.
var ErrImageName = errors.New("unable to parse image name")

var ErrImageNotFound = errors.New("image not found in registry")

var ErrSchemaV1 = errors.New("image schema V1 unsupported")

type RequestCounters map[string]int

func (rc RequestCounters) String() string {
	var sb strings.Builder
	for registry, count := range rc {
		fmt.Fprintf(&sb, "%s:\t%d requests\n", registry, count)
	}
	return sb.String()
}

type ImageOptions struct {
	MinAge          opt.Val[time.Duration]
	IncludeTag      bool
	RequireTag      bool
	RequireDigest   bool
	SkipTime        bool
	RejectLatest    bool
	SkipAuth        bool
	Policies        []Policy
	ExpectedDigest  string
	Retries         int
	Delay           time.Duration
	Backoff         float64
	RequestCounters RequestCounters
}

func DefaultImageOptions() ImageOptions {
	io := ImageOptions{
		Retries: 5,
		Delay:   2 * time.Second,
		Backoff: 1.5,
	}
	if defaultPolicy != nil {
		io.Policies = []Policy{defaultPolicy}
	}
	return io
}

type ImageOption = func(*ImageOptions)

type OptionsHolder interface {
	ImageOptions() []ImageOption
}

func (io *ImageOptions) Apply(opts []ImageOption) {
	for _, f := range opts {
		f(io)
	}
}

// Load loads up all the options. It may [Raise] errors
func (io *ImageOptions) Load(opts []ImageOption) {
	*io = DefaultImageOptions()
	io.Apply(opts)
}

// BuildOptions creates an [ImageOptions] object from a list
// of [ImageOption] functions.
func BuildOptions(opts []ImageOption) *ImageOptions {
	options := DefaultImageOptions()
	options.Apply(opts)
	return &options
}

// IncludeTag causes the returned digested reference to include the original
// tag, (in the suffix format :tag@sha256:...)if it exists. By default, only the
// digest is included (in the suffix format @sha256:...).
func IncludeTag(io *ImageOptions) { io.IncludeTag = true }

// RequireTag causes ImageDigestedReference to return an error if the provided
// image reference does not include a tag.
func RequireTag(io *ImageOptions) { io.RequireTag = true }

// RequireDigest causes ImageDigestedReference to return an error if the
// provided image reference does not include a digest.
func RequireDigest(io *ImageOptions) { io.RequireDigest = true }

// RejectLatest causes ImageDigestedReference to return an error if the provided
// image reference includes the :latest tag or has no tag at all (implying :latest).
func RejectLatest(io *ImageOptions) { io.RejectLatest = true }

// Skip checking the image creation time, unless MinimumAge is specified. The
// returned creation time will be the zero time. This saves an API call.
func SkipTime(io *ImageOptions) { io.SkipTime = true }

// FetchTime asks the image digester to fetch the image build time. Undoes
// the effect of [SkipTime].
func FetchTime(io *ImageOptions) { io.SkipTime = false }

// Skip digests on images that are in a registry that requires authentication,
// but for which we don't have the credentials.
func SkipAuth(io *ImageOptions) { io.SkipAuth = true }

// MinimumAge causes ImageDigestedReference to return an error if the provided
// image was created less than the provided age ago.
func MinimumAge(age time.Duration) ImageOption {
	return func(io *ImageOptions) {
		if age <= 0 {
			io.MinAge = opt.Empty[time.Duration]()
		} else {
			io.MinAge = opt.Value(age)
		}
	}
}

// ExpectDigest requires the incoming image have a digest.
func ExpectDigest(digest string) ImageOption {
	return func(io *ImageOptions) { io.ExpectedDigest = digest }
}

func Retries(count int, initialDelay time.Duration, backoff float64) ImageOption {
	if backoff < 1 || initialDelay < 0 || count < 0 {
		panic("invalid retry configuration")
	}
	return func(io *ImageOptions) {
		io.Retries = count
		io.Delay = initialDelay
		io.Backoff = backoff
	}
}

func RequestCount(counters map[string]int) ImageOption {
	return func(io *ImageOptions) {
		io.RequestCounters = counters
	}
}

// Digest computes the digest of a tagged image name. If the image has no tag,
// :latest is assumed. If the image already has a digest, none is computed. The
// image name with the digest (digested), the digest itself (digest), the build
// time of the image (created) and an error (err) are returned. It takes zero or
// more options. If [IncludeTag] is specifed, the returned image name will
// include a tag in the format :tag@sha256:hash. If [RequireTag] is specified,
// then an error of [ErrNoTag] is returned if the input image name has no tag
// (or has a digest even with a tag). If [RequireDigest] is specified and the
// image name does contain one, an error of [ErrNoDigest] is returned and no
// digest will be computed. [ErrImageTooRecent] error is returned if the image
// was built more recently than the given age. If [SkipTime] is specfied and
// [MinimumAge] is not specified, the image creation time lookup is skipped, and
// the returned creation time will be the zero.

func Digest(image string, options ...ImageOption) (digested string, digest string, created time.Time, err error) {
	defer Catch(&err)
	var opts ImageOptions
	opts.Load(options)
	retries := max(opts.Retries, 1)
	var attempt int
	for attempt = 0; attempt < retries; {
		digested, digest, created, err = digestDispatch(image, &opts)
		if err == nil || !errors.Is(err, ErrTooManyRequests) || opts.Delay == 0 {
			return
		}
		attempt++
		if attempt < retries {
			fmt.Fprintf(os.Stderr, "%s: request rate error, attempt %d/%d: %s (retrying in %s)\n",
				os.Args[0], attempt, retries, err, opts.Delay)
			time.Sleep(opts.Delay)
			opts.Delay = time.Duration(float64(opts.Delay) * opts.Backoff)
		}
	}
	return
}

func isSchemaV1Err(err error) bool {
	errText := err.Error()
	requiredStrings := []string{
		"unsupported MediaType",
		"application/vnd.docker.distribution.manifest.v1",
		"https://github.com/google/go-containerregistry/issues/377",
	}
	return slices.All(requiredStrings,
		func(req string) bool { return strings.Contains(errText, req) })
}

func digestImage(image string, opts *ImageOptions) (digested string, digest string, created time.Time, err error) {
	var ref name.Reference

	decorateErr := func(e error) error {
		if e == nil {
			return nil
		}
		return fmt.Errorf("%q: %w", image, e)
	}

	defer Handle(func(e error) {
		var transportErr *transport.Error
		if ref != nil && errors.As(e, &transportErr) {
			switch transportErr.StatusCode {
			case http.StatusUnauthorized, http.StatusForbidden:
				err = fmt.Errorf("%w: %s status code %d", ErrUnauthorized, ref.Context().Registry, transportErr.StatusCode)
			case http.StatusTooManyRequests:
				err = fmt.Errorf("%w: %s status code %d", ErrTooManyRequests, ref.Context().Registry, transportErr.StatusCode)
			case http.StatusNotFound:
				err = decorateErr(ErrImageNotFound)
			default:
				err = decorateErr(e)
			}
		} else if isSchemaV1Err(e) {
			err = fmt.Errorf("%q: %w: %w", image, ErrSchemaV1, e)
		} else {
			err = decorateErr(e)
		}
	})

	ref = Try(name.ParseReference(image))
	auth := remote.WithAuthFromKeychain(authn.DefaultKeychain)
	var ok bool
	if _, ok = ref.(name.Digest); ok {
		if opts.RequireTag {
			err = ErrNoTag
		}
		digested = ref.String()
		digest = ref.Identifier()
	} else {
		if opts.RejectLatest && ref.Identifier() == "latest" {
			err = ferrors.Join(err, ErrLatestTag)
		} else if opts.RequireDigest {
			err = ferrors.Join(err, ErrNoDigest)
		} else {
			if opts.RequestCounters != nil {
				opts.RequestCounters[ref.Context().RegistryStr()]++
			}
			digest = Try(remote.Get(ref, auth)).Digest.String()
			if opts.IncludeTag {
				digested = ref.Context().Tag(ref.Identifier() + "@" + digest).String()
			} else {
				digested = ref.Context().Digest(digest).String()
			}
		}
	}
	if !(opts.SkipTime && opts.MinAge.IsEmpty()) {
		if opts.RequestCounters != nil {
			// assuming both of the next two calls generate a request.
			opts.RequestCounters[ref.Context().RegistryStr()] += 2
		}
		img := Try(remote.Image(ref, auth))
		cnf := Try(img.ConfigFile())
		created = cnf.Created.Time
		if minAge, ok := opts.MinAge.GetOK(); ok {
			age := time.Since(created)
			// fmt.Fprintf(os.Stderr, "%q: check age is %s (>=%s: %t)\n", image, age.Round(time.Second), minAge, age >= minAge)
			if age < minAge {
				ageErr := fmt.Errorf("%w: was %s, required %s", ErrImageTooRecent, age.Round(time.Second), minAge.Round(time.Second))
				err = ferrors.Join(err, ageErr)
			}
		}
	}
	err = decorateErr(err)
	return
}

// GetDigest returns the digest of the provided image reference. If the image
// reference already includes a digest, that digest is returned. Otherwise, an
// error of [ErrNoDigest] is returned. The image name without the digest suffix is
// also returned.
// This function makes no API calls to any registry.
func GetDigest(image string) (digest string, stripped string, err error) {
	var ref name.Reference
	image = strings.TrimSpace(image)
	if ref, err = name.ParseReference(image); err != nil {
		return
	}
	if digestRef, ok := ref.(name.Digest); ok {
		digest = digestRef.Identifier()
		if strings.HasSuffix(image, "@"+digest) {
			stripped = image[:len(image)-len(digest)-1]
		} else {
			err = fmt.Errorf("%q: %w: digest suffix not found", image, ErrNoDigest)
		}
	} else {
		stripped = image
		err = ErrNoDigest
	}
	return
}

// GetTag returns the tag of the provided image reference, plus the image name
// with any tag or digest removed. If the image reference includes a tag, that
// tag is returned. If it includes a tag plus a digest, the tag part only is
// returned. If there is no tag, an empty string is returned. An error is
// returned if the digest or tag suffix cannot be parsed.
func GetTag(image string) (tag string, stripped string, err error) {
	var ref name.Reference
	image = strings.TrimSpace(image)
	if ref, err = name.ParseReference(image); err != nil {
		return
	}
	if tagRef, ok := ref.(name.Tag); ok {
		tag = tagRef.Identifier()
		if strings.HasSuffix(image, ":"+tag) {
			stripped = image[:len(image)-len(tag)-1]
		} else if tag == "latest" {
			stripped = image
			tag = ""
		} else {
			err = fmt.Errorf("%q: %w: tag suffix not found", image, ErrNoTag)
		}
	} else {
		digest := ref.Identifier()
		if strings.HasSuffix(image, "@"+digest) {
			return GetTag(image[:len(image)-len(digest)-1])
		} else {
			err = fmt.Errorf("%q: %w: tag suffix not recognized", image, ErrNoTag)
		}
	}
	return
}

// LockImage returns the digested image reference if it doesn't already have a
// digest. If the image reference already includes a digest, it is returned as
// is. This is a wrapper around [Digest] with [SkipTime] applied, so it makes no
// API calls to check the image creation time, unless [MinimumAge] is also
// specified.
func LockImage(image string, options ...ImageOption) (string, error) {
	options = slices.Affix(options, SkipTime)
	digested, _, _, err := Digest(image, options...)
	return digested, err
}

// VerifyImage checks that the provided image reference has the expected digest. If
// the image reference does not include a digest, an error of [ErrNoDigest] is
// returned, unless expectedDigest is provided. If expectedDigest is provided,
// the latest digest must match. A new image digest is obtained from the
// registry from the image without its digest suffix if it has one, but with the
// original tag if it also has that. If the image carries a digest and
// expectedDigest is also provided they must match or else the function will
// fail immediately returning an [ErrDigestMismatch] error.
func VerifyImage(image string, expectedDigest string, options ...ImageOption) error {
	options = slices.Affix(options, SkipTime, RejectLatest)
	digest, stripped, err := GetDigest(image)
	if err != nil {
		if !(errors.Is(err, ErrNoDigest) && expectedDigest != "") {
			return fmt.Errorf("%q: %w", image, err) // Includes no digest to verify; neither in the image nor expected.
		}
		stripped = image // No digest in the image, but expected digest provided, so we'll verify against that.
	} else if expectedDigest != "" && digest != expectedDigest {
		return fmt.Errorf("%q: %w: %q != %q", image, ErrDigestMismatch, digest, expectedDigest)
	} else {
		expectedDigest = digest
	}
	var newDigest string
	var hasTag bool
	_, newDigest, _, err = Digest(stripped, options...)
	if err != nil {
		if errors.Is(err, ErrLatestTag) && digest != "" {
			// Image with no tag (or latest) and a digest is OK, but re-resolve for age
			_, newDigest, _, err = Digest(image, options...)
		}
		if err != nil {
			return fmt.Errorf("%q: %w", image, err)
		}
	} else {
		hasTag = true
	}
	if expectedDigest != "" && newDigest != expectedDigest {
		err = ErrDigestMismatch
		if hasTag {
			err = fmt.Errorf("%w (%w)", err, ErrTagDrift)
		}
		return fmt.Errorf("%q: %w: %q != %q", image, err, newDigest, expectedDigest)
	}
	return nil
}

func UpdateImage(image string, options ...ImageOption) (string, error) {
	decorateErr := func(e error) error {
		return fmt.Errorf("%q: %w", image, e)
	}
	_, stripped, err := GetDigest(image)
	if err != nil {
		return "", decorateErr(err)
	}
	newImage, _, _, err := Digest(stripped, slices.Affix(options, SkipTime)...)
	if err != nil {
		return "", decorateErr(err)
	}
	return newImage, nil
}
