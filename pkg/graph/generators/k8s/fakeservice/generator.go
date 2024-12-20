package fakeservice

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/kong/mesh-perf/pkg/graph/apis"
	"github.com/kong/mesh-perf/pkg/graph/generators/k8s"
)

var Formatters = k8s.SimpleFormatters("fake-service")

func GeneratorOpts(registry string) []k8s.Option {
	if registry == "" {
		registry = "nicholasjackson"
	}
	return []k8s.Option{
		k8s.WithPort(9090),
		k8s.WithFormatters(Formatters),
		k8s.WithImage(fmt.Sprintf("%s/fake-service:v0.26.0", registry)),
		k8s.WithPodTemplateSpecMutator(mutatePodTemplate),
	}
}

func mutatePodTemplate(formatters k8s.Formatters, svc apis.Service, template *v1.PodTemplateSpec) error {
	var uris []string
	for _, v := range svc.Edges {
		uris = append(uris, formatters.Url(v, 9090))
	}
	template.Spec.Containers[0].Env = append(template.Spec.Containers[0].Env,
		v1.EnvVar{
			Name:  "SERVICE",
			Value: formatters.Name(svc.Idx),
		},
		v1.EnvVar{
			Name:  "UPSTREAM_URIS",
			Value: strings.Join(uris, ","),
		},
	)
	return nil
}
