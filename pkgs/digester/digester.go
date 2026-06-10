package digester

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/ferrors"
	"github.com/robdavid/img-pin/pkgs/files"
	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/robdavid/img-pin/pkgs/lock"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"gopkg.in/yaml.v3"
)

var (
	ErrUnexpectedResourceType = errors.New("unexpected resource type encountered")
	ErrNoFileWrite            = errors.New("cannot write, input is a stream")
	ErrRoundTrip              = errors.New("output file has not preserved all the data from the input file")
	ErrLockFileName           = errors.New("no lock file name was provided or could be inferred")
)

type DeploymentLoader interface {
	// Filename returns the file path of the file that describes one or
	// more deployments
	Filename() string

	// Loads the deployments associated with this file
	LoadDeployments() ([]types.Deployment, error)
}

type SimpleResource struct {
	Node *yaml.Node
}

func (r *SimpleResource) Load(node *yaml.Node) error {
	r.Node = node
	return nil
}

func (r *SimpleResource) Save() (*yaml.Node, error) {
	return r.Node, nil
}

func (r *SimpleResource) Cleanup() error {
	return nil
}

func (*SimpleResource) CanDigest() bool                 { return false }
func (*SimpleResource) Digest() error                   { panic("Digest called on SimpleResource - not supported") }
func (*SimpleResource) Verify() error                   { panic("Verify called on SimpleResource - not supported") }
func (r *SimpleResource) Expand() ([]*yaml.Node, error) { return []*yaml.Node{r.Node}, nil }

type Options struct {
	updateMethod   types.UpdateMethod
	skipPostVerify bool
	skipV1Schema   bool
	skipNotFound   bool
	trimMultiline  bool
	useLockfile    bool
	mustLockfile   bool
	generateLocks  bool
	noWrite        bool
	lockFileName   string
	imageOptions   []images.ImageOption
}

func (o *Options) ImageOptions() []images.ImageOption { return o.imageOptions }

type Option func(*Options)

func (o *Options) apply(options []Option) {
	for _, f := range options {
		f(o)
	}
}

// UpdateMethod changes the way a digested file is modified on disk
// according to the [types.UpdateMethod] parameter provided.
func UpdateMethod(updateMethod types.UpdateMethod) Option {
	return func(o *Options) { o.updateMethod = updateMethod }
}

// ImageOptions are passed through to the image digest functions, eg. [images.Image.GetDigest].
func ImageOptions(imageOptions ...images.ImageOption) Option {
	return func(o *Options) { o.imageOptions = slices.Affix(o.imageOptions, imageOptions...) }
}

// SkipPostVerify causes the digester to not do a post verification. Normally, the output
// from a call to [CreateDigests] will verify the output to ensure no undigested images
// remain. This option skips this step, allowing any partially patched output
// to be viewed.
func SkipPostVerify(o *Options) { o.skipPostVerify = true }

// TrimMultiline will pre-process all YAML documents to identify literal multiline strings
// that have trailing whitespace before the carriage return and strip it. These kinds of
// strings will not round trip properly as the YAML parser will alway emit normal strings
// with embedded \n escape characters for compatibility reasons.
func TrimMultiline(o *Options) { o.trimMultiline = true }

// SkipV1Schema will cause digesting V1 schema based images to be skipped if they return
// an error. An error may be avoided by not imposing a minimum age criterion on the image
// thereby obviating the need to fetch the required metadata.
func SkipV1Schema(o *Options) { o.skipV1Schema = true }

// SkipNotFound causes images that cannot be found to be ignored.
func SkipNotFound(o *Options) { o.skipNotFound = true }

// GenerateLocks tells the digester process to consult registries for digests and update
// the lock file.
func GenerateLocks(o *Options) { o.generateLocks = true }

// UseLockfile makes all digesting or verification use the associated lock file, if it exists. No API
// calls to registries are made if the lock file exists.
func UseLockfile(o *Options) { o.useLockfile = true }

// MustLockFile raises an error if [UseLockFile] is specified and there is no lock file present.
func MustLockFile(o *Options) { o.mustLockfile = true }

// LockFileName defines the name of the lock file that will be used by the [Digester]. By
// default, lock file names are derived from the input file name (with extension replaced)
// by ".lock.yaml". This option override this behavior and will use this file for all locks.
// This option is required if the input is standard input.
func LockFileName(name string) Option {
	return func(o *Options) {
		o.lockFileName = name
	}
}

// NoWrite configures [CreateDigests] to not emit or write any updated values, but just calculate the
// digests. This is useful when only populating a lock file.
func NoWrite(o *Options) { o.noWrite = true }

type Digester struct {
	Filename      string
	options       Options
	Resources     []types.Resource
	Docs          []*yaml.Node
	DigestedDocs  []*yaml.Node
	skipped       map[string]bool
	lockfile      *lock.Lockfile
	imageDigester ImageDigester
}

func NewDigester(options ...Option) *Digester {
	y := Digester{}
	y.options.apply(options)
	y.imageDigester = NonLockingImageDigester{}
	return &y
}

func NewVerificationDigester(d *Digester) *Digester {
	v := Digester{}
	v.options = d.options
	v.skipped = d.skipped
	v.Filename = "(" + d.Filename + " buffer)"
	v.imageDigester = d.imageDigester
	return &v
}

func NewSubDigester(d *Digester) *Digester {
	v := Digester{}
	v.options = d.options
	v.skipped = d.skipped
	v.Filename = d.Filename
	v.imageDigester = d.imageDigester
	return &v
}

func (ky *Digester) SkipOnPolicy(*images.Image) bool { return true }
func (ky *Digester) SkipNoDigest(*images.Image) bool { return false }
func (ky *Digester) SkipV1Schema(*images.Image) bool { return ky.options.skipV1Schema }
func (ky *Digester) SkipNotFound(*images.Image) bool { return ky.options.skipNotFound }
func (ky *Digester) NoteImageSkipped(image *images.Image) {
	if ky.skipped == nil {
		ky.skipped = make(map[string]bool)
	}
	ky.skipped[image.String()] = true
}
func (ky *Digester) ShouldSkip(image *images.Image) bool {
	return ky.skipped[image.String()]
}
func (ky *Digester) ImageOptions() []images.ImageOption { return ky.options.imageOptions }

func (ky *Digester) lockFileName() (string, error) {
	if ky.options.lockFileName != "" {
		return ky.options.lockFileName, nil
	}
	if ky.Filename == "" || ky.Filename == "-" {
		return "", ErrLockFileName
	}
	ext := filepath.Ext(ky.Filename)
	return ky.Filename[:len(ky.Filename)-len(ext)] + ".lock.yaml", nil
}

func (ky *Digester) configureLockfile() error {
	if ky.options.useLockfile || ky.options.generateLocks {
		if lockFileName, err := ky.lockFileName(); err != nil {
			if ky.options.mustLockfile {
				return err
			}
		} else {
			ky.lockfile = lock.NewLockfile(lockFileName)
			ky.lockfile.CreateIfMissing = ky.options.generateLocks
			ky.lockfile.Locking = ky.options.generateLocks
			if err := ky.lockfile.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			} else if err != nil && ky.options.mustLockfile {
				return err
			} else if err == nil {
				slog.Debug("using lock file: {{.file}}", "file", lockFileName)
				ky.imageDigester = ky.lockfile
				return err
			}
		}
	}
	slog.Debug("not using lock file")
	ky.imageDigester = NonLockingImageDigester{}
	return nil
}

func (ky *Digester) LoadFile(filename string) (err error) {
	defer Catch(&err)
	ky.Filename = filename
	Check(ky.configureLockfile())
	if filename == "-" {
		Check(ky.Read(os.Stdin))
	} else {
		input := Try(os.Open(filename))
		defer input.Close()
		Check(ky.Read(input))
	}
	return
}

func (ky *Digester) WriteAnyLocks() error {
	if ky.lockfile != nil && ky.options.generateLocks {
		return ky.lockfile.Save()
	}
	return nil
}

func (ky *Digester) Read(input io.Reader) (err error) {
	if ky.Docs, err = yu.StreamDocsIn(input); err != nil {
		return
	}
	log := slog.With("file", ky.Filename, "ndocs", len(ky.Docs))
	log.Debug("found {{.ndocs}} document(s) in {{.file}}")
	return ky.ReadDocs()
}

func (ky *Digester) ReadDocs() (err error) {
	log := slog.With("file", ky.Filename, "ndocs", len(ky.Docs))
	ky.Resources = make([]types.Resource, len(ky.Docs))
nextDoc:
	for n, doc := range ky.Docs {
		log := log.With("ndoc", n)
		if ky.options.trimMultiline {
			yu.TrimMultiline(doc)
		}
		for _, reg := range registry {
			if K8S_LIST.Match(doc) {
				subD := NewSubDigester(ky)
				listRes := DigesterResource{digester: subD}
				Check(listRes.Load(doc))
				ky.Resources[n] = listRes
				log.Debug("{{.file}}: document {{.ndoc}} is a List")
				continue nextDoc
			}
			if handler := reg.Handler.Match(doc, ky.imageDigester, ky); handler != nil {
				Check(handler.Load(doc))
				ky.Resources[n] = handler
				log.Debug("{{.file}}: document {{.ndoc}} is a {{.regname}}", "regname", reg.Name)
				continue nextDoc
			}
		}
		log.Debug("{{.file}}: document {{.ndoc}} is a generic Kubernetes resource")
		ky.Resources[n] = &SimpleResource{doc}
		Check(ky.Resources[n].Load(doc))
	}
	return
}

func (ky *Digester) ExpandResources() (err error) {
	newDocs := make([]*yaml.Node, 0, len(ky.Docs))
	for _, resource := range ky.Resources {
		var edocs []*yaml.Node
		if edocs, err = resource.Expand(); err != nil {
			return
		}
		newDocs = append(newDocs, edocs...)
	}
	ky.Docs = newDocs
	ky.Cleanup()

	return ky.ReadDocs()
}

func (ky *Digester) CreateDigests() (err error) {
	log := slog.With("file", ky.Filename)
	for n, r := range ky.Resources {
		log := log.With("ndoc", n)
		if r.CanDigest() {
			log.Debug("{{.file}}: digesting document {{.ndoc}}")
			if err := r.Digest(); err != nil {
				return err
			}
		}
	}
	log.Debug("{{.file}}: digestion complete")
	ky.DigestedDocs = slices.Map(ky.Resources,
		func(r types.Resource) *yaml.Node { return Try(r.Save()) })
	return
}

func (ky *Digester) VerifyDigests() (err error) {
	log := slog.With("file", ky.Filename)
	for n, r := range ky.Resources {
		log := log.With("ndoc", n)
		if r.CanDigest() {
			log.Debug("{{.file}}: verifying document {{.ndoc}} which is a Kubernetes resource with images")
			verrerr := r.Verify()
			err = ferrors.Join(err, verrerr)
		}
	}
	return
}

func (ky *Digester) WriteFile() (err error) {
	defer Catch(&err)
	if ky.Filename == "" {
		return ErrNoFileWrite
	} else if ky.Filename == "-" {
		return ky.Write(os.Stdout)
	}
	var original *os.File
	if ky.options.updateMethod != types.UpdateOverwrite {
		original = Try(os.Open(ky.Filename))
		defer original.Close()
	}
	output := Try(files.OpenForOverwrite(ky.Filename))
	defer output.Close()
	Check(ky.WriteUsingMethod(original, output))
	output.AllowOverwrite(true)
	return nil
}

func (ky *Digester) Write(output io.Writer) (err error) {
	defer Catch(&err)
	docs := slices.Map(ky.Resources, func(r types.Resource) *yaml.Node { return Try(r.Save()) })
	Check(yu.StreamDocsOut(output, docs...))
	return
}

func (ky *Digester) WriteUsingMethod(original io.Reader, output io.Writer) (err error) {
	defer Catch(&err)
	method := ky.options.updateMethod
	if original == nil {
		method = types.UpdateOverwrite
	}
	switch method {
	case types.UpdatePatch:
		Check(yu.PatchDocsOut(string(Try(io.ReadAll(original))), output, ky.DigestedDocs...))
	case types.UpdateSync:
		Check(yu.SyncWrite(original, output, ky.DigestedDocs...))
	default:
		Check(yu.StreamDocsOut(output, ky.DigestedDocs...))
	}
	return nil
}

func (ky *Digester) Cleanup() (err error) {
	for _, doc := range ky.Resources {
		if doc != nil {
			err = ferrors.Join(err, doc.Cleanup())
		}
	}
	return
}

func compareDocs(ds1 []*yaml.Node, ds2 []*yaml.Node) error {
	if len(ds1) != len(ds2) {
		return ErrRoundTrip
	}
	for i := range ds1 {
		d1 := ds1[i]
		d2 := ds2[i]
		kind := yu.Get[string](d1, "kind").GetOr("")
		apiVersion := yu.Get[string](d1, "apiVersion").GetOr("")
		if kind == "HelmChart" && apiVersion == "helm.cattle.io/v1" {
			// Special case for HelmChart resource; it embeds a valuesContent string field which itself
			// is a YAML document, and must be compared as such
			if kind != yu.Get[string](d2, "kind").GetOr("") || apiVersion != yu.Get[string](d2, "apiVersion").GetOr("") {
				return ErrRoundTrip
			}
			chartPaths := []yu.Path{yu.PathOf("spec", "chart"), yu.PathOf("spec", "repo"), yu.PathOf("spec", "version")}
			for _, path := range chartPaths {
				if yu.GetPath[string](d1, path) != yu.GetPath[string](d2, path) {
					return ErrRoundTrip
				}
			}
			values1 := yu.Get[string](d1, "spec", "valuesContent")
			values2 := yu.Get[string](d2, "spec", "valuesContent")
			if values1.HasValue() != values2.HasValue() {
				return ErrRoundTrip
			} else if values1.HasValue() {
				var v1, v2 yaml.Node
				err1 := yaml.Unmarshal([]byte(values1.Get()), &v1)
				err2 := yaml.Unmarshal([]byte(values2.Get()), &v2)
				if err1 == nil && err2 == nil {
					if !yu.EqualData(&v1, &v2) {
						return ErrRoundTrip
					}
				}
			}
		} else if !yu.EqualData(d1, d2) {
			return ErrRoundTrip
		}
	}
	return nil
}

func CreateDigests(filename string, options ...Option) (err error) {
	defer Handle(func(e error) {
		err = fmt.Errorf("%s: %w", filename, e)
	})
	slog := slog.With("file", filename)
	digester := NewDigester(options...)
	defer digester.Cleanup()
	Check(digester.LoadFile(filename))
	Check(digester.CreateDigests())
	if !digester.options.skipPostVerify {
		var buffer bytes.Buffer
		var original io.ReadCloser
		if filename != "-" {
			original = Try(os.Open(filename))
			defer original.Close()
		}
		Check(digester.WriteUsingMethod(original, &buffer))
		verifier := NewVerificationDigester(digester)
		defer verifier.Cleanup()
		slog.Debug("{{.file}}: performing verification pass")
		Check(verifier.Read(&buffer))
		Check(compareDocs(digester.DigestedDocs, verifier.Docs))
		Check(verifier.VerifyDigests())
		slog.Info("{{.file}}: round-trip verified {{.ndocs}} docs", "ndocs", len(verifier.Docs))
	}
	if !digester.options.noWrite {
		slog.Debug("writing updated YAML to {{.file}}")
		Check(digester.WriteFile())
	}
	Check(digester.WriteAnyLocks())
	return
}

func VerifyDigests(filename string, options ...Option) (err error) {
	defer Handle(func(e error) {
		err = fmt.Errorf("%s: %w", filename, e)
	})
	verifier := NewDigester(options...)
	defer verifier.Cleanup()
	Check(verifier.LoadFile(filename))
	Check(verifier.VerifyDigests())
	return
}

func DigestKube(filename string, options ...Option) (digester *Digester, err error) {
	defer Handle(func(e error) {
		err = fmt.Errorf("%s: %w", filename, e)
	})
	slog := slog.With("file", filename)
	digester = NewDigester(options...)
	defer digester.Cleanup()
	Check(digester.LoadFile(filename))
	slog.Debug("{{.file}}: {{.ndoc}} initial documents", "ndoc", len(digester.Docs))
	Check(digester.ExpandResources())
	slog.Debug("{{.file}}: {{.ndoc}} expanded documents", "ndoc", len(digester.Docs))
	Check(digester.CreateDigests())
	Check(digester.WriteAnyLocks())
	return
}

func WriteCombinedDigests(digests []*Digester, output io.Writer) (err error) {
	defer Catch(&err)
	totalLen := slices.Fold(digests, 0, func(total int, digest *Digester) int { return total + len(digest.Resources) })
	finalNodes := make([]*yaml.Node, 0, totalLen)
	for _, d := range digests {
		docs := slices.Map(d.Resources, func(r types.Resource) *yaml.Node { return Try(r.Save()) })
		finalNodes = append(finalNodes, docs...)
	}
	return yu.StreamDocsOut(output, finalNodes...)
}
