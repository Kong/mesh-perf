package k8s_test

import (
	"os"
	"testing"

	. "github.com/kumahq/kuma/test/framework"
	obs "github.com/kumahq/kuma/test/framework/deployments/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/kong/mesh-perf/test/framework"
	"github.com/kumahq/kuma/pkg/test"
)

func TestE2E(t *testing.T) {
	test.RunE2ESpecs(t, "E2E Kubernetes Suite")
}

var cluster *K8sCluster
var meshVersion string

const (
	obsNamespace = "mesh-observability"
)

var _ = BeforeSuite(func() {
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath == "" {
		kubeConfigPath = "${HOME}/.kube/config"
	}
	cluster = NewK8sCluster(NewTestingT(), "mesh-perf", true)

	Expect(helm.RunHelmCommandAndGetOutputE(cluster.GetTesting(), &helm.Options{},
		"repo", "add", "--force-update", Config.HelmChartName, Config.HelmRepoUrl)).Error().To(BeNil())

	cluster.WithKubeConfig(os.ExpandEnv(kubeConfigPath))
	err := cluster.Install(obs.Install(
		"obs",
		obs.WithNamespace(obsNamespace),
		obs.WithComponents(obs.PrometheusComponent, obs.GrafanaComponent),
	))
	Expect(err).ToNot(HaveOccurred())
	Expect(framework.EnablePrometheusAdminAPI(obsNamespace, cluster)).To(Succeed())

	Expect(framework.InstallPrometheusPushgateway(cluster, obsNamespace))
	Eventually(func() error {
		return framework.PortForwardPrometheusPushgateway(cluster, obsNamespace)
	}, "30s", "1s").Should(Succeed())

	meshVersion := os.Getenv("MESH_VERSION")
	if meshVersion == "" {
		panic("MESH_VERSION has to be defined")
	}
})

var _ = AfterSuite(func() {
	promSnapshotsDir := os.Getenv("PROM_SNAPSHOTS_DIR")
	if promSnapshotsDir == "" {
		promSnapshotsDir = "/tmp/prom-snapshots"
	}
	Expect(framework.SavePrometheusSnapshot(cluster, obsNamespace, promSnapshotsDir)).To(Succeed())
	Expect(cluster.DeleteDeployment("obs")).To(Succeed())
})

var (
	_ = Describe("Simple", Simple, Ordered)
)
