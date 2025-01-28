package k8s_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/kennygrant/sanitize"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8s_strings "k8s.io/utils/strings"

	"github.com/kumahq/kuma/pkg/test"
	. "github.com/kumahq/kuma/test/framework"
	obs "github.com/kumahq/kuma/test/framework/deployments/observability"

	"github.com/kong/mesh-perf/test/framework"
)

func TestE2E(t *testing.T) {
	test.RunE2ESpecs(t, "E2E Kubernetes Suite")
}

var cluster *K8sCluster
var stabilizationSleep time.Duration

const obsNamespace = "mesh-observability"

var kmeshLicense string

func requireVar(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("couldn't lookup value %s", key))
	}

	return val
}

var _ = BeforeSuite(func() {
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath == "" {
		kubeConfigPath = "${HOME}/.kube/config"
	}

	kmeshLicense = requireVar("KMESH_LICENSE")
	sleep := requireVar("PERF_TEST_STABILIZATION_SLEEP")
	sleepDur, err := time.ParseDuration(sleep)
	if err != nil {
		panic(err)
	}
	stabilizationSleep = sleepDur

	cluster = NewK8sCluster(NewTestingT(), "mesh-perf", true)

	cluster.WithKubeConfig(os.ExpandEnv(kubeConfigPath))
	Expect(cluster.Install(obs.Install(
		"obs",
		obs.WithNamespace(obsNamespace),
		obs.WithComponents(obs.PrometheusComponent, obs.GrafanaComponent),
	))).To(Succeed())

	// Prometheus PVCs are tied to specific nodes and can't be moved if we change the node
	// where Prometheus runs. To fix this, we create a new PVC and replace it in the Prometheus
	// deployment. The storage size is increased to 80GB to avoid running out of space when
	// deploying 2000 workloads, as the default 8GB might not be enough and could cause hard-to-debug issues.
	Expect(cluster.Install(YamlK8sObject(framework.PVC80GiPrometheus(obsNamespace)))).To(Succeed())

	patchObs := framework.NewPatcher(
		cluster,
		obsNamespace,
		// We explicitly specify the node where observability components like Prometheus are deployed
		// to ensure they are not disrupted by other workloads. When deploying a large number of services,
		// Prometheus resource requirements grow quickly, and if it shares a node with many other pods,
		// there may not be enough resources available for it to function properly.
		framework.SetObservabilityTolerations(),
	)

	Expect(patchObs(
		framework.NamePrometheusServer,
		framework.EnablePrometheusAdminAPIPatch(),
		framework.SetPrometheusResourcesPatch(),
		framework.SetPrometheusPVC80GiPatch(),
	)).To(Succeed())

	Expect(patchObs(framework.NamePrometheusKubeStateMetrics)).To(Succeed())
	Expect(patchObs(framework.NameGrafana)).To(Succeed())

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
	if cluster != nil {
		Expect(framework.SavePrometheusSnapshot(cluster, obsNamespace, promSnapshotsDir)).To(Succeed())
		Expect(cluster.DeleteDeployment("obs")).To(Succeed())
	}
})

var _ = ReportAfterSuite("compile report", func(ginkgoReport Report) {
	reportDir := os.Getenv("REPORT_DIR")
	if reportDir == "" {
		reportDir = "/tmp/perf-test-reports"
	}
	Expect(os.MkdirAll(reportDir, os.ModePerm)).ToNot(HaveOccurred())

	specReports := framework.MakeSpecReports(ginkgoReport)
	for _, specReport := range specReports {
		specReportBytes, err := json.Marshal(specReport)
		Expect(err).ToNot(HaveOccurred())

		fileName := fmt.Sprintf("%s.json", k8s_strings.ShortenString(sanitize.Name(specReport.Description), 250))
		Expect(os.WriteFile(path.Join(reportDir, fileName), specReportBytes, 0666)).To(Succeed())
	}
})

var (
	_ = Describe("Simple", Simple, Ordered)
	_ = Describe("ResourceLimits", Label("limits"), ResourceLimits, Ordered)
)
