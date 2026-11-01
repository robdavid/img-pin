package kustomize

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/k8s"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"go.yaml.in/yaml/v3"
	"sigs.k8s.io/kustomize/api/krusty"
	ktypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

var KUSTOMIZE_FILE yu.Signature = yu.Signature{
	Tests: []yu.SigPath{
		{Path: []any{"apiVersion"}, Re: regexp.MustCompile(`^kustomize\.config\.k8s\.io/v1beta1$`)},
		{Path: []any{"kind"}, Re: regexp.MustCompile(`^Kustomization$`)},
	},
}

// KustomizeDeployment is a [types.Deployment] based on the Kustomization
type KustomizeDeployment struct {
	options       types.HelmOptions
	resource      *yaml.Node
	baseDirectory string
	helmCharts    []*kustomizeHelmChart
	defaultValues *yaml.Node
	values        *yaml.Node
}

type kustomizeHelmChart struct {
	index          int
	chartNode      *yaml.Node
	values         *yaml.Node
	valuesFilePath string
	valuesDoc      *yaml.Node
	inline         bool
	newInline      bool
	dirty          bool
	options        types.HelmOptions
}

// Load loads the Kustomization document and its containing directory.
func (k *KustomizeDeployment) Load(doc *yaml.Node, baseDirectory string) (err error) {
	defer Catch(&err)
	k.resource = doc
	k.baseDirectory = baseDirectory

	var kt ktypes.Kustomization
	Check(k.resource.Decode(&kt))

	if len(kt.HelmCharts) > 0 {
		Check(k.loadHelmCharts(&kt))
		// support a single chart
		k.values = k.helmCharts[0].values
		k.defaultValues = nil
	}
	return nil
}

// Render returns the resources, as YAML nodes, from the Kustomize file.
func (k *KustomizeDeployment) Render() (docs []*yaml.Node, err error) {
	defer Catch(&err)
	opts := krusty.MakeDefaultOptions()
	opts.PluginConfig.HelmConfig.Enabled = true
	opts.PluginConfig.HelmConfig.Command = k8s.HelmBinary.Value()
	kustomizer := krusty.MakeKustomizer(opts)
	resMap := Try(kustomizer.Run(filesys.MakeFsOnDisk(), k.baseDirectory))
	output := Try(resMap.AsYaml())
	return yu.StreamDocsIn(bytes.NewBuffer(output))
}

func (k *KustomizeDeployment) Save() (doc *yaml.Node, err error) {
	defer Catch(&err)
	for _, chart := range k.helmCharts {
		if !chart.dirty {
			continue
		}
		switch {
		case chart.inline:
			chart.attachInlineValues()
		case chart.valuesFilePath != "":
			Check(yu.WriteDocsOut(chart.valuesFilePath, chart.valuesDoc))
		}
	}
	return k.resource, nil
}

func (chart *kustomizeHelmChart) attachInlineValues() {
	if !chart.newInline || chart.chartNode == nil || chart.values == nil {
		return
	}
	node := topNode(chart.chartNode)
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "valuesInline" {
			node.Content[i+1] = chart.values
			chart.newInline = false
			return
		}
	}
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "valuesInline"},
		chart.values,
	)
	chart.newInline = false
}

func (k *KustomizeDeployment) Options() *types.HelmOptions {
	return &k.options
}

func (k *KustomizeDeployment) CRDs() (docs []*yaml.Node, err error) { return }
func (k *KustomizeDeployment) Cleanup() error                       { return nil }

func (k *KustomizeDeployment) DefaultValues() (root *yaml.Node, err error) {
	if len(k.helmCharts) == 0 {
		return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{newMappingNode()}}, nil
	}
	return k8s.LoadDefaultValues(k.helmCharts[0].options)
}

func (k *KustomizeDeployment) Values() (*yaml.Node, error) {
	return k.values, nil
}

func (k *KustomizeDeployment) WriteStringValue(value string, path ...any) (err error) {
	defer Catch(&err)
	Check(yu.WriteValue(k.values, yu.MkPath(path), value))
	for _, chart := range k.helmCharts {
		if chart.values == k.values {
			chart.dirty = true
			break
		}
	}
	return nil
}

func (k *KustomizeDeployment) loadHelmCharts(kt *ktypes.Kustomization) (err error) {
	defer Catch(&err)
	chartHome := ""
	if kt.HelmGlobals != nil && kt.HelmGlobals.ChartHome != "" {
		chartHome = kt.HelmGlobals.ChartHome
	}
	chartsSeq := mappingChild(topNode(k.resource), "helmCharts")
	for i := range kt.HelmCharts {
		hc := &kt.HelmCharts[i]
		chart := &kustomizeHelmChart{index: i}
		if chartsSeq != nil && chartsSeq.Kind == yaml.SequenceNode && i < len(chartsSeq.Content) {
			chart.chartNode = chartsSeq.Content[i]
		}

		chart.options = types.HelmOptions{
			ChartName:     hc.Name,
			Version:       hc.Version,
			Repository:    hc.Repo,
			Namespace:     hc.Namespace,
			BaseDirectory: k.baseDirectory,
		}
		if chartHome != "" {
			chart.options.ChartName = filepath.Join(chartHome, hc.Name)
		}

		var valuesInline *yaml.Node
		if chart.chartNode != nil {
			valuesInline = mappingChild(chart.chartNode, "valuesInline")
		}
		switch {
		case valuesInline != nil:
			chart.values = valuesInline
			if chart.values.Kind != yaml.MappingNode {
				chart.values = newMappingNode()
				chart.newInline = true
			}
			chart.inline = true
		case hc.ValuesFile != "":
			path := filepath.Join(k.baseDirectory, hc.ValuesFile)
			chart.values, chart.valuesDoc, err = readValuesFile(path)
			if err != nil {
				return err
			}
			chart.valuesFilePath = path
		case len(hc.AdditionalValuesFiles) > 0:
			path := filepath.Join(k.baseDirectory, hc.AdditionalValuesFiles[len(hc.AdditionalValuesFiles)-1])
			chart.values, chart.valuesDoc, err = readValuesFile(path)
			if err != nil {
				return err
			}
			chart.valuesFilePath = path
		default:
			chart.values = newMappingNode()
			chart.inline = true
			chart.newInline = true
		}
		if chart.values == nil {
			chart.values = newMappingNode()
		}
		k.helmCharts = append(k.helmCharts, chart)
	}
	return nil
}

func readValuesFile(path string) (top *yaml.Node, doc *yaml.Node, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	docs, err := yu.StreamDocsIn(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	if len(docs) == 0 {
		return newMappingNode(), &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{newMappingNode()}}, nil
	}
	doc = docs[0]
	top = topNode(doc)
	if top == nil || top.Kind != yaml.MappingNode {
		top = newMappingNode()
		return top, &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{top}}, nil
	}
	return top, doc, nil
}

func newValuesRoot() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
		}},
	}
}

func newMappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

func topNode(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return node.Content[0]
	}
	return node
}

func mappingChild(node *yaml.Node, key string) *yaml.Node {
	node = topNode(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

type KustomizeHandler struct{}

func (KustomizeHandler) Match(doc *yaml.Node, imageDigester digester.ImageDigester, options skipping.ImageOptions) types.Resource {
	if KUSTOMIZE_FILE.Match(doc) {
		hproc := digester.MakeHelmProcessor(&KustomizeDeployment{}, options, imageDigester)
		return &hproc
	}
	return nil
}

func init() {
	digester.RegisterHandlerFirst("Kustomize", KustomizeHandler{})
}
