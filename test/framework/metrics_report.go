package framework

import (
	"fmt"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/onsi/ginkgo/v2"
	"github.com/prometheus/client_golang/prometheus"
	prometheus_push "github.com/prometheus/client_golang/prometheus/push"

	"github.com/kumahq/kuma/v2/test/framework"
)

const pushGatewayApp = "prometheus-pushgateway"
const pushGatewayPort = 9091

var registry = prometheus.NewRegistry()
var testStatusStarted *prometheus.GaugeVec

func init() {
	testStatusStarted = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_status",
		Help: "If value is '1' then the test is in progress, if '0' then the test ended",
	}, []string{"spec"})
	registry.MustRegister(testStatusStarted)
}

func PushReportSpecMetric(cluster *framework.K8sCluster, ns string, value float64) error {
	endpoint, err := GetApiServerEndpoint(cluster, ns, pushGatewayApp, pushGatewayPort)
	if err != nil {
		return err
	}

	testStatusStarted.WithLabelValues(ginkgo.CurrentSpecReport().FullText()).Set(value)

	return prometheus_push.New(endpoint, "mesh_perf_test").Gatherer(registry).Push()
}

func InstallPrometheusPushgateway(cluster *framework.K8sCluster, ns string) error {
	if _, err := helm.RunHelmCommandAndGetOutputE(
		cluster.GetTesting(),
		&helm.Options{},
		"repo",
		"add",
		"--force-update",
		"prometheus-community",
		"https://prometheus-community.github.io/helm-charts",
	); err != nil {
		return err
	}

	if err := helm.InstallE(
		cluster.GetTesting(),
		&helm.Options{
			KubectlOptions: cluster.GetKubectlOptions(ns),
			SetStrValues: map[string]string{
				`serviceAnnotations.prometheus\.io/scrape`: "true",
				`serviceAnnotations.prometheus\.io/port`:   fmt.Sprintf("%d", pushGatewayPort),
				"podLabels.app":                            pushGatewayApp,
			},
		},
		"prometheus-community/prometheus-pushgateway",
		pushGatewayApp,
	); err != nil {
		return err
	}

	return nil
}
