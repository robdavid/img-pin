package images

import (
	"fmt"
	"time"

	"github.com/robdavid/genutil-go/maps"
	"github.com/robdavid/genutil-go/slices"
)

type ImageParts struct {
	Registry string
	Group    string
	Name     string
	Tag      string
}

func MakeImageParts(img *Image) ImageParts {
	group, name := img.GroupAndName()
	return ImageParts{Registry: img.Registry, Group: group, Name: name, Tag: img.Tag}
}

func (ip *ImageParts) UpdateImage(img *Image) {
	if ip.Registry != "" {
		img.Registry = ip.Registry
	}
	if ip.Group != "" && ip.Name != "" {
		img.Repository = ip.Group + "/" + ip.Name
	} else if ip.Name != "" {
		img.Repository = ip.Name
	} else if ip.Group != "" {
		_, name := img.GroupAndName()
		img.Repository = ip.Group + "/" + name
	}
	if ip.Tag != "" {
		img.Tag = ip.Tag
	}
}

func (ip *ImageParts) Match(img *Image) bool {
	group, name := img.GroupAndName()
	if ip.Registry != "" && ip.Registry != img.Registry {
		return false
	}
	if ip.Group != "" && ip.Group != group {
		return false
	}
	if ip.Name != "" && ip.Name != name {
		return false
	}
	if ip.Tag != "" && ip.Tag != img.Tag {
		return false
	}
	return true
}

func (ip *ImageParts) Truncate() bool {
	if ip.Tag != "" {
		ip.Tag = ""
		return true
	} else if ip.Name != "" {
		ip.Name = ""
		return true
	} else if ip.Group != "" {
		ip.Group = ""
		return true
	}
	return false
}

type MatchList struct {
	criteria []ImageParts
	cache    map[ImageParts]bool
}

func NewMatchList(parts []ImageParts) *MatchList {
	return &MatchList{
		criteria: parts,
		cache:    make(map[ImageParts]bool),
	}
}

func (ml MatchList) Match(img *Image) bool {
	parts := MakeImageParts(img)
	if ml.cache[parts] {
		return true
	}
	for _, ip := range ml.criteria {
		if ip.Match(img) {
			ml.cache[parts] = true
			return true
		}
	}
	return false
}

func MapperPolicy(mapping map[ImageParts]ImageParts, fallback bool) Policy {
	return func(pol *PolicyContext) error {
		img := pol.Image
		key := MakeImageParts(img)
		for {
			if replacement, ok := mapping[key]; ok {
				replacement.UpdateImage(img)
				if fallback {
					pol.FallbackImage = pol.Image.Clone()
				}
				break
			}
			if !key.Truncate() {
				break
			}
		}
		return nil
	}
}

// DockerToAWS is a helper function that generates a mapping for remapping
// common Docker Hub images to the AWS Public ECR. It assumes the same name,
// and only changes the group and registry. The tag is left unchanged.
func DockerToAWS(imageNames ...string) map[ImageParts]ImageParts {
	mapping := make(map[ImageParts]ImageParts)
	for _, name := range imageNames {
		src := ImageParts{Registry: "docker.io", Group: "library", Name: name, Tag: ""}
		dst := ImageParts{Registry: "public.ecr.aws", Group: "docker/library", Name: name, Tag: ""}
		mapping[src] = dst
	}
	return mapping
}

func DefaultAgeByNamePolicy(table map[string]time.Duration) Policy {
	return func(pol *PolicyContext) error {
		img := pol.Image
		_, name := img.GroupAndName()
		if age, ok := table[name]; ok {
			*pol.Options = append(slices.New(MinimumAge(age)), *pol.Options...)
		}
		return nil
	}
}

// NoDockerHubPolicy is a policy that disallows images from Docker Hub.
// The fallback image is not checked, so any policy that sets a fallback image will bypass this policy.
func NoDockerHubPolicy(pol *PolicyContext) error {
	img := pol.Image
	if img.Registry == "index.docker.io" || img.Registry == "docker.io" {
		return fmt.Errorf("Docker hub images are not allowed")
	}
	return nil
}

func NoDockerHubExceptPolicy(whitelist ...ImageParts) Policy {
	ml := NewMatchList(whitelist)
	return func(pol *PolicyContext) error {
		img := pol.Image
		key := MakeImageParts(img)
		if key.Registry == "index.docker.io" || key.Registry == "docker.io" {
			if ml.Match(img) {
				return nil
			}
			return fmt.Errorf("Docker hub images are not allowed")
		}
		return nil
	}
}

// SkipPolicy is a policy that allows images to be skipped from verification.
func SkipPolicy(skipList ...ImageParts) Policy {
	ml := NewMatchList(skipList)
	return func(pol *PolicyContext) error {
		if ml.Match(pol.Image) {
			return ErrSkipImage
		}
		return nil
	}
}

// RejectLatest policy is a policy that disallows images that use the latest tag,
// barring a whitelist of exceptions.
func RejectLatestExceptPolicy(whitelist ...ImageParts) Policy {
	ml := NewMatchList(whitelist)
	return func(pol *PolicyContext) error {
		img := pol.Image
		if ml.Match(img) {
			return nil
		}
		if img.Tag == "latest" {
			return fmt.Errorf("latest tag is not allowed")
		}
		return nil
	}
}

// DefaultMinAgePolicy sets a default minimum age for images. It adds to the
// start of the image options so that it can be overridden by other policies or
// user settings that add a minimum age.
func DefaultMinAgePolicy(minAge time.Duration) Policy {
	return func(pol *PolicyContext) error {
		*pol.Options = append(slices.New(MinimumAge(minAge)), *pol.Options...)
		return nil
	}
}

type PolicyEntry struct {
	Name   string
	Policy Policy
}

var policyRegistry = make(map[string]Policy)

func RegisterPolicy(name string, policy Policy) {
	policyRegistry[name] = policy
}

var defaultPolicy Policy = nil

func SetDefaultPolicy(policy Policy) {
	defaultPolicy = policy
}

func PolicyNames() []string {
	keys := maps.Keys(policyRegistry)
	slices.Sort(keys)
	return keys
}
