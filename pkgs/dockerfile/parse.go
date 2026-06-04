package dockerfile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/ferrors"
	"github.com/robdavid/img-pin/pkgs/files"
	"github.com/robdavid/img-pin/pkgs/images"
)

var ErrImageNotFound = errors.New("no image name found in FROM instruction")

type options struct {
	verify       bool
	update       bool
	lax          bool
	strict       bool
	skipV1Schema bool
	skipNotFound bool
	imageOptions []images.ImageOption
}

type Option func(*options)

func (o *options) apply(opts []Option) {
	for _, f := range opts {
		f(o)
	}
}

func ImageOptions(imageOptions ...images.ImageOption) Option {
	return func(o *options) {
		o.imageOptions = append(o.imageOptions, imageOptions...)
	}
}

func VerifyOnly(o *options) {
	o.verify = true
}

func UpdateDigests(o *options) {
	o.update = true
}

func LaxParsing(o *options) {
	o.lax = true
}

func StrictParsing(o *options) {
	o.strict = true
}

func SkipV1Schema(o *options) {
	o.skipV1Schema = true
}

type SkipOptions struct {
	options *options
}

func (so SkipOptions) SkipNoDigest(image *images.Image) bool {
	return so.options.verify && skipping.IsSchemaV1(image.String())
}

func (so SkipOptions) SkipOnPolicy(*images.Image) bool { return true }
func (so SkipOptions) SkipV1Schema(*images.Image) bool { return so.options.skipV1Schema }
func (so SkipOptions) SkipNotFound(*images.Image) bool { return so.options.skipNotFound }

type ImageOccurrence struct {
	Image string
	Line  int
}

func walkAST(node *parser.Node, fn func(*parser.Node)) {
	for n := node; n != nil; n = n.Next {
		fn(n)
		for _, child := range n.Children {
			walkAST(child, fn)
		}
	}
}

func ScanForImages(input io.Reader, strict bool) (occurrences []ImageOccurrence, err error) {
	var ast *parser.Result

	if ast, err = parser.Parse(input); err != nil {
		return nil, err
	}

	if strict {
		_, _, err = instructions.Parse(ast.AST, nil)
		if err != nil {
			return nil, err
		}
	}

	walkAST(ast.AST, func(node *parser.Node) {
		if strings.EqualFold(node.Value, "from") {
			parts := strings.Fields(node.Original)
			for i := 0; i < len(parts); i++ {
				if strings.EqualFold(parts[i], "from") {
					i++
					for i < len(parts) {
						if strings.HasPrefix(parts[i], "-") {
							i++
							continue
						}
						occurrences = append(occurrences, ImageOccurrence{
							Image: parts[i],
							Line:  node.StartLine,
						})
						return
					}
					err = ferrors.Join(err, fmt.Errorf("%w: at line %d", ErrImageNotFound, node.StartLine))
				}
			}
		}
	})

	return
}

// https://docs.docker.com/reference/dockerfile/#from
var fromRegex = regexp.MustCompile(`(?i)^\s*FROM\s+(?:--platform\s*=\s*[^\s]+\s+?)?([^\s]+).*$`)

func RegexScanForImages(input io.Reader) (occurrences []ImageOccurrence, err error) {
	scanner := bufio.NewScanner(input)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if matches := fromRegex.FindStringSubmatch(line); matches != nil {
			occurrences = append(occurrences, ImageOccurrence{
				Image: matches[1],
				Line:  lineNum,
			})
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return occurrences, nil
}

func LockImages(input io.ReadSeeker, output io.Writer, opts ...Option) (imagesUpdated int, imagesVerified int, imagesTotal int, err error) {
	// Read lines from input
	var lines []string
	var occurrences []ImageOccurrence
	var options options
	options.apply(opts)
	skipOptions := SkipOptions{options: &options}

	reader := bufio.NewReader(input)
	for {
		var line string
		if line, err = reader.ReadString('\n'); err != nil && err != io.EOF {
			return
		}
		lines = append(lines, line)
		if err == io.EOF {
			break
		}
	}
	input.Seek(0, io.SeekStart)
	if options.lax {
		occurrences, err = RegexScanForImages(input)
	} else {
		occurrences, err = ScanForImages(input, options.strict)
	}
	if err != nil {
		return
	}
	imagesTotal = len(occurrences)

processLines:
	for n, line := range lines {
		for _, occ := range occurrences {
			if occ.Line == n+1 {
				parts := strings.Fields(line)
				foundFrom := false
				for i, part := range parts {
					if strings.EqualFold(part, "from") {
						foundFrom = true
						continue
					}
					if foundFrom && part == occ.Image {
						var img *images.Image
						var parseErr error
						if img, parseErr = images.Parse(occ.Image); parseErr != nil {
							parseErr = fmt.Errorf("%w: at line %d", parseErr, occ.Line)
							if options.verify {
								err = ferrors.Join(err, parseErr)
								continue processLines
							} else {
								err = parseErr
								return
							}
						}
						if options.verify {
							imagesVerified++
							vererr := img.VerifyDigest(options.imageOptions...)
							if vererr != nil {
								vererr = fmt.Errorf("%w: at line %d", vererr, occ.Line)
								vererr = skipping.SkipError(skipOptions, vererr, img)
								err = ferrors.Join(err, vererr)
							}
						} else {
							if options.update {
								err = img.UpdateDigest(options.imageOptions...)
							} else {
								_, err = img.GetDigest(slices.Affix(options.imageOptions, images.SkipTime)...)
							}
							if err != nil {
								err = fmt.Errorf("%w: at line %d", err, occ.Line)
								err = skipping.SkipError(skipOptions, err, img)
								if err == nil {
									continue processLines
								}
								return
							}
							digested := img.String()
							if parts[i] != digested {
								imagesUpdated++
								parts[i] = digested
								line = strings.Join(parts, " ") + "\n"
								lines[n] = line
							}
						}
						continue processLines
					}
				}
				err = fmt.Errorf("%w: at line %d", ErrImageNotFound, occ.Line)
				return
			}
		}
	}

	if output != nil {
		// Write modified lines to output
		for _, line := range lines {
			if _, werr := output.Write([]byte(line)); werr != nil {
				err = werr
				return
			}
		}
	}
	return
}

func Patch(dockerfile string, options ...Option) (numPatches int, numVerified int, numTotal int, err error) {
	defer Handle(func(e error) {
		err = fmt.Errorf("%w in %s", e, dockerfile)
	})
	output := Try(files.OpenForOverwrite(dockerfile))
	defer output.Close()
	input := Try(os.Open(dockerfile))
	defer input.Close()
	numPatches, numVerified, numTotal = Try3(LockImages(input, output, options...))
	output.AllowOverwrite(true)
	return
}
