package k8s

import (
	"errors"
	"fmt"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kong/mesh-perf/pkg/graph/apis"
)

type PodTemplateSpecMutator func(
	formatters Formatters,
	svc apis.Service,
	template *v1.PodTemplateSpec,
) error

// Generator is a Kubernetes resource generator
type generator struct {
	asStatefulSet           bool
	namespace               string
	image                   string
	port                    int32
	formatters              Formatters
	configMapGenerator      func(formatters Formatters, svc apis.Service) (string, error)
	podTemplateSpecMutators []PodTemplateSpecMutator
	skipNamespaceCreation   bool
}

type Formatters struct {
	BaseName string
	Name     func(idx int) string
	Url      func(idx int, port int) string
}

func SimpleFormatters(baseName string) Formatters {
	return Formatters{
		BaseName: baseName,
		Name: func(idx int) string {
			return fmt.Sprintf("%s-%03d", baseName, idx)
		},
		Url: func(idx int, port int) string {
			return fmt.Sprintf("http://%s-%03d:%d", baseName, idx, port)
		},
	}

}

type Option interface {
	Apply(g *generator) error
}
type OptionFn func(g *generator) error

func (f OptionFn) Apply(g *generator) error {
	return f(g)
}

func WithConfigMapGenerator(fn func(f Formatters, svc apis.Service) (string, error)) Option {
	return OptionFn(func(g *generator) error {
		g.configMapGenerator = fn
		return nil
	})
}

func WithPodTemplateSpecMutators(fns ...PodTemplateSpecMutator) Option {
	return OptionFn(func(g *generator) error {
		g.podTemplateSpecMutators = slices.Concat(g.podTemplateSpecMutators, fns)
		return nil
	})
}

func WithPort(p int) Option {
	return OptionFn(func(g *generator) error {
		g.port = int32(p)
		return nil
	})
}

func WithFormatters(f Formatters) Option {
	return OptionFn(func(g *generator) error {
		g.formatters = f
		return nil
	})
}

func WithImage(image string) Option {
	return OptionFn(func(g *generator) error {
		g.image = image
		return nil
	})
}

func WithNamespace(name string) Option {
	return OptionFn(func(g *generator) error {
		g.namespace = name
		return nil
	})
}

func AsStatefulSet() Option {
	return OptionFn(func(g *generator) error {
		g.asStatefulSet = true
		return nil
	})
}

func SkipNamespaceCreation() Option {
	return OptionFn(func(g *generator) error {
		g.skipNamespaceCreation = true
		return nil
	})
}

func NewGenerator(opts ...Option) (Generator, error) {
	out := Generator{
		Serializer: DefaultSerializer,
	}
	g := &generator{
		formatters: SimpleFormatters("microservice"),
	}
	for _, o := range opts {
		if err := o.Apply(g); err != nil {
			return out, err
		}
	}
	out.WorkloadGenerator = g
	if !g.skipNamespaceCreation {
		out.CommonSetup = commonSetup(g.namespace)
	}
	return out, nil
}

func commonSetup(ns string) CommonSetupFn {
	return func(svcs apis.ServiceGraph) ([]runtime.Object, []byte, error) {
		ns := &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
			Spec: v1.NamespaceSpec{},
		}
		return []runtime.Object{
			ns,
		}, nil, nil
	}

}

func (g generator) Apply(svc apis.Service) ([]runtime.Object, []byte, error) {
	if g.image == "" {
		return nil, nil, errors.New("must set an image")
	}
	if g.port < 0 || g.port > 65535 {
		return nil, nil, errors.New("invalid port")
	}
	name := g.formatters.Name(svc.Idx)
	baseObjectMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: g.namespace,
		Labels: map[string]string{
			"app": name,
		},
	}
	var workload runtime.Object
	podTemplateSpec := v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{},
			Containers: []v1.Container{
				{
					Name:            "app",
					Image:           g.image,
					ImagePullPolicy: v1.PullAlways,
					VolumeMounts:    []v1.VolumeMount{},
					LivenessProbe: &v1.Probe{
						InitialDelaySeconds: 3,
						ProbeHandler: v1.ProbeHandler{
							HTTPGet: &v1.HTTPGetAction{
								Port: intstr.FromInt32(g.port),
								Path: "/health",
							},
						},
					},
					ReadinessProbe: &v1.Probe{
						InitialDelaySeconds: 3,
						ProbeHandler: v1.ProbeHandler{
							HTTPGet: &v1.HTTPGetAction{
								Port: intstr.FromInt32(g.port),
								Path: "/ready",
							},
						},
					},
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("32Mi"),
						},
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("32Mi"),
							v1.ResourceCPU:    resource.MustParse("100m"),
						},
					},
				},
			},
		},
	}
	if g.configMapGenerator != nil {
		podTemplateSpec.Spec.Volumes = append(podTemplateSpec.Spec.Volumes, v1.Volume{
			Name: "config",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: name,
					},
				},
			},
		})
		podTemplateSpec.Spec.Containers[0].VolumeMounts = append(podTemplateSpec.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      "config",
			MountPath: "/etc/config",
		})
	}
	baseObjectMeta.DeepCopyInto(&podTemplateSpec.ObjectMeta)
	if g.podTemplateSpecMutators != nil {
		for _, mutator := range g.podTemplateSpecMutators {
			if err := mutator(g.formatters, svc, &podTemplateSpec); err != nil {
				return nil, nil, err
			}
		}
	}

	repl := int32(svc.Replicas)
	if g.asStatefulSet {
		sts := &appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			},
			Spec: appsv1.StatefulSetSpec{
				ServiceName: name,
				Replicas:    &repl,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": name,
					},
				},
				Template: podTemplateSpec,
			},
		}
		baseObjectMeta.DeepCopyInto(&sts.ObjectMeta)
		workload = sts
	} else {
		surge := intstr.FromString("25%")
		deployment := &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &repl,
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxSurge:       &surge,
						MaxUnavailable: &surge,
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": name,
					},
				},
				Template: podTemplateSpec,
			},
		}
		baseObjectMeta.DeepCopyInto(&deployment.ObjectMeta)
		workload = deployment
	}

	http := "http"
	service := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": name,
			},
			Ports: []v1.ServicePort{
				{
					Name:        "api",
					AppProtocol: &http,
					Port:        g.port,
					TargetPort:  intstr.FromInt32(g.port),
				},
			},
		},
	}
	baseObjectMeta.DeepCopyInto(&service.ObjectMeta)
	outObj := []runtime.Object{
		workload,
		service,
	}

	if g.configMapGenerator != nil {
		conf, err := g.configMapGenerator(g.formatters, svc)
		if err != nil {
			return nil, nil, err
		}
		configMap := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: g.namespace,
				Labels: map[string]string{
					"app": name,
				},
			},
			Data: map[string]string{
				"config.yaml": conf,
			},
		}
		baseObjectMeta.DeepCopyInto(&configMap.ObjectMeta)
		outObj = append(outObj, configMap)
	}

	return outObj, nil, nil
}
