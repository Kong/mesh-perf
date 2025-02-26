package k8s_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/kennygrant/sanitize"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8s_strings "k8s.io/utils/strings"

	"github.com/kong/mesh-perf/test/framework"

	"github.com/kumahq/kuma/pkg/test"
	. "github.com/kumahq/kuma/test/framework"
	obs "github.com/kumahq/kuma/test/framework/deployments/observability"
)

func TestE2E(t *testing.T) {
	test.RunE2ESpecs(t, "E2E Kubernetes Suite")
}

const obsNamespace = "monitoring"

var (
	cluster            *K8sCluster
	stabilizationSleep time.Duration
	suiteNumServices   int
	suiteNumInstances  int
	kmeshLicense       string
	containerRegistry  string
	debug              bool
)

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

	if v, ok := os.LookupEnv("DEBUG"); ok {
		debug = slices.Contains([]string{"1", "true"}, v)
	}

	containerRegistry = os.Getenv("CONTAINER_REGISTRY")

	kmeshLicense = requireVar("KMESH_LICENSE")
	sleep := requireVar("PERF_TEST_STABILIZATION_SLEEP")
	sleepDur, err := time.ParseDuration(sleep)
	if err != nil {
		panic(err)
	}
	stabilizationSleep = sleepDur

	suiteNumServices, err = strconv.Atoi(requireVar("PERF_TEST_NUM_SERVICES"))
	Expect(err).ToNot(HaveOccurred(), "invalid value of PERF_TEST_NUM_SERVICES")

	suiteNumInstances, err = strconv.Atoi(requireVar("PERF_TEST_INSTANCES_PER_SERVICE"))
	Expect(err).ToNot(HaveOccurred(), "invalid value of PERF_TEST_INSTANCES_PER_SERVICE")

	cluster = NewK8sCluster(NewTestingT(), "mesh-perf", true)

	cluster.WithKubeConfig(os.ExpandEnv(kubeConfigPath))

	obsComponents := []obs.Component{obs.PrometheusComponent}
	if debug {
		obsComponents = append(obsComponents, obs.GrafanaComponent)
	}

	Expect(cluster.Install(obs.Install(
		"obs",
		obs.WithNamespace(obsNamespace),
		obs.WithComponents(obsComponents...),
	))).To(Succeed())

	patchObs := framework.NewPatcher(cluster, obsNamespace)

	Expect(patchObs(
		framework.KindDeployment,
		framework.NamePrometheusServer,
		slices.Concat(
			framework.EnablePrometheusAdminAPIPatch(),
			framework.SetPrometheusResourcesPatch(),
		),
	)).To(Succeed())

	if debug {
		Expect(patchObs(framework.KindDeployment, framework.NameGrafana, framework.GrafanaDeploymentPatch())).To(Succeed())
		Expect(patchObs(framework.KindService, framework.NameGrafana, framework.GrafanaServicePatch())).To(Succeed())
	}

	Expect(framework.InstallPrometheusPushgateway(cluster, obsNamespace))

	Eventually(func() error {
		return framework.PortForwardPrometheusPushgateway(cluster, obsNamespace)
	}, "300s", "1s").Should(Succeed())
	Eventually(func() error {
		return framework.PortForwardPrometheusServer(cluster, obsNamespace)
	}, "300s", "1s").Should(Succeed())
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
