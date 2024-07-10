package k8s_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/gruntwork-io/terratest/modules/logger"
	"k8s.io/apimachinery/pkg/watch"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/kumahq/kuma-tools/graph"
	"github.com/kumahq/kuma/pkg/config/core"
	. "github.com/kumahq/kuma/test/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/mesh-perf/test/framework"
)

func ResourceLimits() {

	var alternativeContainerRegistry string

	BeforeAll(func() {
		opts := []KumaDeploymentOption{
			WithCtlOpts(map[string]string{
				"--set": "" +
					"kuma.controlPlane.resources.requests.cpu=500m," +
					"kuma.controlPlane.resources.requests.memory=512Mi," +
					"kuma.controlPlane.resources.limits.cpu=2000m," +
					"kuma.controlPlane.resources.limits.memory=2048Mi",
				"--env-var": "" +
					"GODEBUG=gctrace=1," +
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
			Setup(cluster)
		Expect(err).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		Expect(framework.ReportSpecStart(cluster)).To(Succeed())
	})

	E2EAfterAll(func() {
		Expect(cluster.DeleteKuma()).To(Succeed())
	})

	It("should deploy the mesh", func() {
		// just to see stabilized stats before we go further
		Expect(true).To(BeTrue())
	})

	It("should deploy mesh wide policy", func() {
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
	})

	Context("load", func() {
		var cpu int
		var memory int
		var numServices = 5
		var instancesPerService = 1
		var goMemLimitAdded = false

		adjustResource := func(miliCPU, memMega int, addGoMemLimitEnv bool) {
			Logf("adjusting control plane resource limits to cpu %dm, memory %dMi\n", miliCPU, memMega)

			patchJson := []string{
				fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/cpu", "value": "%dm"}`, miliCPU),
				fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/memory", "value": "%dMi"}`, memMega),
				fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/cpu", "value": "%dm"}`, miliCPU),
				fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value": "%dMi"}`, memMega),
			}

			if addGoMemLimitEnv {
				// set 90% of the container memory as GOMEMLIMIT, the remaining 10% is for the rest of the container
				memLimit := int(float64(memMega) * 0.9)
				if memLimit < 10 {
					memLimit = 10
				}

				if !goMemLimitAdded {
					patchJson = append(patchJson, fmt.Sprintf(`{"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": {"name": "GOMEMLIMIT", "value":"%dMiB"}}`, memLimit))
					goMemLimitAdded = true
				} else {
					//  get the existing env array, and update the existing GOMEMLIMIT
					out, err := k8s.RunKubectlAndGetOutputE(
						cluster.GetTesting(),
						cluster.GetKubectlOptions(Config.KumaNamespace),
						"get", "deployment", Config.KumaServiceName,
						"-o=jsonpath='{.spec.template.spec.containers[0].env}'")
					Expect(err).ToNot(HaveOccurred())
					var jsonEnvArray []map[string]interface{}
					err = yaml.Unmarshal([]byte(out), &jsonEnvArray)
					Expect(err).ToNot(HaveOccurred())
					var idx = -1
					for i, m := range jsonEnvArray {
						if m["name"] == "GOMEMLIMIT" {
							idx = i
							break
						}
					}

					op := "replace"
					idxStr := fmt.Sprintf("%d", idx)
					if idx == -1 {
						op = "add"
						idxStr = "-"
					}

					patchJson = append(patchJson, fmt.Sprintf(`{"op": "%s", "path": "/spec/template/spec/containers/0/env/%s", "value": {"name": "GOMEMLIMIT", "value":"%dMiB"}}`,
						op, idxStr, memLimit))
				}
			}

			err := k8s.RunKubectlE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(Config.KumaNamespace),
				"patch", "deployment", Config.KumaServiceName,
				"--type=json",
				"--patch", fmt.Sprintf("[%s]", strings.Join(patchJson, ",")),
			)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)

			err = k8s.RunKubectlE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(Config.KumaNamespace),
				"rollout", "status", "deployment", Config.KumaServiceName)
			Expect(err).ToNot(HaveOccurred())
		}

		deployDPs := func() {
			Logf("deploying %d services and %d instances per service\n", numServices, instancesPerService)

			svcGraph := graph.GenerateRandomServiceMesh(872835240, numServices, 50, instancesPerService, instancesPerService)

			buffer := bytes.Buffer{}
			fakeServiceRegistry := "nicholasjackson"
			if alternativeContainerRegistry != "" {
				fakeServiceRegistry = alternativeContainerRegistry
			}
			Expect(svcGraph.ToYaml(&buffer, graph.ServiceConf{
				WithReachableServices: true,
				WithNamespace:         false,
				WithMesh:              true,
				Namespace:             TestNamespace,
				Mesh:                  "default",
				Image:                 fmt.Sprintf("%s/fake-service:v0.25.2", fakeServiceRegistry),
			})).To(Succeed())

			Expect(cluster.Install(YamlK8s(buffer.String()))).To(Succeed())
		}

		waitForDPs := func(ret chan<- bool) {
			Logf("waiting for the data planes to become ready\n")
			go func() {
				expectedNumOfPods := numServices * instancesPerService
				created := Eventually(func() error {
					opts := cluster.GetKubectlOptions(TestNamespace)
					opts.Logger = logger.Discard
					return k8s.WaitUntilNumPodsCreatedE(cluster.GetTesting(), opts,
						metav1.ListOptions{}, expectedNumOfPods, 1, 0)
				}, "10m", "3s").Should(Succeed())

				pods, err := k8s.ListPodsE(cluster.GetTesting(), cluster.GetKubectlOptions(TestNamespace), metav1.ListOptions{})
				if err != nil {
					Logf("failed to list pods: %v\n", err)
					ret <- false
					return
				}

				if created {
					Logf("%d pods are now all created\n", expectedNumOfPods)
				} else {
					Logf("only %d of %d pods created\n", len(pods), expectedNumOfPods)
					ret <- created
					return
				}

				wg := sync.WaitGroup{}
				wg.Add(len(pods))
				for _, pod := range pods {
					go func(p *corev1.Pod) {
						available := Eventually(func() error {
							opts := cluster.GetKubectlOptions(TestNamespace)
							opts.Logger = logger.Discard
							return k8s.WaitUntilPodAvailableE(cluster.GetTesting(), opts, p.Name, 1, 0)
						}, "3m", "3s").Should(Succeed())

						wg.Done()
						if available {
							Logf("pod %s is now available\n", p.Name)
						} else {
							Logf("pod %s failed to become available\n", p.Name)
							ret <- false
						}
					}(&pod)
				}
				wg.Wait()
				Logf("dataplane pods are now all available, entering the stabilization sleep")
				time.Sleep(stabilizationSleep)
				ret <- true
			}()
		}

		watchControlPlane := func(ctx context.Context, errCh chan<- error) {
			Logf("monitoring health of control plane pods for at most 10 min\n")

			clientset, err := k8s.GetKubernetesClientFromOptionsE(cluster.GetTesting(), cluster.GetKubectlOptions(Config.KumaNamespace))
			Expect(err).ToNot(HaveOccurred())

			ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
			watcher, err := clientset.CoreV1().Pods(Config.KumaNamespace).Watch(ctx2, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s", Config.KumaServiceName),
			})
			Expect(err).ToNot(HaveOccurred())
			secondTicker := time.NewTicker(1 * time.Second)

			go func() {
				for {
					select {
					case <-secondTicker.C:
						pods, err := k8s.ListPodsE(
							cluster.GetTesting(),
							cluster.GetKubectlOptions(Config.KumaNamespace), metav1.ListOptions{
								LabelSelector: fmt.Sprintf("app=%s", Config.KumaServiceName),
							})
						Expect(err).ToNot(HaveOccurred())
						pod := pods[0]
						metrics, err := k8s.RunKubectlAndGetOutputE(
							cluster.GetTesting(),
							cluster.GetKubectlOptions(Config.KumaNamespace),
							"exec", pod.Name, "-c", "control-plane", "--", "sh", "-c", "wget -O - http://localhost:5680/metrics")
						Expect(err).ToNot(HaveOccurred())
						Logf("control plane metrics: %s\n", metrics)
					case e := <-watcher.ResultChan():
						if e.Object == nil {
							cancel()
							return
						}

						pod, ok := e.Object.(*corev1.Pod)
						if !ok {
							continue
						}

						if e.Type == watch.Deleted || e.Type == watch.Added {
							continue
						}

						if e.Type == watch.Error {
							errCh <- errors.New("error watching pod")
							continue
						}

						if pod.DeletionTimestamp != nil ||
							pod.Status.Phase == "Pending" ||
							k8s.IsPodAvailable(pod) {
							continue
						}

						args := []string{"logs", "-c", "control-plane", pod.Name}
						if hasPodContainerCrashed(pod) {
							args = append(args, "-p")
						}

						podLogs, err := k8s.RunKubectlAndGetOutputE(
							cluster.GetTesting(),
							cluster.GetKubectlOptions(Config.KumaNamespace),
							args...)
						Expect(err).ToNot(HaveOccurred())

						y, err := yaml.Marshal(pod)
						if err != nil {
							y = []byte(fmt.Sprintf("failed to marshal pod yaml: %v", err))
						}
						errCh <- fmt.Errorf("pod '%s' failed in phase '%s'; pod yaml: %s, pod logs: %s",
							pod.Name, pod.Status.Phase, y, podLogs)
						watcher.Stop()
						return
					case <-ctx2.Done():
						watcher.Stop()
						close(errCh)
						return
					}
				}
			}()
		}

		BeforeAll(func() {
			cpuLimit := requireVar("PERF_LIMIT_MILLI_CPU")
			i, err := strconv.Atoi(cpuLimit)
			Expect(err).ToNot(HaveOccurred(), "invalid value of PERF_LIMIT_MILLI_CPU")
			cpu = i

			memLimit := requireVar("PERF_LIMIT_MEGA_MEMORY")
			i, err = strconv.Atoi(memLimit)
			Expect(err).ToNot(HaveOccurred(), "invalid value of PERF_LIMIT_MEGA_MEMORY")
			memory = i

			svcNum, _ := os.LookupEnv("PERF_TEST_NUM_SERVICES")
			insNum, _ := os.LookupEnv("PERF_TEST_INSTANCES_PER_SERVICE")

			if svcNum != "" {
				numServices, _ = strconv.Atoi(svcNum)
			}
			if insNum != "" {
				instancesPerService, _ = strconv.Atoi(insNum)
			}
		})

		BeforeEach(func() {
			Expect(cluster.Install(NamespaceWithSidecarInjection(TestNamespace))).To(Succeed())
		})

		AfterEach(func() {
			Expect(cluster.DeleteNamespace(TestNamespace)).To(Succeed())
		})

		// should not crash, dump memory usage
		It("should deploy all services and instances", func() {

			adjustResource(cpu, memory, false)

			deployDPs()

			errCh := make(chan error)
			ctx, cancelMonitoring := context.WithCancel(context.Background())
			defer cancelMonitoring()
			watchControlPlane(ctx, errCh)

			dpCh := make(chan bool)
			waitForDPs(dpCh)

			var err error
			select {
			case dpRet := <-dpCh:
				Logf("dpCh returned\n")
				Expect(dpRet).To(BeTrue(), "data planes should be run and available")
			case err = <-errCh:
				Logf("errCh returned\n")
			}
			Expect(err).ToNot(HaveOccurred(), "control plane should not crash")
		})

		// half the resource, should crash without GOMEMLIMIT
		It("should crash when deploy all services and instances with half CP resource", func() {
			adjustResource(cpu/2, memory/2, false)

			deployDPs()

			errCh := make(chan error)
			ctx, cancelMonitoring := context.WithCancel(context.Background())
			watchControlPlane(ctx, errCh)

			dpCh := make(chan bool)
			waitForDPs(dpCh)

			var err error
			select {
			case <-dpCh:
				Logf("dpCh returned\n")
			case err = <-errCh:
				Logf("errCh returned\n")
			}
			cancelMonitoring()
			Expect(err).To(HaveOccurred(), "control plane should crash with half resource")

			cpPods, err := k8s.ListPodsE(cluster.GetTesting(), cluster.GetKubectlOptions(Config.KumaNamespace), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s", Config.KumaServiceName),
			})
			Expect(err).ToNot(HaveOccurred())
			for _, pod := range cpPods {
				if k8s.IsPodAvailable(&pod) {
					continue
				}

				_, _ = fmt.Fprintf(GinkgoWriter, "Pod %s: %s\n", pod.Name, pod.Status.Phase)
				for _, cts := range pod.Status.ContainerStatuses {
					_, _ = fmt.Fprintf(GinkgoWriter, "Pod %s - %s: %s\n", pod.Name, cts.Name, cts.State.String())
				}
			}
		})

		It("should not crash when control plane has GOMEMLIMIT set when all services and instances with half CP resource", func() {
			adjustResource(cpu/2, memory/2, true)

			deployDPs()

			errCh := make(chan error)
			ctx, cancelMonitoring := context.WithCancel(context.Background())
			watchControlPlane(ctx, errCh)

			dpCh := make(chan bool)
			waitForDPs(dpCh)

			var err error
			select {
			case <-dpCh:
				Logf("dpCh returned\n")
			case err = <-errCh:
				Logf("errCh returned\n")
			}
			cancelMonitoring()
			Expect(err).ToNot(HaveOccurred(), "control plane should not crash")
			if err != nil {
				_, _ = fmt.Fprint(GinkgoWriter, err.Error())
			}

			dpPods, err := k8s.ListPodsE(cluster.GetTesting(), cluster.GetKubectlOptions(TestNamespace), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			for _, pod := range dpPods {
				if k8s.IsPodAvailable(&pod) {
					continue
				}

				_, _ = fmt.Fprintf(GinkgoWriter, "Pod %s: %s\n", pod.Name, pod.Status.Phase)
				for _, cts := range pod.Status.ContainerStatuses {
					_, _ = fmt.Fprintf(GinkgoWriter, "Pod %s - %s: %s\n", pod.Name, cts.Name, cts.State.String())
				}
			}

		})
	})
}

func hasPodContainerCrashed(pod *corev1.Pod) bool {
	for _, cts := range pod.Status.ContainerStatuses {
		if cts.RestartCount > 0 {
			return true
		}
	}

	return false
}
