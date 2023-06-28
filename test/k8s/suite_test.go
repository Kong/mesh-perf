package k8s_test

import (
	"os"
	"testing"

	. "github.com/kumahq/kuma/test/framework"
	. "github.com/onsi/ginkgo/v2"

	"github.com/kumahq/kuma/pkg/test"
)

func TestE2E(t *testing.T) {
	test.RunE2ESpecs(t, "E2E Kubernetes Suite")
}

var cluster Cluster
var meshVersion string

var _ = BeforeSuite(func() {
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath == "" {
		kubeConfigPath = "${HOME}/.kube/config"
	}
	cluster = NewK8sCluster(NewTestingT(), "mesh-perf", true)

	cluster.(*K8sCluster).WithKubeConfig(os.ExpandEnv(kubeConfigPath))
	meshVersion := os.Getenv("MESH_VERSION")
	if meshVersion == "" {
		panic("MESH_VERSION has to be defined")
	}
})

var (
	_ = Describe("Simple", Simple, Ordered)
)
