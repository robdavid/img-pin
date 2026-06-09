package digester

import (
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"go.yaml.in/yaml/v4"
)

// ResourceHandler matches a specific kind of resource by looking at the
// structure and values of the YAML document. If the resource is identified an
// appropriate [Resource] processor implementation is returned, otherwise nil
// is returned.
type ResourceHandler interface {
	Match(doc *yaml.Node, imageDigester ImageDigester, options skipping.ImageOptions) types.Resource
}

type HandlerEntry struct {
	Name    string
	Handler ResourceHandler
}

var registry []HandlerEntry

func RegisterHandlerLast(name string, handler ResourceHandler) {
	registry = append(registry, HandlerEntry{Name: name, Handler: handler})
}

func RegisterHandlerFirst(name string, handler ResourceHandler) {
	registry = append([]HandlerEntry{{Name: name, Handler: handler}}, registry...)
}
