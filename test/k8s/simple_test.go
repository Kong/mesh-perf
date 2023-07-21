package k8s_test

import (
	"bytes"
	"encoding/base64"
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
	numServices := 5

	BeforeAll(func() {
		opts := []KumaDeploymentOption{}

		if license := os.Getenv("KMESH_LICENSE_INLINE"); license != "" {
			licenseEncoded := base64.StdEncoding.EncodeToString([]byte(license))
			err := NewClusterSetup().
				Install(Namespace(Config.KumaNamespace)).
				Install(YamlK8s(fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: kong-mesh-license
  namespace: %s
type: Opaque
data:
  license.json: %s
`, Config.KumaNamespace, licenseEncoded))).
				Setup(cluster)
			Expect(err).ToNot(HaveOccurred())
			opts = append(opts,
				WithHelmOpt("controlPlane.secrets[0].Env", "KMESH_LICENSE_INLINE"),
				WithHelmOpt("controlPlane.secrets[0].Secret", "kong-mesh-license"),
				WithHelmOpt("controlPlane.secrets[0].Key", "license.json"),
			)
		}

		err := NewClusterSetup().
			Install(Kuma(core.Standalone, opts...)).
			Install(NamespaceWithSidecarInjection(TestNamespace)).
			Setup(cluster)
		Expect(err).ToNot(HaveOccurred())
		if num := os.Getenv("TEST_NUM_SERVICES"); num != "" {
			i, err := strconv.Atoi(num)
			Expect(err).ToNot(HaveOccurred(), "invalid value of TEST_NUM_SERVICES")
			numServices = i
		}
	})

	BeforeEach(func() {
		Expect(ReportSpecStart(cluster)).To(Succeed())
	})

	AfterEach(func() {
		time.Sleep(stabilizationSleep)
		Expect(ReportSpecEnd(cluster)).To(Succeed())
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
		services := graph.GenerateRandomServiceMesh(time.Now().Unix(), numServices, 50, 1, 1)
		buffer := bytes.Buffer{}
		err := services.ToYaml(&buffer, graph.ServiceConf{
			WithReachableServices: true,
			WithNamespace:         false,
			WithMesh:              true,
			Namespace:             TestNamespace,
			Mesh:                  "default",
			Image:                 "nicholasjackson/fake-service:v0.25.2",
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.Install(YamlK8s(buffer.String()))).To(Succeed())

		wg := sync.WaitGroup{}
		wg.Add(numServices)
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

		//time.Sleep(10 * time.Hour)
	})

	It("should deploy mesh wide policy", func() {
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

		Eventually(func(g Gomega) {
			newDeliveryCount, err := XdsDeliveryCount(promClient)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(newDeliveryCount - deliveryCount).To(Equal(numServices))
		}, "60s", "1s").Should(Succeed())
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
			scale(2)
			// there is no straightforward way to check if all Envoys received the config with the new endpoint, therefore we need to rely on stabilization sleep
		})

		It("should scale down a service", func() {
			scale(1)
			// there is no straightforward way to check if all Envoys received the config without the removed endpoint, therefore we need to rely on stabilization sleep
		})
	})

	It("should distribute certs when mTLS is enabled", func() {
		Expect(cluster.Install(MTLSMeshKubernetes("default"))).To(Succeed())

		Eventually(func(g Gomega) {
			out, err := k8s.RunKubectlAndGetOutputE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(),
				"get", "meshinsights", "default", "-ojsonpath='{.spec.mTLS.issuedBackends.ca-1.total}'",
			)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(out).To(Equal(fmt.Sprintf("'%d'", numServices)))
		}, "60s", "1s").Should(Succeed())
	})
}
