package k8s_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/kumahq/kuma-tools/graph"
	"github.com/kumahq/kuma/pkg/config/core"
	. "github.com/kumahq/kuma/test/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kong/mesh-perf/test/framework"
)

func Simple() {
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
	})

	BeforeEach(func() {
		Expect(ReportSpecStart(cluster)).To(Succeed())
	})

	AfterEach(func() {
		Expect(ReportSpecEnd(cluster)).To(Succeed())
	})

	E2EAfterAll(func() {
		Expect(cluster.DeleteNamespace(TestNamespace)).To(Succeed())
		Expect(cluster.DeleteKuma()).To(Succeed())
	})

	It("should deploy graph", func() {
		numServices := 5
		if num := os.Getenv("TEST_NUM_SERVICES"); num != "" {
			i, err := strconv.Atoi(num)
			Expect(err).ToNot(HaveOccurred(), "invalid value of TEST_NUM_SERVICES")
			numServices = i
		}
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
	})
}
