package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/clilog"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/dockerfile"
	"github.com/robdavid/img-pin/pkgs/ferrors"
	"github.com/robdavid/img-pin/pkgs/images"
)

var ErrNoAction = errors.New("no action provided")
var ErrNoInputs = errors.New("no non-option arguments provided")

func ConfigureLogging(opts *UserOpts) {
	slog.SetDefault(slog.New(clilog.New(os.Stderr, &clilog.Options{
		Level:           opts.LogLevel,
		AlwaysEmitLevel: opt.Value(slog.LevelDebug),
		NoTime:          true,
	})))
}

func CreateImageOptions(opts *UserOpts) (imageOptions []images.ImageOption, requestCounters images.RequestCounters) {
	if opts.UpdateDigests || opts.Dockerfiles || opts.KubeExpand {
		opts.IncludeTag = true
	}

	if len(opts.Policies) > 0 {
		imageOptions = append(imageOptions, images.ClearPolicies)
		for _, p := range opts.Policies {
			imageOptions = append(imageOptions, images.AddNamedPolicy(p))
		}
	}

	if opts.IncludeTag {
		imageOptions = append(imageOptions, images.IncludeTag)
	}
	if opts.MinAge > 0 {
		imageOptions = append(imageOptions, images.MinimumAge(opts.MinAge))
	}
	if opts.CountRequests {
		requestCounters = make(images.RequestCounters)
		imageOptions = append(imageOptions, images.RequestCount(requestCounters))
	}
	return
}

func CreateDigestOptions(opts *UserOpts) (imageOptions []images.ImageOption, digesterOptions []digester.Option, requestCounters images.RequestCounters) {
	imageOptions, requestCounters = CreateImageOptions(opts)

	digesterOptions = []digester.Option{
		digester.ImageOptions(imageOptions...),
		digester.UseLockfile, // If available
	}
	if opts.SkipPostVerify {
		digesterOptions = append(digesterOptions, digester.SkipPostVerify)
	}
	if opts.TrimMultiline {
		digesterOptions = append(digesterOptions, digester.TrimMultiline)
	}
	if opts.SkipNotFound {
		digesterOptions = append(digesterOptions, digester.SkipNotFound)
	}
	if opts.SkipV1Schema {
		digesterOptions = append(digesterOptions, digester.SkipV1Schema)
	}
	if opts.Lockfile != "" {
		digesterOptions = append(digesterOptions, digester.LockFileName(opts.Lockfile), digester.MustLockFile)
	}
	if opts.Lock {
		digesterOptions = append(digesterOptions, digester.GenerateLocks, digester.MustLockFile, digester.NoWrite)
	}
	return
}

func Run(opts *UserOpts, args []string) (requestCounters images.RequestCounters, err error) {
	ConfigureLogging(opts)
	if !(opts.Yamlfiles || opts.Dockerfiles || opts.Image || opts.KubeExpand) {
		return nil, fmt.Errorf("%w: please supply one of --yaml, --dockerfile, or --image", ErrNoAction)
	}
	if len(args) == 0 {
		return nil, ErrNoInputs
	}
	if opts.Yamlfiles || opts.KubeExpand {
		var digesterOptions []digester.Option
		_, digesterOptions, requestCounters = CreateDigestOptions(opts)
		noLocks := slices.Filter(args, func(name string) bool { return !strings.HasSuffix(name, ".lock.yaml") })
		if opts.KubeExpand {
			err = processKubeResource(noLocks, opts, digesterOptions)
		} else {
			err = processYamlResources(noLocks, opts, digesterOptions)
		}
	} else {
		var imageOptions []images.ImageOption
		imageOptions, requestCounters = CreateImageOptions(opts)
		if opts.Dockerfiles {
			err = processDockerfiles(args, opts, imageOptions)
		} else if opts.Image {
			err = digestImages(args, imageOptions)
		}
	}
	return
}

func Main() {
	userOpts, args := ParseOsFlags()
	requestCounters, err := Run(userOpts, args)
	if err != nil {
		ErrorExit(err)
	}
	DisplayCounters(requestCounters)
}

func DisplayCounters(requestCounters images.RequestCounters) {
	if len(requestCounters) > 0 {
		fmt.Fprintf(os.Stderr, "%s: registry request counters:\n  %s\n",
			os.Args[0],
			strings.ReplaceAll(strings.TrimRight(requestCounters.String(), "\n"), "\n", "\n  "))
	}
}

func ErrorExit(err error) {
	if err == nil {
		return
	}
	if l := len(ferrors.Split(err)); l > 1 {
		fmt.Fprintf(os.Stderr, "%s: %d issues encountered\n", os.Args[0], l)
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
	os.Exit(1)
}

func digestImages(args []string, imageOptions []images.ImageOption) (err error) {

	for _, arg := range args {
		var img *images.Image
		var created time.Time

		if img, err = images.Parse(arg); err != nil {
			return
		}

		if created, err = img.GetDigest(imageOptions...); err != nil {
			return
		}

		fmt.Printf("%s %s\n", img, time.Since(created).Round(time.Second))
	}

	return
}

func processYamlResources(files []string, opts *UserOpts, digesterOptions []digester.Option) (err error) {
	digOpts := slices.Affix(digesterOptions, digester.UpdateMethod(opts.UpdateMethod))
	for _, arg := range files {
		var fileErr error
		if opts.VerifyDigests {
			fileErr = digester.VerifyDigests(arg, digOpts...)
		} else {
			fileErr = digester.CreateDigests(arg, digOpts...)
		}
		if opts.VerifyDigests || opts.BatchMode {
			if fileErr == nil {
				fmt.Printf("%s: OK\n", arg)
			} else {
				fmt.Printf("%s: Failed\n", arg)
			}
		}
		err = ferrors.Join(err, fileErr)

		if err != nil && !opts.VerifyDigests && !opts.BatchMode {
			return // Aborting as soon as an issue is found
		}
	}
	return
}

func processKubeResource(files []string, opts *UserOpts, digesterOptions []digester.Option) (err error) {
	digests := make([]*digester.Digester, len(files))
	for n, file := range files {
		if digests[n], err = digester.DigestKube(file, digesterOptions...); err != nil {
			return
		}
	}
	if !opts.Lock {
		if err = digester.WriteCombinedDigests(digests, os.Stdout); err != nil {
			return
		}
	}
	return
}

func processDockerfiles(files []string, opts *UserOpts, imageOptions []images.ImageOption) (err error) {
	var templatedOptions []dockerfile.Option
	defaultOptions := slices.New(dockerfile.ImageOptions(imageOptions...))
	if opts.VerifyDigests {
		defaultOptions = append(defaultOptions, dockerfile.VerifyOnly)
	} else if opts.UpdateDigests {
		defaultOptions = append(defaultOptions, dockerfile.UpdateDigests)
	}
	if opts.StrictParsing {
		// Explicit strict flag forces strict parsing for all files
		defaultOptions = append(defaultOptions, dockerfile.StrictParsing)
		templatedOptions = defaultOptions
	} else if opts.LaxParsing {
		// Explicit lax flag forces lax parsing for all files
		defaultOptions = append(defaultOptions, dockerfile.LaxParsing)
		templatedOptions = defaultOptions
	} else {
		// By default, use strict parsing for non-templated files and lax parsing for templated files
		templatedOptions = defaultOptions
		defaultOptions = slices.Affix(templatedOptions, dockerfile.StrictParsing)
	}
	templatedExtensions := []string{".m4", ".tpl", ".tmpl"}
	numFailed := 0
	for _, path := range files {
		options := defaultOptions
		if slices.Contains(templatedExtensions, filepath.Ext(path)) {
			options = templatedOptions
		}
		patched, verified, total, patchErr := dockerfile.Patch(path, options...)
		if patchErr != nil {
			fmt.Printf("%s: Error\n", path)
			err = ferrors.Join(err, fmt.Errorf("%s: %w", path, patchErr))
		} else if patched > 0 {
			fmt.Printf("%s: %d image(s) updated of %d total\n", path, patched, total)
			if patched < total {
				numFailed++
			}
		} else if verified > 0 {
			fmt.Printf("%s: %d image(s) verified of %d total\n", path, verified, total)
			if verified < total {
				numFailed++
			}
		} else {
			fmt.Printf("%s: no changes in %d image(s)\n", path, total)
		}
		if err != nil && !opts.VerifyDigests && !opts.BatchMode {
			return
		}
	}
	return
}
