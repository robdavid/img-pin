package yaml

import (
	"fmt"
	"regexp"

	"github.com/robdavid/genutil-go/slices"
	"gopkg.in/yaml.v3"
)

type SigPath struct {
	Path []any
	Re   *regexp.Regexp
}

type Signature struct {
	Tests   []SigPath
	Targets []SigPath
}

func (sp *SigPath) Match(node *yaml.Node) []PathAndNode {
	var result []PathAndNode = nil
	matches := MatchManyWithNode(node, MkPath(sp.Path))
	for _, match := range matches {
		if match.Node.Kind != yaml.ScalarNode {
			continue
		}
		if sp.Re == nil {
			result = append(result, match)
		} else {
			var value any
			err := match.Node.Decode(&value)
			if err != nil {
				continue
			}
			valueStr := fmt.Sprintf("%v", value)
			if sp.Re.MatchString(valueStr) {
				result = append(result, match)
			}
		}
	}
	return result
}

func (sig Signature) Match(node *yaml.Node) bool {
	return slices.All(sig.Tests, func(s SigPath) bool { return s.Match(node) != nil })
}

func (sig Signature) Matches(node *yaml.Node) []PathAndNode {
	matches := slices.All(sig.Tests, func(s SigPath) bool { return s.Match(node) != nil })
	if matches {
		targetList := slices.Map(sig.Targets, func(s SigPath) []PathAndNode { return s.Match(node) })
		return slices.Concat(targetList...)
	}
	return nil
}

var K8S_SIGNATURE Signature = Signature{
	Tests: []SigPath{
		{[]any{"apiVersion"}, nil},
		{[]any{"kind"}, nil},
		{[]any{"metadata", "name"}, nil},
	},
}

// K8S_WORKLOAD_SIGNATURE is a signature and target match for any of
// the Kubernetes types that run container images, Deployment, StatefulSet,
// Job, etc.
var K8S_WORKLOAD_SIGNATURE Signature = Signature{
	Tests: []SigPath{
		{[]any{"apiVersion"}, nil},
		{[]any{"kind"}, regexp.MustCompile(`^(Deployment|StatefulSet|Job|CronJob|DaemonSet)$`)},
		{[]any{"metadata", "name"}, nil},
	},
	Targets: []SigPath{
		{[]any{"spec", "template", "spec", "containers", "*", "image"}, nil},
		{[]any{"spec", "template", "spec", "initContainers", "*", "image"}, nil},
		{[]any{"spec", "template", "spec", "ephemeralContainers", "*", "image"}, nil},
		{[]any{"spec", "jobTemplate", "spec", "template", "spec", "containers", "*", "image"}, nil},
		{[]any{"spec", "jobTemplate", "spec", "template", "spec", "initContainers", "*", "image"}, nil},
		{[]any{"spec", "jobTemplate", "spec", "template", "spec", "ephemeralContainers", "*", "image"}, nil},
	},
}

var DOCKER_COMPOSE_SIGNATURE Signature = Signature{
	Targets: []SigPath{
		{[]any{"services", "*", "image"}, nil},
	},
}

func MatchImagePaths(doc *yaml.Node) []PathAndNode {
	for _, sig := range []*Signature{&K8S_WORKLOAD_SIGNATURE, &DOCKER_COMPOSE_SIGNATURE} {
		if targets := sig.Matches(doc); len(targets) > 0 {
			return targets
		}
	}
	return nil
}
