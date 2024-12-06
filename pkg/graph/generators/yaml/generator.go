package yaml

import (
	"io"

	"gopkg.in/yaml.v3"

	"github.com/kong/mesh-perf/pkg/graph/apis"
)

// Generator outputs the service graph as a yaml.
var Generator = apis.GeneratorFunc(func(writer io.Writer, svc apis.ServiceGraph) error {
	return yaml.NewEncoder(writer).Encode(svc)
})
