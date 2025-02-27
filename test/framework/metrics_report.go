package framework

import (
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/onsi/ginkgo/v2"
	"github.com/prometheus/client_golang/prometheus"
	prometheus_push "github.com/prometheus/client_golang/prometheus/push"

	"time"

	"github.com/kumahq/kuma/test/framework"
)

const prometheusPushgatewayApp = "prometheus-pushgateway"

var registry = prometheus.NewRegistry()
var testStatusStarted *prometheus.GaugeVec

func init() {
	testStatusStarted = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_status",
		Help: "If value is '1' then the test is in progress, if '0' then the test ended",
	}, []string{"spec"})
	registry.MustRegister(testStatusStarted)
}

func ReportSpecStart(cluster *framework.K8sCluster) error {
	return push(cluster, 1)
}

func ReportSpecEnd(cluster *framework.K8sCluster) error {
	return push(cluster, 0)
}

func push(cluster *framework.K8sCluster, value float64) error {
	endpoint := cluster.GetPortForward(prometheusPushgatewayApp).ApiServerEndpoint
	testStatusStarted.WithLabelValues(ginkgo.CurrentSpecReport().FullText()).Set(value)
	_, err := retry.DoWithRetryableErrorsE(
		cluster.GetTesting(),
		"push metrics",
		map[string]string{
			"connection refused": "connect: connection refused",
		},
		60,
		10*time.Second,
		func() (string, error) {
			return "", prometheus_push.New(endpoint, "mesh_perf_test").Gatherer(registry).Push()
		},
	)
	return err
}

func InstallPrometheusPushgateway(cluster *framework.K8sCluster, ns string) error {
	_, err := helm.RunHelmCommandAndGetOutputE(cluster.GetTesting(), &helm.Options{},
		"repo", "add", "--force-update", "prometheus-community", "https://prometheus-community.github.io/helm-charts")
	if err != nil {
		return err
	}

	err = helm.InstallE(cluster.GetTesting(), &helm.Options{
		KubectlOptions: cluster.GetKubectlOptions(ns),
		SetStrValues: map[string]string{
			`serviceAnnotations.prometheus\.io/scrape`: "true",
			`serviceAnnotations.prometheus\.io/port`:   "9091",
			"podLabels.app":                            prometheusPushgatewayApp,
		},
	}, "prometheus-community/prometheus-pushgateway", prometheusPushgatewayApp)
	if err != nil {
		return err
	}

	return nil
}

func PortForwardPrometheusPushgateway(cluster *framework.K8sCluster, ns string) error {
	return cluster.PortForwardService(prometheusPushgatewayApp, ns, 9091)
}
