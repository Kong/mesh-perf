package silent_kubectl

import (
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/testing"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// GetKubernetesClientFromOptionsE returns a Kubernetes API client given a configured KubectlOptions object.
func GetKubernetesClientFromOptionsE(t testing.TestingT, options *k8s.KubectlOptions) (*kubernetes.Clientset, error) {
	var err error
	var config *rest.Config

	if options.InClusterAuth {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else if options.RestConfig != nil {
		config = options.RestConfig
	} else {
		kubeConfigPath, err := options.GetConfigPath(t)
		if err != nil {
			return nil, err
		}
		// Load API config (instead of more low level ClientConfig)
		config, err = k8s.LoadApiClientConfigE(kubeConfigPath, options.ContextName)
		if err != nil {
			config, err = rest.InClusterConfig()
			if err != nil {
				return nil, err
			}
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
