package fakeservice

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/kong/mesh-perf/pkg/graph/apis"
	"github.com/kong/mesh-perf/pkg/graph/generators/k8s"

	"github.com/kumahq/kuma/v2/api/common/v1alpha1"
	"github.com/kumahq/kuma/v2/pkg/plugins/runtime/k8s/controllers"
	"github.com/kumahq/kuma/v2/pkg/plugins/runtime/k8s/metadata"
	"github.com/kumahq/kuma/v2/pkg/util/pointer"
)

var Formatters = k8s.SimpleFormatters("fake-service")

type Options struct {
	imageRegistry        string
	useReachableBackends bool
	useReachableServices bool
}

type OptionFn func(Options) Options

func WithRegistry(imageRegistry string) OptionFn {
	return func(o Options) Options {
		if imageRegistry != "" {
			o.imageRegistry = imageRegistry
		}
		return o
	}
}

func WithReachableBackends() OptionFn {
	return func(o Options) Options {
		o.useReachableBackends = true
		return o
	}
}

func WithReachableServices() OptionFn {
	return func(o Options) Options {
		o.useReachableServices = true
		return o
	}
}

func GeneratorOpts(fns ...OptionFn) []k8s.Option {
	opts := Options{
		imageRegistry: "nicholasjackson",
	}

	for _, fn := range fns {
		if fn != nil {
			opts = fn(opts)
		}
	}

	return []k8s.Option{
		k8s.WithPort(9090),
		k8s.WithFormatters(Formatters),
		k8s.WithImage(fmt.Sprintf("%s/fake-service:v0.26.0", opts.imageRegistry)),
		k8s.WithPodTemplateSpecMutators(
			mutatePodTemplate,
			mutateMaybe(opts.useReachableServices && !opts.useReachableBackends, configureReachableServices),
			mutateMaybe(opts.useReachableBackends, configureReachableBackends),
		),
	}
}

func mutateMaybe(predicate bool, fn k8s.PodTemplateSpecMutator) k8s.PodTemplateSpecMutator {
	if !predicate {
		return nil
	}

	return fn
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

func configureReachableBackends(formatters k8s.Formatters, svc apis.Service, template *v1.PodTemplateSpec) error {
	var refs controllers.ReachableBackendRefs

	for _, v := range svc.Edges {
		refs.Refs = append(refs.Refs, &controllers.ReachableBackendRef{
			Kind:      string(v1alpha1.MeshService),
			Name:      pointer.To(formatters.Name(v)),
			Namespace: &template.Namespace,
		})
	}

	refsAnnotationValue, err := json.Marshal(refs)
	if err != nil {
		return err
	}

	if template.Annotations == nil {
		template.Annotations = map[string]string{}
	}

	template.Annotations[metadata.KumaReachableBackends] = string(refsAnnotationValue)

	return nil
}

func configureReachableServices(formatters k8s.Formatters, svc apis.Service, template *v1.PodTemplateSpec) error {
	var names []string

	for _, v := range svc.Edges {
		names = append(names, fmt.Sprintf(
			"%s_%s_svc_%d",
			formatters.Name(v),
			template.GetNamespace(),
			9090,
		))
	}

	if template.Annotations == nil {
		template.Annotations = map[string]string{}
	}

	if len(names) == 0 {
		names = append(names, fmt.Sprintf(
			"%s_%s_svc_%d",
			formatters.Name(svc.Idx),
			template.GetNamespace(),
			9090,
		))
	}

	template.Annotations[metadata.KumaTransparentProxyingReachableServicesAnnotation] = strings.Join(names, ",")

	return nil
}
