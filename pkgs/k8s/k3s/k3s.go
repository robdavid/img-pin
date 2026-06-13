package k3s

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/k8s/kube"
	"github.com/robdavid/img-pin/pkgs/run"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"gopkg.in/yaml.v3"
)

var (
	ErrInsufficientChartData = errors.New("insufficient data to determine chart details")
)

var K3S_HELM_CHART yu.Signature = yu.Signature{
	Tests: []yu.SigPath{
		{Path: []any{"apiVersion"}, Re: regexp.MustCompile(`^helm\.cattle\.io/v1$`)},
		{Path: []any{"kind"}, Re: regexp.MustCompile(`^HelmChart$`)},
		{Path: []any{"metadata", "name"}},
	},
}

type ChartName struct {
	Chart string
	Base  string
	Dir   string
}

func ParseChartName(chart string) (chartName ChartName) {
	if strings.Contains(chart, "/") {
		if u, err := url.Parse(chart); err == nil {
			if (u.Scheme == "file" || u.Scheme == "") && u.Host == "" {
				chartName.Dir = u.Path
				chartName.Base = path.Base(u.Path)
				chartName.Chart = u.Path
			} else {
				chartName.Chart = chart
				chartName.Base = path.Base(u.Path)
			}
		} else {
			chartName.Dir = chart
			chartName.Base = path.Base(chart)
			chartName.Chart = chart
		}
	} else {
		chartName.Chart = chart
		chartName.Base = chart
	}
	return
}

// HelmChartDeployment is a [Deployment] based on the k3s HelmChart resource
type HelmChartDeployment struct {
	options     types.HelmOptions
	kubeVersion string
	values      *yaml.Node
	resource    *yaml.Node
}

// Load loads spec data from a HelmChart node
func (hc *HelmChartDeployment) Load(doc *yaml.Node) (err error) {
	hc.options = types.HelmOptions{
		ChartName:  yu.Get[string](doc, "spec", "chart").GetOr(""),
		Repository: yu.Get[string](doc, "spec", "repo").GetOr(""),
		Version:    yu.Get[string](doc, "spec", "version").GetOr(""),
		Namespace:  yu.Get[string](doc, "spec", "targetNamespace").GetOr(""),
	}
	hc.kubeVersion, _ = kube.GetClusterVersion()
	slog.Debug("Loaded HelmChart Deployment, chart={{.chart}}, repository={{.repository}}, version={{.version}}, detected cluster={{.kubeVersion}}",
		"chart", hc.options.ChartName, "repository", hc.options.Repository, "version", hc.options.Version,
		"kubeVersion", hc.kubeVersion)

	var valuesNode yaml.Node
	var values string
	values = yu.Get[string](doc, "spec", "valuesContent").GetOr("{}")
	Check(yaml.Unmarshal([]byte(values), &valuesNode))
	hc.values = &valuesNode
	Check(hc.processSet(doc))
	hc.resource = doc
	return nil
}

func (hc *HelmChartDeployment) processSet(doc *yaml.Node) (err error) {
	if node, ok := yu.GetNode(doc, "spec", "set").GetOK(); ok {
		if node.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(node.Content); i += 2 {
				var path yu.Path
				if path, err = yu.ParsePath(node.Content[i].Value); err != nil {
					return
				}
				if err = yu.WriteNode(hc.values, path, node.Content[i+1]); err != nil {
					return fmt.Errorf("%q: %w", path, err)
				}
			}
		}
	}
	return nil
}

// Render returns the resources, as YAML nodes, from the Helm chart template of
// this HelmChart resource, applying the values and parameters defined in the
// resource.
func (hc *HelmChartDeployment) Render() (docs []*yaml.Node, err error) {
	defer Catch(&err)
	if hc.options.ChartName == "" {
		err = fmt.Errorf("%w: chart name not found", ErrInsufficientChartData)
		return
	}
	fh := Try(os.CreateTemp("", "helmchart-values-*.yaml"))
	defer os.Remove(fh.Name())
	defer fh.Close()
	encoder := yaml.NewEncoder(fh)
	Check(encoder.Encode(hc.values))
	Check(encoder.Close())
	fh.Close()
	helmCommand := []string{"helm", "template", "verification", hc.options.ChartName, "--values", fh.Name()}
	if hc.options.Version != "" {
		helmCommand = append(helmCommand, "--version", hc.options.Version)
	}
	if hc.options.Repository != "" {
		helmCommand = append(helmCommand, "--repo", hc.options.Repository)
	}
	if hc.kubeVersion != "" {
		helmCommand = append(helmCommand, "--dry-run=server")
	}
	var output []byte
	output, err = run.Run(helmCommand...)
	return yu.StreamDocsIn(bytes.NewBuffer(output))
}

func (hc *HelmChartDeployment) DefaultValues() (root *yaml.Node, err error) {
	if hc.options.ChartName == "" {
		err = fmt.Errorf("%w: chart name not found", ErrInsufficientChartData)
		return
	}
	helmCommand := []string{"helm", "show", "values", hc.options.ChartName}
	if hc.options.Version != "" {
		helmCommand = append(helmCommand, "--version", hc.options.Version)
	}
	if hc.options.Repository != "" {
		helmCommand = append(helmCommand, "--repo", hc.options.Repository)
	}
	var valueData []byte
	if valueData, err = run.Run(helmCommand...); err != nil {
		return
	}
	var yamlRoot yaml.Node
	err = yaml.Unmarshal(valueData, &yamlRoot)
	root = &yamlRoot
	return
}

func (hc *HelmChartDeployment) CRDs() (docs []*yaml.Node, err error) {
	defer Catch(&err)
	if hc.options.ChartName == "" {
		err = fmt.Errorf("%w: chart name not found", ErrInsufficientChartData)
		return
	}
	chartName := ParseChartName(hc.options.ChartName)
	if chartName.Dir != "" {
		docs = hc.readCrds(chartName.Dir)
		return
	}
	tmpDir := Try(os.MkdirTemp("", "tmp-img-pin-helm-*"))
	defer os.RemoveAll(tmpDir)
	helmCommand := []string{"helm", "fetch", "--untar", "--untardir", tmpDir, chartName.Chart}
	if hc.options.Version != "" {
		helmCommand = append(helmCommand, "--version", hc.options.Version)
	}
	if hc.options.Repository != "" {
		helmCommand = append(helmCommand, "--repo", hc.options.Repository)
	}
	if _, err = run.Run(helmCommand...); err != nil {
		return
	}
	docs = hc.readCrds(filepath.Join(tmpDir, chartName.Base))
	return
}

func (hc *HelmChartDeployment) readCrds(helmDir string) (docs []*yaml.Node) {
	crdsDir := filepath.Join(helmDir, "crds")
	if _, err := os.Stat(crdsDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} else {
			Raise(err)
		}
	}
	entries := Try(os.ReadDir(crdsDir))
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".yaml" || ext == ".yml" {
			fname := filepath.Join(crdsDir, entry.Name())
			func() {
				fh := Try(os.Open(fname))
				defer fh.Close()
				var doc yaml.Node
				decoder := yaml.NewDecoder(fh)
				Check(decoder.Decode(&doc))
				docs = append(docs, &doc)
			}()
		}
	}
	return
}

func (hc *HelmChartDeployment) Values() (*yaml.Node, error) {
	return hc.values, nil
}

func (hc *HelmChartDeployment) WriteStringValue(value string, path ...any) error {
	return yu.Put(hc.values, value, path...)
}

func (hc *HelmChartDeployment) updateValuesText() {
	var valuesContent strings.Builder
	encoder := yaml.NewEncoder(&valuesContent)
	encoder.SetIndent(2)
	Check(encoder.Encode(hc.values))
	encoder.Close()
	Check(yu.Put(hc.resource, valuesContent.String(), "spec", "valuesContent"))
}

func (hc *HelmChartDeployment) Save() (doc *yaml.Node, err error) {
	defer Catch(&err)
	hc.updateValuesText()
	return hc.resource, nil
}

func (hc *HelmChartDeployment) Options() *types.HelmOptions {
	return &hc.options
}

func (hc *HelmChartDeployment) Cleanup() error {
	return nil
}

type HelmChartHandler struct{}

func (HelmChartHandler) Match(doc *yaml.Node, imageDigester digester.ImageDigester, options skipping.ImageOptions) types.Resource {
	if K3S_HELM_CHART.Match(doc) {
		hproc := digester.MakeHelmProcessor(&HelmChartDeployment{}, options, imageDigester)
		return &hproc
	}
	return nil
}

func init() {
	digester.RegisterHandlerFirst("HelmChart", HelmChartHandler{})
}
