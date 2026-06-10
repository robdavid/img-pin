package digester

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"gopkg.in/yaml.v3"
)

var ErrYamlParse = errors.New("error parsing YAML")

var K8S_LIST yu.Signature = yu.Signature{
	Tests: []yu.SigPath{
		{Path: []any{"apiVersion"}, Re: regexp.MustCompile(`^v1$`)},
		{Path: []any{"kind"}, Re: regexp.MustCompile(`^List$`)},
	},
}

type ListYaml struct {
	ApiVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Items      []*yaml.Node `yaml:"items"`
}

type DigesterResource struct {
	digester *Digester
}

func (dr DigesterResource) Load(doc *yaml.Node) error {
	const items = "items"
	optNode := yu.GetNode(doc, items)
	if optNode.IsEmpty() {
		return fmt.Errorf("%w: expected %q field", ErrYamlParse, items)
	}
	dr.digester.Docs = optNode.Ref().Content
	return dr.digester.ReadDocs()
}

func (dr DigesterResource) Save() (*yaml.Node, error) {
	yamlMap := ListYaml{
		ApiVersion: "v1",
		Kind:       "List",
		Items:      dr.digester.DigestedDocs,
	}
	var yamlNode yaml.Node
	if err := yamlNode.Encode(yamlMap); err != nil {
		return nil, err
	}
	return &yamlNode, nil
}

// Performs any necessary cleanup actions after processing.
func (dr DigesterResource) Cleanup() error {
	return dr.digester.Cleanup()
}

func (dr DigesterResource) CanDigest() bool {
	return slices.Any(dr.digester.Resources,
		func(r types.Resource) bool { return r.CanDigest() })
}

// Digest runs digests over the images in the resource
func (dr DigesterResource) Digest() error {
	return dr.digester.CreateDigests()
}

// Verify checks all digests are present and correct
func (dr DigesterResource) Verify() error {
	return dr.digester.VerifyDigests()
}

func (dr DigesterResource) Expand() ([]*yaml.Node, error) {
	if node, err := dr.Save(); err != nil {
		return nil, err
	} else {
		return []*yaml.Node{node}, nil
	}
}
