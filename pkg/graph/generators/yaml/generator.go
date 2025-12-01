package yaml

import (
	"io"

	"sigs.k8s.io/yaml"

	"github.com/kong/mesh-perf/pkg/graph/apis"
)

// Generator outputs the service graph as a yaml.
var Generator = apis.GeneratorFunc(func(writer io.Writer, svc apis.ServiceGraph) error {
	data, err := yaml.Marshal(svc)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
})
