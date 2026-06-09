package k3s

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/k8s/kube"
	"github.com/robdavid/img-pin/pkgs/run"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"go.yaml.in/yaml/v4"
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

// HelmChartDeployment is a [Deployment] based on the k3s HelmChart resource
type HelmChartDeployment struct {
	options     types.HelmOptions
	kubeVersion string
	values      *yaml.Node
	resource    *yaml.Node
}

func (hc *HelmChartDeployment) Load(doc *yaml.Node) (err error) {
	hc.options = types.HelmOptions{
		ChartName:  yu.Get[string](doc, "spec", "chart").GetOr(""),
		Repository: yu.Get[string](doc, "spec", "repo").GetOr(""),
		Version:    yu.Get[string](doc, "spec", "version").GetOr(""),
	}
	hc.kubeVersion, _ = kube.GetClusterVersion()
	slog.Debug("Loaded HelmChart Deployment, chart={{.chart}}, repository={{.repository}}, version={{.version}}, detected cluster={{.kubeVersion}}",
		"chart", hc.options.ChartName, "repository", hc.options.Repository, "version", hc.options.Version,
		"kubeVersion", hc.kubeVersion)

	var valuesNode yaml.Node
	var values string
	values = yu.Get[string](doc, "spec", "valuesContent").GetOr("")
	Check(yaml.Unmarshal([]byte(values), &valuesNode))
	hc.values = &valuesNode
	hc.resource = doc
	return nil
}

func (hc *HelmChartDeployment) Render() (output []byte, err error) {
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
	output, err = run.Run(helmCommand...)
	return
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
