package framework

import (
	"github.com/kumahq/kuma/test/framework"
)

func Customize(config *framework.E2eConfig) {
	config.KumaImageRegistry = "kong"
	config.KumaNamespace = "kong-mesh-system"
	config.KumaServiceName = "kong-mesh-control-plane"
	config.ZoneEgressApp = "kong-mesh-egress"
	config.ZoneIngressApp = "kong-mesh-ingress"
	config.KumaGlobalZoneSyncServiceName = "kong-mesh-global-zone-sync"
	config.CNIApp = "kong-mesh-cni"
	config.HelmChartPath = "kong-mesh/kong-mesh"
	config.HelmChartName = "kong-mesh"
	config.HelmSubChartPrefix = "kuma."
	config.HelmRepoUrl = "https://kong.github.io/kong-mesh-charts"
	config.DefaultClusterStartupRetries = 60
}
