package k8s_test

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/kumahq/kuma-tools/graph"
	"github.com/kumahq/kuma/pkg/config/core"
	. "github.com/kumahq/kuma/test/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func Simple() {
	BeforeAll(func() {
		err := NewClusterSetup().
			Install(Kuma(core.Standalone,
				WithInstallationMode(HelmInstallationMode),
				WithHelmChartVersion(meshVersion),
				WithHelmReleaseName(fmt.Sprintf("kuma-%s", strings.ToLower(random.UniqueId()))),
				WithHelmChartPath(Config.HelmChartName), // we pass chart name to use production chart
				WithoutHelmOpt("global.image.tag"),      // required to use production chart
			)).
			Install(NamespaceWithSidecarInjection(TestNamespace)).
			Setup(cluster)
		Expect(err).ToNot(HaveOccurred())
	})

	E2EAfterAll(func() {
		Expect(cluster.DeleteNamespace(TestNamespace)).To(Succeed())
		Expect(cluster.DeleteKuma()).To(Succeed())
	})

	It("should deploy graph", func() {
		numServices := 5 // todo provide a license to spin up more
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
			Image:                 "nicholasjackson/fake-service:v0.21.1",
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.Install(YamlK8s(buffer.String()))).To(Succeed())

		wg := sync.WaitGroup{}
		wg.Add(numServices)
		for i := 0; i < numServices; i++ {
			name := fmt.Sprintf("srv-%03d", i)
			go func() {
				defer GinkgoRecover()
				err := NewClusterSetup().
					Install(WaitService(TestNamespace, name)).
					Install(WaitNumPods(TestNamespace, 1, name)).
					Install(WaitPodsAvailable(TestNamespace, name)).
					Setup(cluster)
				Expect(err).ToNot(HaveOccurred())
				wg.Done()
			}()
		}
		wg.Wait()
	})
}
