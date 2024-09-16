package k8s_test

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/kumahq/kuma-tools/graph"
	"github.com/kumahq/kuma/pkg/config/core"
	. "github.com/kumahq/kuma/test/framework"
	"github.com/kumahq/kuma/test/framework/envoy_admin"
	"github.com/kumahq/kuma/test/framework/envoy_admin/tunnel"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/mesh-perf/test/framework"
)

func Simple() {
	var numServices int
	var instancesPerService int
	var start time.Time

	var svcGraph graph.Services

	var alternativeContainerRegistry string

	BeforeAll(func() {
		opts := []KumaDeploymentOption{
			WithCtlOpts(map[string]string{
				"--set": "" +
					"kuma.controlPlane.resources.requests.cpu=1," +
					"kuma.controlPlane.resources.requests.memory=2Gi," +
					"kuma.controlPlane.resources.limits.memory=32Gi",
				"--env-var": "" +
					"KUMA_RUNTIME_KUBERNETES_LEADER_ELECTION_LEASE_DURATION=100s," +
					"KUMA_RUNTIME_KUBERNETES_LEADER_ELECTION_RENEW_DEADLINE=80s",
			}),
		}

		alternativeContainerRegistry, _ = os.LookupEnv("ALTERNATIVE_CONTAINER_REGISTRY")

		if alternativeContainerRegistry != "" {
			opts = append(opts,
				WithCtlOpts(map[string]string{
					"--dataplane-registry": alternativeContainerRegistry,
				}))
		}

		opts = append(opts,
			WithCtlOpts(map[string]string{
				"--license-path": kmeshLicense,
			}))

		err := NewClusterSetup().
			Install(Kuma(core.Zone, opts...)).
			Install(NamespaceWithSidecarInjection(TestNamespace)).
			Setup(cluster)
		Expect(err).ToNot(HaveOccurred())

		num := requireVar("PERF_TEST_NUM_SERVICES")
		i, err := strconv.Atoi(num)
		Expect(err).ToNot(HaveOccurred(), "invalid value of PERF_TEST_NUM_SERVICES")
		numServices = i

		num = requireVar("PERF_TEST_INSTANCES_PER_SERVICE")
		i, err = strconv.Atoi(num)
		Expect(err).ToNot(HaveOccurred(), "invalid value of PERF_TEST_INSTANCES_PER_SERVICE")
		instancesPerService = i

		svcGraph = graph.GenerateRandomServiceMesh(872835240, numServices, 50, instancesPerService, instancesPerService)
	})

	BeforeEach(func() {
		Expect(framework.ReportSpecStart(cluster)).To(Succeed())
		start = time.Now()
	})

	AfterEach(func() {
		endpoint := cluster.GetPortForward("prometheus-server").ApiServerEndpoint
		promClient, err := framework.NewPromClient(fmt.Sprintf("http://%s", endpoint))
		Expect(err).ToNot(HaveOccurred())

		stopCh := make(chan struct{})
		metricCh := make(chan int)
		errCh := make(chan error)

		go framework.WatchXdsDeliveryCount(promClient, stopCh, metricCh, errCh)
		defer close(stopCh)

	Loop:
		for {
			select {
			case <-metricCh:
			case err := <-errCh:
				Fail(err.Error())
			case <-time.After(stabilizationSleep):
				break Loop
			}
		}

		Expect(framework.ReportSpecEnd(cluster)).To(Succeed())
		end := time.Now()
		AddReportEntry("duration", end.Sub(start).Milliseconds())
	})

	E2EAfterAll(func() {
		Expect(cluster.TriggerDeleteNamespace(TestNamespace)).To(Succeed())
		Expect(cluster.DeleteKuma()).To(Succeed())
	})

	It("should deploy the mesh", func() {
		// just to see stabilized stats before we go further
		Expect(true).To(BeTrue())
	})

	It("should deploy graph", func() {
		buffer := bytes.Buffer{}
		fakeServiceRegistry := "nicholasjackson"
		if alternativeContainerRegistry != "" {
			fakeServiceRegistry = alternativeContainerRegistry
		}
		Expect(svcGraph.ToYaml(&buffer, graph.ServiceConf{
			WithReachableServices: false,
			WithReachableBackends: true,
			WithKubeURIs:          true,
			MeshServicesMode:      "Exclusive",
			WithNamespace:         false,
			WithMesh:              true,
			Namespace:             TestNamespace,
			Mesh:                  "default",

			Image: fmt.Sprintf("%s/fake-service:v0.25.2", fakeServiceRegistry),
		})).To(Succeed())

		Expect(cluster.Install(YamlK8s(buffer.String()))).To(Succeed())

		Eventually(func() error {
			expectedNumOfPods := numServices * instancesPerService
			return k8s.WaitUntilNumPodsCreatedE(cluster.GetTesting(), cluster.GetKubectlOptions(TestNamespace),
				metav1.ListOptions{}, expectedNumOfPods, 1, 0)
		}, "10m", "3s").Should(Succeed())
	})

	It("should deploy mesh wide policy", func() {
		endpoint := cluster.GetPortForward("prometheus-server").ApiServerEndpoint
		promClient, err := framework.NewPromClient(fmt.Sprintf("http://%s", endpoint))
		Expect(err).ToNot(HaveOccurred())

		var acks int
		Eventually(func(g Gomega) {
			newAcks, err := framework.XdsAckRequestsReceived(promClient)
			g.Expect(err).ToNot(HaveOccurred())
			if acks != newAcks {
				acks = newAcks
				g.Expect(true).To(BeFalse(), "acks are not stable")
			}
		}, "2m", "5s").MustPassRepeatedly(7).Should(Succeed())

		policy := `
apiVersion: kuma.io/v1alpha1
kind: MeshRateLimit
metadata:
  name: mesh-rate-limit
  namespace: kong-mesh-system
spec:
  targetRef:
    kind: Mesh
    proxyTypes:
      - Sidecar
  from:
    - targetRef:
        kind: Mesh
      default:
        local:
          http:
            requestRate:
              num: 10000
              interval: 1s
            onRateLimit:
              status: 429
`
		Expect(cluster.Install(YamlK8s(policy))).To(Succeed())
		propagationStart := time.Now()

		Eventually(func(g Gomega) {
			newAcks, err := framework.XdsAckRequestsReceived(promClient)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(newAcks - acks).To(Equal(numServices * instancesPerService))
		}, "2m", "1s").Should(Succeed())
		AddReportEntry("policy_propagation_duration", time.Since(propagationStart).Milliseconds())
	})

	Context("scaling", func() {
		var admin envoy_admin.Tunnel

		var observer string
		var observable string

		BeforeAll(func() {
			// finding a service for scaling (observable) and a service to observe the scale (observer)
			for _, svc := range svcGraph {
				if len(svc.Edges) != 0 {
					observer = graph.ToName(svc.Idx)
					observable = graph.ToName(svc.Edges[0])
				}
			}
			pod := k8s.ListPods(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(TestNamespace),
				metav1.ListOptions{
					LabelSelector: fmt.Sprintf("app=%s", observer),
				},
			)[0]
			tnl := k8s.NewTunnel(cluster.GetKubectlOptions(TestNamespace), k8s.ResourceTypePod, pod.Name, 0, 9901)
			Expect(tnl.ForwardPortE(cluster.GetTesting())).To(Succeed())
			admin = tunnel.NewK8sEnvoyAdminTunnel(cluster.GetTesting(), tnl.Endpoint())
		})

		scale := func(replicas int) {
			err := k8s.RunKubectlE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(TestNamespace),
				"scale", "statefulset", observable, fmt.Sprintf("--replicas=%d", replicas),
			)
			Expect(err).ToNot(HaveOccurred())

			err = cluster.Install(WaitNumPods(TestNamespace, replicas, observable))
			Expect(err).ToNot(HaveOccurred())

			propagationStart := time.Now()
			Eventually(func(g Gomega) {
				membership, err := admin.GetStats(fmt.Sprintf("cluster.default_%s_kuma-test_default_msvc_80.membership_total", observable))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(membership.Stats).ToNot(BeEmpty())
				g.Expect(membership.Stats[0].Value).To(BeNumerically("==", replicas))
			}, "60s", "1s").Should(Succeed())
			AddReportEntry("endpoint_propagation_duration", time.Since(propagationStart).Milliseconds())
		}

		It("should scale up a service", func() {
			scale(instancesPerService + 1)
		})

		It("should scale down a service", func() {
			scale(instancesPerService)
		})
	})

	It("should distribute certs when mTLS is enabled", func() {
		expectedCerts := numServices * instancesPerService
		Expect(cluster.Install(MTLSMeshKubernetes("default"))).To(Succeed())

		propagationStart := time.Now()
		Eventually(func(g Gomega) {
			out, err := k8s.RunKubectlAndGetOutputE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(),
				"get", "meshinsights", "default", "-ojsonpath='{.spec.mTLS.issuedBackends.ca-1.total}'",
			)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(out).To(Equal(fmt.Sprintf("'%d'", expectedCerts)))
		}, "60s", "1s").Should(Succeed())
		AddReportEntry("certs_propagation_duration", time.Since(propagationStart).Milliseconds())
	})
}
