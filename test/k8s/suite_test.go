package k8s_test

import (
	"os"
	"testing"
	"time"

	"github.com/kumahq/kuma/pkg/test"
	. "github.com/kumahq/kuma/test/framework"
	obs "github.com/kumahq/kuma/test/framework/deployments/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kong/mesh-perf/test/framework"
)

func TestE2E(t *testing.T) {
	test.RunE2ESpecs(t, "E2E Kubernetes Suite")
}

var cluster *K8sCluster
var stabilizationSleep = 10 * time.Second

const obsNamespace = "mesh-observability"

var _ = BeforeSuite(func() {
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath == "" {
		kubeConfigPath = "${HOME}/.kube/config"
	}

	if sleep := os.Getenv("STABILIZATION_SLEEP"); sleep != "" {
		sleepDur, err := time.ParseDuration(sleep)
		if err != nil {
			panic(err)
		}
		stabilizationSleep = sleepDur
	}

	cluster = NewK8sCluster(NewTestingT(), "mesh-perf", true)

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
	Eventually(func() error {
		return framework.PortForwardPrometheusServer(cluster, obsNamespace)
	}, "30s", "1s").Should(Succeed())
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
