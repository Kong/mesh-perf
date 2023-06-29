package k8s_test

import (
	"os"
	"testing"

	. "github.com/kumahq/kuma/test/framework"
	obs "github.com/kumahq/kuma/test/framework/deployments/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kumahq/kuma/pkg/test"
)

func TestE2E(t *testing.T) {
	test.RunE2ESpecs(t, "E2E Kubernetes Suite")
}

var cluster Cluster
var meshVersion string

var _ = BeforeSuite(func() {
	cluster = NewK8sCluster(NewTestingT(), Kuma1, true)
	err := cluster.Install(obs.Install(
		"obs",
		obs.WithNamespace("mesh-observability"),
		obs.WithComponents(obs.PrometheusComponent, obs.GrafanaComponent),
	))
	Expect(err).ToNot(HaveOccurred())
	meshVersion := os.Getenv("MESH_VERSION")
	if meshVersion == "" {
		panic("MESH_VERSION has to be defined")
	}
})

var _ = AfterSuite(func() {
	Expect(cluster.DeleteDeployment("obs")).To(Succeed())
})

var (
	_ = Describe("Simple", Simple, Ordered)
)
