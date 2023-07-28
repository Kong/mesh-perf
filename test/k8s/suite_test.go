package k8s_test

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/kumahq/kuma/pkg/test"
	. "github.com/kumahq/kuma/test/framework"
	obs "github.com/kumahq/kuma/test/framework/deployments/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/kong/mesh-perf/test/framework"
)

func TestE2E(t *testing.T) {
	test.RunE2ESpecs(t, "E2E Kubernetes Suite")
}

var cluster *K8sCluster
var stabilizationSleep time.Duration

const obsNamespace = "mesh-observability"

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
	if cluster != nil {
		Expect(framework.SavePrometheusSnapshot(cluster, obsNamespace, promSnapshotsDir)).To(Succeed())
		Expect(cluster.DeleteDeployment("obs")).To(Succeed())
	}
})

var _ = ReportAfterSuite("compile report", func(ginkgoReport Report) {
	report := makeReport(ginkgoReport)

	reportBytes, err := yaml.Marshal(report)
	Expect(err).ToNot(HaveOccurred())

	reportDir := os.Getenv("REPORT_DIR")
	if reportDir == "" {
		reportDir = "/tmp/perf-test-reports"
	}
	root := requireVar("TEST_ROOT")
	relativeSuitePath := strings.TrimPrefix("/", strings.TrimPrefix(report.SuitePath, root))
	fileName := fmt.Sprintf("%s.yaml", strings.ReplaceAll(relativeSuitePath, "/", "_"))
	Expect(os.MkdirAll(reportDir, os.ModePerm))
	Expect(os.WriteFile(path.Join(reportDir, fileName), reportBytes, 0666)).To(Succeed())
})

var (
	_ = Describe("Simple", Simple, Ordered)
)
