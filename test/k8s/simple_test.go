package k8s_test

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/kumahq/kuma-tools/graph"
	"github.com/kumahq/kuma/pkg/config/core"
	. "github.com/kumahq/kuma/test/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kong/mesh-perf/test/framework"
)

func Simple() {
	var numServices int
	var instancesPerService int
	var start time.Time

	BeforeAll(func() {
		opts := []KumaDeploymentOption{}

		if license := os.Getenv("KMESH_LICENSE"); license != "" {
			opts = append(opts,
				WithCtlOpts(map[string]string{
					"--license-path": license,
				}))
		}

		err := NewClusterSetup().
			Install(Kuma(core.Standalone, opts...)).
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
	})

	BeforeEach(func() {
		Expect(ReportSpecStart(cluster)).To(Succeed())
		start = time.Now()
		AddReportEntry("spec.start", start)
	})

	AfterEach(func() {
		time.Sleep(stabilizationSleep)
		Expect(ReportSpecEnd(cluster)).To(Succeed())
		end := time.Now()
		AddReportEntry("spec.end", end)
		AddReportEntry("spec.duration", end.Sub(start))
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
		services := graph.GenerateRandomServiceMesh(872835240, numServices, 50, instancesPerService, instancesPerService)
		buffer := bytes.Buffer{}
		Expect(services.ToYaml(&buffer, graph.ServiceConf{
			WithReachableServices: true,
			WithNamespace:         false,
			WithMesh:              true,
			Namespace:             TestNamespace,
			Mesh:                  "default",
			Image:                 "nicholasjackson/fake-service:v0.25.2",
		})).To(Succeed())

		Expect(cluster.Install(YamlK8s(buffer.String()))).To(Succeed())

		wg := sync.WaitGroup{}
		wg.Add(numServices)

		start := time.Now()

		for i := 0; i < numServices; i++ {
			name := fmt.Sprintf("srv-%03d", i)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				err := NewClusterSetup().
					Install(WaitService(TestNamespace, name)).
					Install(WaitNumPods(TestNamespace, 1, name)).
					Install(WaitPodsAvailable(TestNamespace, name)).
					Setup(cluster)
				Expect(err).ToNot(HaveOccurred())
			}()
		}
		wg.Wait()

		AddReportEntry("duration", time.Now().Sub(start))
	})

	It("should deploy mesh wide policy", func() {
		endpoint := cluster.GetPortForward("prometheus-server").ApiServerEndpoint
		promClient, err := NewPromClient(fmt.Sprintf("http://%s", endpoint))
		Expect(err).ToNot(HaveOccurred())

		deliveryCount, err := XdsDeliveryCount(promClient)
		Expect(err).ToNot(HaveOccurred())

		policy := `
apiVersion: kuma.io/v1alpha1
kind: MeshRateLimit
metadata:
  name: mesh-rate-limit
  namespace: kong-mesh-system
spec:
  targetRef:
    kind: Mesh
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
		start := time.Now()

		Eventually(func(g Gomega) {
			newDeliveryCount, err := XdsDeliveryCount(promClient)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(newDeliveryCount - deliveryCount).To(Equal(numServices))
		}, "60s", "1s").Should(Succeed())
		AddReportEntry("duration", time.Now().Sub(start))
	})

	Context("scaling", func() {
		scale := func(replicas int) {
			err := k8s.RunKubectlE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(TestNamespace),
				"scale", "statefulset", "srv-000", fmt.Sprintf("--replicas=%d", replicas),
			)
			Expect(err).ToNot(HaveOccurred())

			err = cluster.Install(WaitNumPods(TestNamespace, replicas, "srv-000"))
			Expect(err).ToNot(HaveOccurred())
		}

		It("should scale up a service", func() {
			start := time.Now()
			scale(2)
			// there is no straightforward way to check if all Envoys received the config with the new endpoint, therefore we need to rely on stabilization sleep
			AddReportEntry("duration", time.Now().Sub(start))
		})

		It("should scale down a service", func() {
			start := time.Now()
			scale(1)
			// there is no straightforward way to check if all Envoys received the config without the removed endpoint, therefore we need to rely on stabilization sleep
			AddReportEntry("duration", time.Now().Sub(start))
		})
	})

	It("should distribute certs when mTLS is enabled", func() {
		Expect(cluster.Install(MTLSMeshKubernetes("default"))).To(Succeed())

		start := time.Now()
		Eventually(func(g Gomega) {
			out, err := k8s.RunKubectlAndGetOutputE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(),
				"get", "meshinsights", "default", "-ojsonpath='{.spec.mTLS.issuedBackends.ca-1.total}'",
			)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(out).To(Equal(fmt.Sprintf("'%d'", numServices)))
		}, "60s", "1s").Should(Succeed())
		AddReportEntry("duration", time.Now().Sub(start))
	})
}
