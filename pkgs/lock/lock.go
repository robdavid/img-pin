package lock

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/images"
)

var ErrImageNoLock = errors.New("no lock info found for image")

type Time struct {
	time.Time
}

func (t Time) MarshalYAML() (any, error) {
	return t.Format(time.RFC3339), nil
}

func (t *Time) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("invalid ISO 8601 time %q: %w", s, err)
	}
	t.Time = parsed
	return nil
}

type ImageData struct {
	Source            images.Image          `yaml:"source"`
	Digest            opt.Ref[images.Image] `yaml:"digest,omitempty"`
	Created           Time                  `yaml:"created,omitempty"`
	UnsupportedSchema int                   `yaml:"schemaVersion,omitempty"`
}

func img2StringPtr(i *images.Image) *string { return new(i.String()) }

func (id *ImageData) String() string {
	return fmt.Sprintf("%s => %s (%s %v)",
		&id.Source,
		opt.MapRef(id.Digest, img2StringPtr).GetOr("none"),
		id.Created, id.UnsupportedSchema)
}

type LockData struct {
	Images []ImageData `yaml:"images"`
}

type LockIndex map[string]*ImageData

type Lockfile struct {
	Filename        string
	Locks           LockData
	Locking         bool
	CreateIfMissing bool
	Index           LockIndex
}

func NewLockfile(filename string) *Lockfile {
	return &Lockfile{Filename: filename}
}

// New creates an empty lockfile object pointer, that can lock and verify
// images, but cannot [Lockfile.Load] or [Lockfile.Save].
func New() *Lockfile {
	return &Lockfile{}
}

// New creates an empty lockfile object, that can lock and verify images, but
// cannot [Lockfile.Load] or [Lockfile.Save].
func Make() Lockfile {
	return Lockfile{}
}

func (lf *Lockfile) index() {
	lf.Index = make(LockIndex)
	for i := range lf.Locks.Images {
		key := lf.Locks.Images[i].Source.String()
		lf.Index[key] = &lf.Locks.Images[i]
		if dig, ok := lf.Locks.Images[i].Digest.RefOK(); ok {
			key2 := dig.String()
			lf.Index[key2] = &lf.Locks.Images[i]
		}
	}
}

func (lf *Lockfile) Load() error {
	bytes, err := os.ReadFile(lf.Filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && lf.CreateIfMissing {
			emptyData := LockData{}
			var out []byte
			var err error
			if out, err = yaml.Marshal(&emptyData); err != nil {
				return err
			}
			return os.WriteFile(lf.Filename, out, 0644)
		}
		return err
	}
	if err := yaml.Unmarshal(bytes, &lf.Locks); err != nil {
		return err
	}
	lf.index()
	return nil
}

func (lf *Lockfile) Save() error {
	if out, err := yaml.Marshal(&lf.Locks); err != nil {
		return err
	} else {
		return os.WriteFile(lf.Filename, out, 0644)
	}
}

func imageKey(image *images.Image) string {
	keyImage := *image
	// keyImage.Digest = "" // Also create locks for pre-digested images
	return keyImage.String()
}

// GetDigest obtains a digest for an image, either from the lock file data or
// from requests to the registry, if [Lockfile.Locking] is true. When
// [Lockfile.Locking] is true, any missing image digests are requested from the
// registry and added to the lock file. No existing lock data for the same image
// and tag are replaced.
//
// TODO: Whatever tag options (e.g. [images.IncludeTag]) are used for the
// initial locking call will persist in the lock file and propagate to the
// result of subsequent calls to this method regardless of the tag options used
// in that subsequent call.
func (lf *Lockfile) GetDigest(image *images.Image, options ...images.ImageOption) (created time.Time, err error) {
	if lf.Locking {
		imageKey := imageKey(image)
		slog := slog.With("key", imageKey)
		var imageData ImageData
		if locked := lf.Index[imageKey]; locked != nil {
			slog.Debug("not changing {{.key}} which already has a digest")
			if locked.Digest.HasValue() {
				*image = locked.Digest.Get()
			}
			return
		}
		if lf.Index == nil {
			lf.Index = make(LockIndex)
		}
		imageData.Source = *image
		if created, err = image.GetDigest(slices.Affix(options, images.FetchTime)...); err != nil {
			if errors.Is(err, images.ErrSchemaV1) {
				imageData.UnsupportedSchema = 1
				_, err = image.GetDigest(options...)
			}
		}
		if err != nil {
			// This image digest has failed. This may have failed for a reason the caller
			// considers to be non-fatal, so the logic here is to simply create a lock entry
			// with no digest. If the lock file is eventually written, it will have captured
			// this information. If the error was fatal, the lock data is ultimately discarded
			// and this bad digest will not persist.
			slog.Debug("locking image digest: {{.key}}: no digest")
		} else {
			slog = slog.With("digest", &imageData.Digest)
			imageData.Digest = opt.Reference(image.Clone())
			imageData.Created = Time{created}
			slog.Debug("locking image digest: {{.key}}: {{.digest}}")
		}
		if lf.Index == nil {
			lf.Index = make(LockIndex)
		}
		lf.Locks.Images = append(lf.Locks.Images, imageData)
		lf.Index[imageKey] = &lf.Locks.Images[len(lf.Locks.Images)-1]
		lf.Index[imageData.Digest.String()] = &lf.Locks.Images[len(lf.Locks.Images)-1]
	} else {
		imageKey := imageKey(image)
		imageData := lf.Index[imageKey]
		slog.Debug("lockfile lookup of {{.key}} gives {{.digest}}", "key", imageKey, "digest", imageData)
		if imageData == nil {
			err = fmt.Errorf("%q: %w", image, ErrImageNoLock)
			return
		}
		if imageData.Digest.HasValue() {
			*image = imageData.Digest.Get()
			slog.Debug("retrieved digest from lock file: {{.digest}}", "digest", image)
		} else {
			// Assumption: this digest was non-fatally skipped when evaluated.
			// Therefore we will return ErrSkipImage here.
			err = images.ErrSkipImage
			slog.Debug("no digest in lock file for: {{.image}}; assuming ErrSkipImage", "image", image)
		}
		created = imageData.Created.Time
		if imageData.UnsupportedSchema == 1 {
			err = images.ErrSchemaV1
		}
	}
	return
}

func (lf *Lockfile) VerifyDigest(image *images.Image, options ...images.ImageOption) (err error) {
	imageKey := imageKey(image)
	imageData := lf.Index[imageKey]
	slog.Debug("lockfile lookup of {{.key}} gives {{.digest}}", "key", imageKey, "digest", imageData)
	if imageData == nil {
		err = fmt.Errorf("%q: %w", image, ErrImageNoLock)
	} else if digest, ok := imageData.Digest.RefOK(); !ok || *image != *digest {
		err = fmt.Errorf("%q: %w", image, images.ErrNoDigest)
	}
	return
}
