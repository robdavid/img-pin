package k8s_test

import (
	"testing"

	"github.com/robdavid/img-pin/pkgs/k8s"
	"github.com/stretchr/testify/assert"
)

func TestParseChartName(t *testing.T) {
	t.Run("bare name", func(t *testing.T) {
		assert := assert.New(t)
		cn := k8s.ParseChartName("harbor")
		assert.Equal("", cn.Dir)
		assert.Equal("harbor", cn.Base)
		assert.Equal("harbor", cn.Chart)
	})

	t.Run("oci", func(t *testing.T) {
		assert := assert.New(t)
		const url = "oci://ghcr.io/nginxinc/charts/nginx-ingress"
		cn := k8s.ParseChartName(url)
		assert.Equal("", cn.Dir)
		assert.Equal("nginx-ingress", cn.Base)
		assert.Equal(url, cn.Chart)
	})

	t.Run("file url", func(t *testing.T) {
		assert := assert.New(t)
		const url = "file:///path/chart"
		cn := k8s.ParseChartName(url)
		assert.Equal("/path/chart", cn.Dir)
		assert.Equal("chart", cn.Base)
		assert.Equal("/path/chart", cn.Chart)
	})

	t.Run("just file", func(t *testing.T) {
		assert := assert.New(t)
		const path = "/path/chart"
		cn := k8s.ParseChartName(path)
		assert.Equal("/path/chart", cn.Dir)
		assert.Equal("chart", cn.Base)
		assert.Equal("/path/chart", cn.Chart)
	})

	t.Run("relative file", func(t *testing.T) {
		assert := assert.New(t)
		const path = "../path/chart"
		cn := k8s.ParseChartName(path)
		assert.Equal("../path/chart", cn.Dir)
		assert.Equal("chart", cn.Base)
		assert.Equal("../path/chart", cn.Chart)
	})

	t.Run("chart with namespace", func(t *testing.T) {
		assert := assert.New(t)
		const path = "ns/chart"
		cn := k8s.ParseChartName(path)
		assert.Equal("", cn.Dir)
		assert.Equal("chart", cn.Base)
		assert.Equal("ns/chart", cn.Chart)
	})

}
