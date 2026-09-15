package workload

import (
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"go.yaml.in/yaml/v3"
)

// K8SResource is an implementation of [digester.Resource] which handles image pinning in well known
// workload resource types, such as StatefulSet, Deployment, Job etc.
type K8SResource struct {
	resource *yaml.Node
	targets  []yu.PathAndNode
	options  skipping.ImageOptions
	digester digester.YAMLDigester
}

func (k8s *K8SResource) Load(doc *yaml.Node) error {
	k8s.resource = doc
	return nil
}

func (k8s *K8SResource) Save() (*yaml.Node, error) {
	return k8s.resource, nil
}

func (k8s *K8SResource) Digest() error {
	return k8s.digester.DigestTargets(k8s.targets, k8s.options)
}

func (k8s *K8SResource) Verify() error {
	return k8s.digester.VerifyTargets(k8s.targets, k8s.options)
}

func (k8s *K8SResource) Cleanup() error                { return nil }
func (k8s *K8SResource) CanDigest() bool               { return true }
func (k8s *K8SResource) Expand() ([]*yaml.Node, error) { return []*yaml.Node{k8s.resource}, nil }
func (k8s *K8SResource) CRDs() ([]*yaml.Node, error)   { return nil, nil }

type K8SWorkloadHandler struct{}

// Match matches the given resource YAML document with a well known workload
// resource type, and determines path location of image names that require
// patching when pinning. If there is a match, a *[K8SResource] implementation
// of [types.Resource] is returned. Otherwise nil is returned.
func (K8SWorkloadHandler) Match(doc *yaml.Node, imageDigester digester.ImageDigester, options skipping.ImageOptions) types.Resource {
	if targets := yu.K8S_WORKLOAD_SIGNATURE.Matches(doc); targets != nil {
		yamlDigester := digester.MakeYamlDigester(imageDigester)
		return &K8SResource{targets: targets, options: options, digester: yamlDigester, resource: doc}
	}
	return nil
}

func init() {
	digester.RegisterHandlerFirst("KubernetesWorkload", K8SWorkloadHandler{})
}
