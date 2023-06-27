package k8s_test

import (
	"fmt"
	"strings"

	"github.com/gruntwork-io/terratest/modules/random"
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
			// todo we don't have images published
			//Install(testserver.Install(
			//	testserver.WithName("demo-client"),
			//)).
			//Install(testserver.Install(
			//	testserver.WithName("test-server"),
			//)).
			Setup(cluster)
		Expect(err).ToNot(HaveOccurred())
	})

	E2EAfterAll(func() {
		Expect(cluster.DeleteNamespace(TestNamespace)).To(Succeed())
		Expect(cluster.DeleteKuma()).To(Succeed())
	})

	It("should pass", func() {
		Expect(true).To(BeTrue())
	})
}
