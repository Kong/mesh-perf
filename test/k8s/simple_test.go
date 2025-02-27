package k8s_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mesh "github.com/kumahq/kuma/api/mesh/v1alpha1"
	"github.com/kumahq/kuma/pkg/config/core"
	"github.com/kumahq/kuma/pkg/test/resources/builders"
	. "github.com/kumahq/kuma/test/framework"
	"github.com/kumahq/kuma/test/framework/envoy_admin"
	"github.com/kumahq/kuma/test/framework/envoy_admin/tunnel"

	graph_apis "github.com/kong/mesh-perf/pkg/graph/apis"
	graph_k8s "github.com/kong/mesh-perf/pkg/graph/generators/k8s"
	"github.com/kong/mesh-perf/pkg/graph/generators/k8s/fakeservice"
	"github.com/kong/mesh-perf/test/framework"
)

func Simple() {
	var start time.Time
	var svcGraph graph_apis.ServiceGraph

	BeforeAll(func() {
		opts := []KumaDeploymentOption{
			WithSkipDefaultMesh(true),
			WithCtlOpts(map[string]string{
				"--set": strings.Join([]string{
					"kuma.controlPlane.resources.requests.cpu=1",
					"kuma.controlPlane.resources.requests.memory=2Gi",
					"kuma.controlPlane.resources.limits.memory=32Gi",
				}, ","),
				"--env-var": strings.Join([]string{
					"KUMA_RUNTIME_KUBERNETES_LEADER_ELECTION_LEASE_DURATION=100s",
					"KUMA_RUNTIME_KUBERNETES_LEADER_ELECTION_RENEW_DEADLINE=80s",
					fmt.Sprintf("KUMA_DIAGNOSTICS_DEBUG_ENDPOINTS=%v", debug),
				}, ","),
			}),
		}

		if containerRegistry != "" {
			opts = append(opts,
				WithCtlOpts(map[string]string{
					"--dataplane-registry": containerRegistry,
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

		Expect(cluster.Install(YamlK8s(builders.
			Mesh().
			WithMeshServicesEnabled(mesh.Mesh_MeshServices_Exclusive).
			WithBuiltinMTLSBackend("ca-1").
			WithEnabledMTLSBackend("ca-1").
			WithoutInitialPolicies().
			KubeYaml(),
		))).To(Succeed())

		Expect(cluster.Install(YamlK8s(`
apiVersion: kuma.io/v1alpha1
kind: MeshMetric
metadata:
  name: default
  namespace: kong-mesh-system
spec:
  default:
    backends:
    - type: Prometheus
      prometheus:
        port: 5670
        path: "/metrics"
    sidecar:
      profiles:
        appendProfiles:
        - name: Basic
`))).To(Succeed())

		svcGraph = graph_apis.GenerateRandomMesh(
			872835240,
			suiteNumServices,
			50,
			suiteNumInstances,
			suiteNumInstances,
		)
	})

	BeforeEach(func() {
		Expect(framework.PushReportSpecMetric(cluster, obsNamespace, 1)).To(Succeed())
		start = time.Now()
	})

	AfterEach(func(ctx context.Context) {
		promClient, err := framework.NewPromClient(cluster, obsNamespace)
		Expect(err).ToNot(HaveOccurred())

		stopCh := make(chan struct{})
		metricCh := make(chan int)
		errCh := make(chan error)

		go framework.WatchXdsDeliveryCount(ctx, promClient, stopCh, metricCh, errCh)
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

		Expect(framework.PushReportSpecMetric(cluster, obsNamespace, 0)).To(Succeed())
		end := time.Now()
		AddReportEntry("duration", end.Sub(start).Milliseconds())
	})

	E2EAfterAll(func() {
		Expect(cluster.DeleteNamespace(TestNamespace)).To(Succeed())
		Expect(cluster.DeleteKuma()).To(Succeed())
	})

	It("should deploy the mesh", func() {
		// just to see stabilized stats before we go further
		Expect(true).To(BeTrue())
	})

	It("should deploy graph", func() {
		buffer := bytes.Buffer{}
		opts := append(
			fakeservice.GeneratorOpts(
				fakeservice.WithRegistry(containerRegistry),
				fakeservice.WithReachableBackends(),
			),
			graph_k8s.WithNamespace(TestNamespace),
			graph_k8s.SkipNamespaceCreation(),
		)

		generator, err := graph_k8s.NewGenerator(opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(generator.Apply(&buffer, svcGraph)).To(Succeed())
		Expect(cluster.Install(YamlK8s(buffer.String()))).To(Succeed())

		Eventually(func() error {
			expectedNumOfPods := suiteNumServices * suiteNumInstances
			return k8s.WaitUntilNumPodsCreatedE(cluster.GetTesting(), cluster.GetKubectlOptions(TestNamespace),
				metav1.ListOptions{}, expectedNumOfPods, 1, 0)
		}, "10m", "3s").Should(Succeed())
	})

	It("should deploy mesh wide policy", func(ctx context.Context) {
		promClient, err := framework.NewPromClient(cluster, obsNamespace)
		Expect(err).ToNot(HaveOccurred())

		var acks int
		Eventually(func(g Gomega) {
			newAcks, err := framework.XdsAckRequestsReceived(ctx, promClient)
			g.Expect(err).ToNot(HaveOccurred())
			if acks != newAcks {
				acks = newAcks
				g.Expect(true).To(BeFalse(), "acks are not stable")
			}
		}, "10m", "5s").MustPassRepeatedly(7).Should(Succeed())

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
			newAcks, err := framework.XdsAckRequestsReceived(ctx, promClient)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(newAcks - acks).To(Equal(suiteNumServices * suiteNumInstances))
		}, "10m", "5s").Should(Succeed())
		AddReportEntry("policy_propagation_duration", time.Since(propagationStart).Milliseconds())
	})

	Context("scaling", func() {
		var admin envoy_admin.Tunnel

		var observer string
		var observable string

		BeforeAll(func() {
			// finding a service for scaling (observable) and a service to observe the scale (observer)
			for _, svc := range svcGraph.Services {
				if len(svc.Edges) != 0 {
					observer = fakeservice.Formatters.Name(svc.Idx)
					observable = fakeservice.Formatters.Name(svc.Edges[0])
					break
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
				"scale", "deployment", observable, fmt.Sprintf("--replicas=%d", replicas),
			)
			Expect(err).ToNot(HaveOccurred())

			err = cluster.Install(WaitNumPods(TestNamespace, replicas, observable))
			Expect(err).ToNot(HaveOccurred())

			propagationStart := time.Now()
			Eventually(func(g Gomega) {
				membership, err := admin.GetStats(fmt.Sprintf("cluster.default_%s_%s_default_msvc_9090.membership_total", observable, TestNamespace))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(membership.Stats).ToNot(BeEmpty())
				g.Expect(membership.Stats[0].Value).To(BeNumerically("==", replicas))
			}, "60s", "1s").Should(Succeed())
			AddReportEntry("endpoint_propagation_duration", time.Since(propagationStart).Milliseconds())
		}

		It("should scale up a service", func() {
			scale(suiteNumInstances + 1)
		})

		It("should scale down a service", func() {
			scale(suiteNumInstances)
		})
	})

	It("should distribute certs when mTLS is enabled", func() {
		expectedCerts := suiteNumServices * suiteNumInstances
		Expect(cluster.Install(
			YamlK8s(builders.
				Mesh().
				WithMeshServicesEnabled(mesh.Mesh_MeshServices_Exclusive).
				WithBuiltinMTLSBackend("ca-2").
				WithEnabledMTLSBackend("ca-2").
				WithoutInitialPolicies().
				KubeYaml(),
			))).To(Succeed())

		propagationStart := time.Now()
		Eventually(func(g Gomega) {
			out, err := k8s.RunKubectlAndGetOutputE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(),
				"get", "meshinsights", "default", "-ojsonpath='{.spec.mTLS.issuedBackends.ca-2.total}'",
			)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(out).To(Equal(fmt.Sprintf("'%d'", expectedCerts)))
		}, "600s", "5s").Should(Succeed())
		AddReportEntry("certs_propagation_duration", time.Since(propagationStart).Milliseconds())
	})
}
