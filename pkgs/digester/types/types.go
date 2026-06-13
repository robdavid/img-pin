package types

import (
	"gopkg.in/yaml.v3"
)

// UpdateMethod describe methods for modifying a YAML file in place,
// one of [UpdateOverwrite], [UpdatePatch] or [UpdateSync]. The various
// methods are attempting, with varying degrees of success, to preserve
// blank line whitespace.
//
//go:generate stringer -type UpdateMethod -linecomment
type UpdateMethod int

const (
	// UpdateOverwrite indicates update output YAML with simple overwrite.
	// This will lose some blank line cosmetic whitespace
	UpdateOverwrite UpdateMethod = iota // overwrite
	// UpdatePatch uses a diff then patch technique to try to retain newline
	// whitespace. This is experimental.
	UpdatePatch // patch
	// UpdateSync attempts to synchronize output file with input file in terms
	// on blank lines. This does not always work.
	UpdateSync //sync
)

type BaseResource interface {
	Load(doc *yaml.Node) error
	Save() (*yaml.Node, error)
	// Performs any necessary cleanup actions after processing.
	Cleanup() error
}

// Resource is the basic common interface for all YAML resources.
type Resource interface {
	BaseResource

	CanDigest() bool
	// Digest runs digests over the images in the resource
	Digest() error

	// Verify checks all digests are present and correct
	Verify() error

	Expand() ([]*yaml.Node, error)

	CRDs() ([]*yaml.Node, error)
}

// HelmOptions holds basic deployment options
type HelmOptions struct {
	ChartName  string
	Repository string
	Version    string
	Namespace  string
}

type Deployment interface {
	BaseResource

	// Render generates the YAML resources that describes the deployment
	Render() ([]*yaml.Node, error)

	// DefaultValues returns the YAML resources that describe the Helm
	// chart's default values.
	DefaultValues() (*yaml.Node, error)

	// Values returns the parsed YAML nodes of the Helm values used in the
	// deployment.
	Values() (*yaml.Node, error)

	// WriteStringValue writes a string value into the deployment's values, at the
	// given path
	WriteStringValue(value string, path ...any) error

	// Options returns the options required for the deployment
	Options() *HelmOptions

	// CRDs returns crd documents as YAML nodes.
	CRDs() ([]*yaml.Node, error)
}
