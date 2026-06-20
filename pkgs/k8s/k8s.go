package k8s

import (
	"net/url"
	"path"
	"strings"
)

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
