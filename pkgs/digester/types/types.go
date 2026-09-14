package types

import (
	"go.yaml.in/yaml/v3"
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

	// Load populates the resource with raw YAML data provided in the doc parameter.
	Load(doc *yaml.Node) error

	// Save returns YAML data that represents the current state of the resource.
	Save() (*yaml.Node, error)

	// Performs any necessary cleanup actions after processing.
	Cleanup() error
}

// Resource is the basic common interface for all YAML resources.
type Resource interface {
	BaseResource

	// CanDigest returns true if the [Resource.Digest] and [Resource.Verify]
	// methods are supported.
	CanDigest() bool

	// Digest runs digests over the images in the resource
	Digest() error

	// Verify checks all digests are present and correct
	Verify() error

	// Expand returns a list of YAML nodes that represent all the YAML documents
	// the resource can be expanded into, if it can be so expanded. For example
	// a Helm will be expanded into YAML documents via the helm template
	// command. A resource that cannot be expanded simply returns itself as a
	// YAML node.
	Expand() ([]*yaml.Node, error)

	// CRDs returns any custom resource definitions associated with this resource, that
	// are not explicit in the resource itself. This typically applies to Helm charts.
	// Resources without such definitions will return an empty list (or nil).
	CRDs() ([]*yaml.Node, error)
}

// HelmOptions holds basic deployment options
type HelmOptions struct {
	ChartName    string
	InstanceName string
	Repository   string
	Version      string
	Namespace    string
}

// Deployment represents a Helm deployment resource (such as a HelmChart).
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
