package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/robdavid/img-pin/pkgs/clilog"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/enum"
	"github.com/robdavid/img-pin/pkgs/images"
	"github.com/spf13/pflag"
	flag "github.com/spf13/pflag"
)

type UserOpts struct {
	Yamlfiles      bool
	Dockerfiles    bool
	KubeExpand     bool
	SkipAuth       bool
	SkipPostVerify bool
	SkipNotFound   bool
	SkipV1Schema   bool
	IncludeTag     bool
	VerifyDigests  bool
	UpdateDigests  bool
	LaxParsing     bool
	StrictParsing  bool
	CountRequests  bool
	BatchMode      bool
	TrimMultiline  bool
	Lockfile       string
	Lock           bool
	Help           bool
	Policies       []string
	UpdateMethod   types.UpdateMethod
	Image          bool
	FileDesc       string
	MinAge         time.Duration
	LogLevel       slog.Level
}

func FlagSet(opts *UserOpts, handling pflag.ErrorHandling) *pflag.FlagSet {
	flag := pflag.NewFlagSet("dockerhash", handling)
	opts.LogLevel = slog.LevelInfo
	flag.BoolVarP(&opts.Help, "help", "h", false, "Display this help message")
	flag.BoolVarP(&opts.BatchMode, "batch", "b", false, "Batch mode; don't stop processing files if there's an error")
	flag.BoolVarP(&opts.CountRequests, "counters", "c", false, "Count registry requests and print a summary at the end")
	flag.BoolVarP(&opts.Dockerfiles, "dockerfile", "d", false, "Process arguments as Dockerfile path names, patching in place as required")
	flag.StringVar(&opts.FileDesc, "file-source", "", "A descriptive file source to print in messages")
	flag.BoolVarP(&opts.Image, "image", "i", false, "arguments are image names to determine the hashes of")
	flag.BoolVarP(&opts.KubeExpand, "kube", "K", false, "all documents in all arguments that expand to Kubernetes resources are expanded")
	flag.BoolVarP(&opts.LaxParsing, "lax", "l", false, "Use a more permissive regex-based parser for Dockerfiles (works with templated files)")
	flag.BoolVarP(&opts.Lock, "lock", "k", false, "Generate/update lock file")
	flag.StringVarP(&opts.Lockfile, "lock-file", "f", "", "The name of the lock file to use")
	flag.VarP(clilog.MakeLevelFlag(&opts.LogLevel), "log-level", "L", "Set log level")
	flag.DurationVarP(&opts.MinAge, "min-age", "A", 0, "Minimum build age of the image")
	flag.StringSliceVarP(&opts.Policies, "policy", "p", []string{},
		"Apply one or more image policies, overriding any default policy, one or more of "+strings.Join(images.PolicyNames(), ", "))
	flag.BoolVarP(&opts.IncludeTag, "preserve-tags", "t", false, "Retain any tag in the generated digest")
	flag.BoolVarP(&opts.SkipAuth, "skip-auth", "a", false, "Skip digest of unauthorized private repos")
	flag.BoolVarP(&opts.SkipNotFound, "skip-not-found", "N", false, "Skip images that can't be found in the registry")
	flag.BoolVarP(&opts.SkipPostVerify, "skip-post-verify", "P", false, "Skip post verification that all images have digests")
	flag.BoolVarP(&opts.SkipV1Schema, "skip-v1-schema", "V", false, "Skips images that won't digest because they use the old V1 schema")
	flag.BoolVarP(&opts.StrictParsing, "strict", "s", false, "Use a more strict parser for Dockerfiles (fails on syntax errors)")
	flag.BoolVarP(&opts.TrimMultiline, "trim-multiline", "T", false, "Trims trailing whitespace from the end of literal multi-line strings")
	flag.BoolVarP(&opts.UpdateDigests, "update-digests", "u", false, "Update digests for tags that have moved since the last digest was generated (implies --preserve-tags)")
	flag.VarP(enum.NewEnumValue(&opts.UpdateMethod), "update-method", "m",
		"Method for updating re-written YAML files, one of "+strings.Join(enum.AllStrings[types.UpdateMethod](), ", "))
	flag.BoolVarP(&opts.VerifyDigests, "verify", "v", false, "Verify that all images digests, and that they are within the given age range")
	flag.BoolVarP(&opts.Yamlfiles, "yaml", "y", false, "Treat arguments as YAML files containing K8S resources to parse")
	return flag
}

func ParseFlags(cmdArgs []string, handling pflag.ErrorHandling) (userOpts *UserOpts, args []string, err error) {
	userOpts = &UserOpts{}
	flags := FlagSet(userOpts, handling)
	err = flags.Parse(cmdArgs)
	args = flag.Args()
	return

}

func ParseOsFlags() (userOpts *UserOpts, args []string) {
	userOpts = &UserOpts{}
	flags := FlagSet(userOpts, flag.ExitOnError)
	flags.Parse(os.Args[1:])
	if userOpts.Help {
		fmt.Print(flags.FlagUsages())
		os.Exit(0)
	}
	args = flags.Args()
	return
}
