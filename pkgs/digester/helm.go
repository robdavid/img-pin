package digester

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/ferrors"
	"github.com/robdavid/img-pin/pkgs/images"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"go.yaml.in/yaml/v4"
)

// ImageDetails holds image fields detected from chart and deployment values,
// including where they are located in the YAML AST.
type ImageDetails struct {
	Registry   yu.PathValue[string]
	Repository yu.PathValue[string]
	Tag        yu.PathValue[string]
}

func ImageDetailsFromImage(img *images.Image) (d ImageDetails) {
	d.Set(img)
	return d
}

func (d *ImageDetails) WriteInto(dep types.Deployment) error {
	for _, pathValue := range []yu.PathValue[string]{d.Registry, d.Repository, d.Tag} {
		if pathValue.Path != nil {
			if err := dep.WriteStringValue(pathValue.Value, pathValue.Path.Objects()...); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *ImageDetails) String() string {
	if d.Tag.Value != "" {
		if d.Registry.Value != "" {
			return joinStr(joinStr(d.Registry.Value, "/", d.Repository.Value), ":", d.Tag.Value)
		}
		return joinStr(d.Repository.Value, ":", d.Tag.Value)
	} else {
		if d.Registry.Value != "" {
			return joinStr(d.Registry.Value, "/", d.Repository.Value)
		} else {
			return d.Repository.Value
		}
	}
}

func joinStr(prefix, delim, suffix string) string {
	if strings.HasSuffix(prefix, delim) || strings.HasPrefix(suffix, delim) {
		return prefix + suffix
	} else {
		return prefix + delim + suffix
	}
}

func (d *ImageDetails) Parent() yu.Path {
	for _, pv := range []yu.PathValue[string]{d.Registry, d.Repository, d.Tag} {
		if pv.Path != nil {
			return pv.Path[:len(pv.Path)-1]
		}
	}
	panic("image details have no parent path!")
}

func (d *ImageDetails) Set(img *images.Image) {
	tag := img.Tag
	if img.Digest != "" {
		tag += "@" + img.Digest
	}
	if d.Registry.IsEmpty() {
		if d.Tag.IsEmpty() {
			d.Repository.Value = img.String()
		} else {
			d.Repository.Value = img.Registry + "/" + img.Repository
			d.Tag.Value = tag
		}
	} else {
		d.Registry.Value = img.Registry
		if d.Tag.IsEmpty() {
			d.Repository.Value = img.Repository + ":" + tag
		} else {
			d.Repository.Value = img.Repository
			d.Tag.Value = tag
		}
	}
}

func (d *ImageDetails) Where() string {
	for _, pv := range []*yu.PathValue[string]{&d.Registry, &d.Repository, &d.Tag} {
		if len(pv.Path) > 1 {
			parent := pv.Path[:len(pv.Path)-1]
			return fmt.Sprintf("at path %s", parent)
		}
	}
	return ""
}

func groupToDetails(root *yaml.Node, group yu.EntryGroup) (details ImageDetails, match bool) {
	posns := make(map[string]int) // Positions of matches in group

	for posn, entry := range group.Entries {
		name := entry.Path[len(entry.Path)-1].String()
		posns[name] = posn
	}

	fetchAt := func(n int) yu.PathValue[string] {
		var value string
		entry := group.Entries[n]
		if err := entry.Node.Decode(&value); err != nil {
			value = "" // Not expected to happen
		}
		return yu.PathValue[string]{Path: entry.Path, Value: value}
	}

	combo := func(keys ...string) []int {
		result := make([]int, len(posns))
		for n, key := range keys {
			if value, ok := posns[key]; ok {
				result[n] = value
			} else {
				return nil
			}
		}
		return result
	}

	match = true
	if entries := combo("registry", "repository", "tag"); entries != nil {
		details.Registry = fetchAt(entries[0])
		details.Repository = fetchAt(entries[1])
		details.Tag = fetchAt(entries[2])
	} else if entries := combo("repository", "name", "tag"); entries != nil {
		details.Registry = fetchAt(entries[0])
		details.Repository = fetchAt(entries[1])
		details.Tag = fetchAt(entries[2])
	} else if entries := combo("repository", "tag"); entries != nil {
		details.Repository = fetchAt(entries[0])
		details.Tag = fetchAt(entries[1])
	} else if entries := combo("imageName", "imageTag"); entries != nil {
		details.Repository = fetchAt(entries[0])
		details.Tag = fetchAt(entries[1])
	} else if entries := combo("image", "imageTag"); entries != nil {
		details.Repository = fetchAt(entries[0])
		details.Tag = fetchAt(entries[1])
	} else if entries := combo("image"); entries != nil {
		details.Repository = fetchAt(entries[0])
	} else {
		match = false
	}
	return
}

func searchForImageDetails(root *yaml.Node) (details []ImageDetails) {
	matches1 := yu.MatchManyWithNode(root, yu.PathOf("**", "image", `/^(registry|repository|tag|name)$/`))
	matches2 := yu.MatchManyWithNode(root, yu.PathOf("**", `/^(imageName|imageTag|image)$/`))
	// matches3 := yu.MatchManyWithNode(root, yu.PathOf("**", "image")) // Some charts allow direct image insertion
	matches := slices.Concat(matches1, matches2)
	groups := yu.GroupByPrefix(matches)
	details = make([]ImageDetails, 0, len(groups))
	for _, group := range groups {
		if d, ok := groupToDetails(root, group); ok {
			if _, err := images.Parse(d.String()); err == nil {
				details = append(details, d)
			} else {
				fmt.Fprintf(os.Stderr, "%s: cannot parse %q as an image", os.Args[0], d.String())
			}
		}
	}
	return
}

type HelmProcessor struct {
	Deployment types.Deployment
	// Images holds the default image values found in the
	// helm chart's values file
	Images []ImageDetails
	// DeploymentImages are the images with any overrides
	// applied by the
	DeploymentImages []ImageDetails
	// DigestedImages are the overridden images with digests applied
	DigestedImages []ImageDetails
	options        skipping.ImageOptions
	digester       ImageDigester
}

func MakeHelmProcessor(deployment types.Deployment, options skipping.ImageOptions, digester ImageDigester) HelmProcessor {
	return HelmProcessor{Deployment: deployment, options: options, digester: digester}
}

func (*HelmProcessor) CanDigest() bool {
	return true
}

func (hp *HelmProcessor) LoadDefaultImages() (err error) {
	defer Catch(&err)
	root := Try(hp.Deployment.DefaultValues())
	hp.Images = searchForImageDetails(root)
	return
}

// AddValuesSpecificImages finds any images in the chart values that don't
// appear in the base chart. They are added to [HelmProcessor.Images].
func (hp *HelmProcessor) AddValuesSpecificImages(values *yaml.Node) {
	imageIndex := make(map[string]*ImageDetails)
	for i := range hp.Images {
		id := &hp.Images[i]
		imageIndex[id.Parent().String()] = id
	}
	newImages := searchForImageDetails(values)
	for _, id := range newImages {
		if imageIndex[id.Parent().String()] == nil {
			hp.Images = append(hp.Images, id)
		}
	}
}

func (hp *HelmProcessor) ResolveOverrides() (err error) {
	defer Catch(&err)
	values := Try(hp.Deployment.Values())
	if hp.Images == nil {
		Check(hp.LoadDefaultImages())
		hp.AddValuesSpecificImages(values)
	}
	hp.DeploymentImages = make([]ImageDetails, len(hp.Images))
	for i := range hp.Images {
		hp.DeploymentImages[i] = hp.Images[i]
		dep := &hp.DeploymentImages[i]
		for _, v := range []*yu.PathValue[string]{&dep.Registry, &dep.Repository, &dep.Tag} {
			yu.GetPath[string](values, v.Path).Then(func(field string) { v.Value = field })
		}
	}
	return nil
}

func (hp *HelmProcessor) Digest() (err error) {
	if hp.DeploymentImages == nil {
		if err = hp.ResolveOverrides(); err != nil {
			return
		}
	}
	options := hp.options.ImageOptions()
	slog.Debug("digesting images")
	hp.DigestedImages = make([]ImageDetails, len(hp.DeploymentImages))
	for i := range hp.DeploymentImages {
		var img *images.Image
		deployImg := &hp.DeploymentImages[i]
		if img, err = images.Parse(hp.DeploymentImages[i].String()); err != nil {
			return fmt.Errorf("%w: %s %s", err, deployImg, deployImg.Where())
		}
		imgOpts := slices.Affix(options, images.SkipTime, images.IncludeTag)
		if _, err = hp.digester.GetDigest(img, imgOpts...); err != nil {
			err = skipping.SkipError(hp.options, err, img)
			if err == nil {
				hp.DigestedImages[i] = *deployImg
				continue
			}
			return fmt.Errorf("%w: %s %s", err, deployImg, deployImg.Where())
		}
		slog.Debug("patching {{.before}} => {{.after}}", "before", deployImg, "after", img)
		hp.DigestedImages[i] = *deployImg
		hp.DigestedImages[i].Set(img)
	}
	for _, id := range hp.DigestedImages {
		if err := id.WriteInto(hp.Deployment); err != nil {
			return err
		}
	}
	return nil
}

func (hp *HelmProcessor) Verify() (err error) {
	defer Catch(&err)
	yd := MakeYamlDigester(hp.digester)
	resourceYaml := Try(hp.Deployment.Render())
	inbuf := bytes.NewBuffer(resourceYaml)
	docs := Try(yu.StreamDocsIn(inbuf))
	for docn, doc := range docs {
		targets := yu.MatchImagePaths(doc)
		if verErr := yd.VerifyTargets(targets, hp.options); verErr != nil {
			err = ferrors.Join(err, fmt.Errorf("[doc %d] %w", docn, verErr))
		}
	}
	return
}

func (hp *HelmProcessor) Load(doc *yaml.Node) error {
	return hp.Deployment.Load(doc)
}

func (hp *HelmProcessor) Save() (*yaml.Node, error) {
	return hp.Deployment.Save()
}

func (hp *HelmProcessor) Cleanup() error {
	return hp.Deployment.Cleanup()
}

func (hp *HelmProcessor) Expand() (docs []*yaml.Node, err error) {
	var docContent []byte
	if docContent, err = hp.Deployment.Render(); err != nil {
		return
	}
	return yu.StreamDocsIn(bytes.NewBuffer(docContent))
}
