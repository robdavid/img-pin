package digester

import "gopkg.in/yaml.v3"

type ListYaml struct {
	ApiVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Items      []*yaml.Node `yaml:"items"`
}

type DigesterResource struct {
	digester Digester
}

func (dr DigesterResource) Load(doc *yaml.Node) error {
	//dr.digester.Docs =
	panic("not implemented") // TODO: Implement
}

func (s DigesterResource) Save() (*yaml.Node, error) {
	panic("not implemented") // TODO: Implement
}

// Performs any necessary cleanup actions after processing.
func (s DigesterResource) Cleanup() error {
	panic("not implemented") // TODO: Implement
}

func (s DigesterResource) CanDigest() bool {
	panic("not implemented") // TODO: Implement
}

// Digest runs digests over the images in the resource
func (s DigesterResource) Digest() error {
	panic("not implemented") // TODO: Implement
}

// Verify checks all digests are present and correct
func (s DigesterResource) Verify() error {
	panic("not implemented") // TODO: Implement
}

func (s DigesterResource) Expand() ([]*yaml.Node, error) {
	panic("not implemented") // TODO: Implement
}
