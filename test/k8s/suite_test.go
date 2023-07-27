package k8s_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kumahq/kuma/pkg/test"
	. "github.com/kumahq/kuma/test/framework"
	obs "github.com/kumahq/kuma/test/framework/deployments/observability"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
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

type reportEntry struct {
	Time  string `yaml:"time"`
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type specReport struct {
	State         string `yaml:"state"`
	Description   string `yaml:"description"`
	ReportEntries []reportEntry
}

type report struct {
	Parameters       map[string]string `yaml:"parameters"`
	SuitePath        string            `yaml:"suitePath"`
	SuiteDescription string            `yaml:"suiteDescription"`
	SpecReports      []specReport      `yaml:"specReports"`
}

var _ = ReportAfterSuite("compile report", func(ginkgoReport Report) {
	parameters := map[string]string{}
	for _, envKeyVal := range os.Environ() {
		if strings.HasPrefix(envKeyVal, "PERF_TEST") {
			assignment := strings.SplitN(envKeyVal, "=", 2)
			parameters[assignment[0]] = assignment[1]
		}
	}

	report := report{
		Parameters:       parameters,
		SuitePath:        ginkgoReport.SuitePath,
		SuiteDescription: ginkgoReport.SuiteDescription,
	}

	for _, rep := range ginkgoReport.SpecReports {
		if rep.LeafNodeType != types.NodeTypeIt {
			continue
		}
		specReport := specReport{
			State:       rep.State.String(),
			Description: rep.LeafNodeText,
		}
		for _, entry := range rep.ReportEntries {
			specReport.ReportEntries = append(
				specReport.ReportEntries,
				reportEntry{
					Time:  entry.Time.String(),
					Name:  entry.Name,
					Value: entry.Value.String(),
				},
			)
		}
		report.SpecReports = append(
			report.SpecReports,
			specReport,
		)
	}

	reportBytes, err := yaml.Marshal(report)
	Expect(err).ToNot(HaveOccurred())

	// this is the directory of this file
	os.WriteFile("report.yaml", reportBytes, 0666)
})

var (
	_ = Describe("Simple", Simple, Ordered)
)
