package lock

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/images"
)

var (
	ErrImageNoLock = errors.New("no lock info found for image")
	ErrNoFileName  = errors.New("lock file has no file name")
)

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

// Lockfile represents an image lock file containing mappings from
// original provided image names to their pinned equivalents.
type Lockfile struct {
	// Filename is the name of the underlying lock file (may be empty)
	Filename string
	// Locks contains the main locking data; this object is marshalled as YAML
	// when writing the lock file.
	Locks LockData
	// Locking is true when digests are being computed from registries
	Locking bool
	// CreateIfMissing indicates whether a new lock file is to be created by
	// [Lockfile.Save] if it does not already exist,
	CreateIfMissing bool
	// Index contains a map of image names to their associated lock data.
	Index LockIndex
	// Updating indicates that existing digests can be updated from an existing
	// tagged image.
	Updating bool
	// UpdateOnly, when non-nil, defines a set of tagged images that are allowed
	// to be updated when [Lockfile.Update] is true.
	UpdateOnly map[string]bool
}

// NewLockFile creates a new empty [Lockfile] to be stored at the
// provided file name, which may or may not exist. No attempt is
// made to load the file. This can be done with the [Lockfile.Load]
// method, or the lockfile can be populated with data and the
// file created/overwritten with [Lockfile.Save].
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

// Load loads lock file data from its file name. If the file name
// is empty, an [ErrNoFileName] error is returned. If the file
// does not exist and [Lockfile.CreateIfMissing] is set, a new
// empty file is created, and the lock data is zeroised. Otherwise,
// a missing file returns an error.
func (lf *Lockfile) Load() error {
	if lf.Filename == "" {
		return fmt.Errorf("%w, cannot load", ErrNoFileName)
	}
	defer lf.index()
	bytes, err := os.ReadFile(lf.Filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && lf.CreateIfMissing {
			emptyData := LockData{}
			var out []byte
			var err error
			if out, err = yaml.Marshal(&emptyData); err != nil {
				return err
			}
			if err := os.WriteFile(lf.Filename, out, 0644); err != nil {
				return err
			}
			lf.Locks = emptyData
			return nil
		}
		return err
	}
	if err := yaml.Unmarshal(bytes, &lf.Locks); err != nil {
		return err
	}
	return nil
}

// AsYAML returns the lockfile YAML text as a byte array
func (lf *Lockfile) AsYAML() (yml []byte, err error) {
	yml, err = yaml.Marshal(&lf.Locks)
	return
}

// Saves the [Lockfile] data to its filename. If the file name
// is empty, an error is returned.
func (lf *Lockfile) Save() error {
	if out, err := yaml.Marshal(&lf.Locks); err != nil {
		return err
	} else if lf.Filename == "" {
		return fmt.Errorf("%w, cannot save", ErrNoFileName)
	} else {
		return os.WriteFile(lf.Filename, out, 0644)
	}
}

// Saves the [Lockfile] data to the provided yaml Encoder
func (lf *Lockfile) SaveTo(encoder yaml.Encoder) error {
	return encoder.Encode(&lf.Locks)
}

func imageKey(image *images.Image) string {
	return image.String()
}

func (lf *Lockfile) upsert(slog *slog.Logger, image *images.Image, imageKey string, options ...images.ImageOption) (created time.Time, err error) {
	var imageData ImageData
	imageData.Source = *image
	if created, err = image.GetDigest(slices.Affix(options, images.FetchTime)...); err != nil {
		if errors.Is(err, images.ErrSchemaV1) {
			imageData.UnsupportedSchema = 1
			_, err = image.GetDigest(options...)
		}
	}
	if err != nil {
		// This image digest has failed. This may have failed for a reason
		// the caller will consider to be non-fatal, so the logic here is to
		// simply create a lock entry with no digest. If the lock file is
		// eventually written, it will have captured this information. If
		// the error was fatal, the lock data is ultimately discarded and
		// this bad digest will non persist.
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
	if lockImage := lf.Index[imageKey]; lockImage != nil {
		*lockImage = imageData
	} else {
		lf.Locks.Images = append(lf.Locks.Images, imageData)
		lf.Index[imageKey] = &lf.Locks.Images[len(lf.Locks.Images)-1]
		lf.Index[imageData.Digest.String()] = &lf.Locks.Images[len(lf.Locks.Images)-1]
	}
	return
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

	imageKey := imageKey(image)
	slog := slog.With("key", imageKey)
	var locked *ImageData
	if lf.Index == nil {
		lf.Index = make(LockIndex)
		locked = nil
	} else {
		locked = lf.Index[imageKey]
	}

	if lf.Updating && locked != nil {
		if lf.UpdateOnly == nil || lf.UpdateOnly[imageKey] {
			return lf.upsert(slog, image, imageKey, options...)
		} else {
			slog.Debug("not updating {{.key}} which is not to be updated")
		}
	} else if lf.Locking {
		if locked == nil {
			return lf.upsert(slog, image, imageKey, options...)
		} else {
			slog.Debug("not changing {{.key}} which already has a digest")
		}
	}

	slog.Debug("lockfile lookup of {{.key}} gives {{.digest}}", "key", imageKey, "digest", locked)
	if locked == nil {
		err = fmt.Errorf("%q: %w", image, ErrImageNoLock)
		return
	}
	if locked.Digest.HasValue() {
		*image = locked.Digest.Get()
		slog.Debug("retrieved digest from lock file: {{.digest}}", "digest", image)
	} else {
		err = images.ErrSkipImage
		slog.Debug("retrieved digest from lock file: {{.digest}}", "digest", image)
	}
	created = locked.Created.Time
	if locked.UnsupportedSchema == 1 {
		err = images.ErrSchemaV1
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
