package k8s

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/robdavid/img-pin/pkgs/digester/types"
	k8stypes "github.com/robdavid/img-pin/pkgs/k8s/types"
	"github.com/robdavid/img-pin/pkgs/run"
	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/semver"
)

const (
	DefaultHelmBinary = "helm"
)

var (
	ErrInsufficientChartData = errors.New("insufficient data to determine chart details")

	HelmBinary = k8stypes.HelmOpt{EnvOpt: k8stypes.EnvOpt{Env: "IMG_PIN_HELM", DefaultValue: "helm"}}
)

func HelmVersionAtLeast(version string) bool {
	return semver.Compare(HelmBinary.Version().GetOr("v0"), version) >= 0
}



type ChartName struct {
	Chart string
	Base  string
	Dir   string
}

func ParseChartName(chart string) (chartName ChartName) {
	if numSlash := strings.Count(chart, "/"); numSlash > 0 {
		if u, err := url.Parse(chart); err == nil && numSlash > 1 {
			if (u.Scheme == "file" || u.Scheme == "") && u.Host == "" {
				chartName.Dir = u.Path
				chartName.Base = path.Base(u.Path)
				chartName.Chart = u.Path
			} else {
				chartName.Chart = chart
				chartName.Base = path.Base(u.Path)
			}
		} else if numSlash > 1 ||
			strings.HasPrefix(chart, "/") ||
			strings.HasPrefix(chart, "./") ||
			strings.HasPrefix(chart, "../") {
			chartName.Dir = chart
			chartName.Base = path.Base(chart)
			chartName.Chart = chart
		} else {
			chartName.Base = path.Base(chart)
			chartName.Chart = chart
		}
	} else {
		chartName.Chart = chart
		chartName.Base = chart
	}
	return
}


func LoadDefaultValues(options types.HelmOptions) (root *yaml.Node, err error) {
	if options.ChartName == "" {
		err = fmt.Errorf("%w: chart name not found", ErrInsufficientChartData)
		return
	}
	helmCommand := []string{HelmBinary.Value(), "show", "values", options.ChartName}
	if options.Version != "" {
		helmCommand = append(helmCommand, "--version", options.Version)
	}
	if options.Repository != "" {
		helmCommand = append(helmCommand, "--repo", options.Repository)
	}
	var valueData []byte
	if options.BaseDirectory != "" {
		if valueData, err = run.RunInDir(options.BaseDirectory, helmCommand...); err != nil {
			return
		}
	} else {
		if valueData, err = run.Run(helmCommand...); err != nil {
			return
		}
	}
	var yamlRoot yaml.Node
	err = yaml.Unmarshal(valueData, &yamlRoot)
	root = &yamlRoot
	return
}