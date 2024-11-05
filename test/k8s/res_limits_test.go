package k8s_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/testing"
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
	"github.com/kong/mesh-perf/test/framework/silent_kubectl"
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

		adjustResource := func(miliCPU, memMega int, addGoMemLimitEnv bool, waitForComplete bool) {
			Logf("adjusting control plane resource limits to cpu %dm, memory %dMi\n", miliCPU, memMega)
			kumaNsOptions := cluster.GetKubectlOptions(Config.KumaNamespace)

			out, err := k8s.RunKubectlAndGetOutputE(
				cluster.GetTesting(), kumaNsOptions,
				"get", "deployment", Config.KumaServiceName,
				"-o=jsonpath={.spec.template.spec.containers[0]}")
			Expect(err).ToNot(HaveOccurred())

			var container corev1.Container
			err = json.Unmarshal([]byte(out), &container)
			Expect(err).ToNot(HaveOccurred())

			patchJson := []string{
				fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/cpu", "value": "%dm"}`, miliCPU),
				fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/memory", "value": "%dMi"}`, memMega),
				fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/cpu", "value": "%dm"}`, miliCPU),
				fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value": "%dMi"}`, memMega),
			}

			if container.ReadinessProbe != nil {
				patchJson = append(patchJson, `{"op": "remove", "path": "/spec/template/spec/containers/0/readinessProbe"}`)
			}
			if container.LivenessProbe != nil {
				patchJson = append(patchJson, `{"op": "remove", "path": "/spec/template/spec/containers/0/livenessProbe"}`)
			}

			idxMemLimit := getEnvIndex(&container, "GOMEMLIMIT")
			idxMaxProcs := getEnvIndex(&container, "GOMAXPROCS")
			if idxMaxProcs != -1 && idxMaxProcs > idxMemLimit {
				patchJson = append(patchJson, fmt.Sprintf(`{"op": "remove", "path": "/spec/template/spec/containers/0/env/%s"}`, fmt.Sprintf("%d", idxMaxProcs)))
			}
			if addGoMemLimitEnv {
				// set 90% of the container memory as GOMEMLIMIT, the remaining 10% is for the rest of the container
				memLimit := int(float64(memMega) * 0.9)
				if memLimit < 10 {
					memLimit = 10
				}

				if idxMemLimit == -1 {
					patchJson = append(patchJson, fmt.Sprintf(`{"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": {"name": "GOMEMLIMIT", "value":"%dMiB"}}`, memLimit))
				} else {
					//  get the existing env array, and update the existing GOMEMLIMIT
					op := "replace"
					patchJson = append(patchJson, fmt.Sprintf(`{"op": "%s", "path": "/spec/template/spec/containers/0/env/%s", "value": {"name": "GOMEMLIMIT", "value":"%dMiB"}}`,
						op, fmt.Sprintf("%d", idxMemLimit), memLimit))
				}
			} else if idxMemLimit > -1 {
				patchJson = append(patchJson, fmt.Sprintf(`{"op": "remove", "path": "/spec/template/spec/containers/0/env/%s"}`, fmt.Sprintf("%d", idxMemLimit)))
			}
			if idxMaxProcs != -1 && idxMaxProcs < idxMemLimit {
				patchJson = append(patchJson, fmt.Sprintf(`{"op": "remove", "path": "/spec/template/spec/containers/0/env/%s"}`, fmt.Sprintf("%d", idxMaxProcs)))
			}

			err = k8s.RunKubectlE(
				cluster.GetTesting(),
				kumaNsOptions,
				"patch", "deployment", Config.KumaServiceName,
				"--type=json",
				"--patch", fmt.Sprintf("[%s]", strings.Join(patchJson, ",")),
			)
			Expect(err).ToNot(HaveOccurred())

			if waitForComplete {
				err = k8s.RunKubectlE(
					cluster.GetTesting(),
					kumaNsOptions,
					"rollout", "status", "deployment", Config.KumaServiceName)
				Expect(err).ToNot(HaveOccurred())
			}
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
				defer GinkgoRecover()

				expectedNumOfPods := numServices * instancesPerService
				optsCopy := *cluster.GetKubectlOptions(TestNamespace)
				optsCopy.Logger = logger.Discard
				createErr := silent_kubectl.WaitUntilNumPodsCreatedE(cluster.GetTesting(), &optsCopy,
					metav1.ListOptions{}, expectedNumOfPods, 100, 5*time.Second)
				created := createErr == nil

				pods, err2 := silent_kubectl.ListPodsE(cluster.GetTesting(), cluster.GetKubectlOptions(TestNamespace), metav1.ListOptions{})
				if err2 != nil {
					Logf("failed to list pods: %v\n", err2)
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
						defer GinkgoRecover()

						opts2 := *cluster.GetKubectlOptions(TestNamespace)
						opts2.Logger = logger.Discard
						podErr := silent_kubectl.WaitUntilPodAvailableE(cluster.GetTesting(), &opts2, p.Name, 60, 3*time.Second)
						Expect(podErr).ToNot(HaveOccurred())

						wg.Done()
						if podErr == nil {
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

		watchControlPlane := func(ctx context.Context, metricsCh chan<- string, errCh chan<- error) {
			Logf("monitoring health of control plane pods\n")

			clientset, err := silent_kubectl.GetKubernetesClientFromOptionsE(cluster.GetTesting(), cluster.GetKubectlOptions(Config.KumaNamespace))
			Expect(err).ToNot(HaveOccurred())

			watcher, err := clientset.CoreV1().Pods(Config.KumaNamespace).Watch(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s", Config.KumaServiceName),
			})
			Expect(err).ToNot(HaveOccurred())
			secondTicker := time.NewTicker(1 * time.Second)

			go func() {
				defer GinkgoRecover()

				for {
					select {
					case <-secondTicker.C:
						opts2 := *cluster.GetKubectlOptions(TestNamespace)
						opts2.Logger = logger.Discard
						pods, err := silent_kubectl.ListPodsE(
							cluster.GetTesting(),
							&opts2, metav1.ListOptions{
								LabelSelector: fmt.Sprintf("app=%s", Config.KumaServiceName),
							})
						Expect(err).ToNot(HaveOccurred())

						// find a ready pod and for longer than 10s
						if len(pods) < 1 {
							continue
						}

						var metricsPod *corev1.Pod
						for _, pod := range pods {
							p := pod
							if k8s.IsPodAvailable(&p) && time.Since(p.CreationTimestamp.Time) > 15*time.Second {
								metricsPod = &p
								break
							}
						}
						if metricsPod == nil {
							continue
						}

						metrics, err := k8s.RunKubectlAndGetOutputE(
							cluster.GetTesting(),
							&opts2,
							"exec", metricsPod.Name, "-c", "control-plane", "--", "sh", "-c", "wget -O - http://localhost:5680/metrics")
						if err == nil {
							metricsCh <- metrics
						}
					case e := <-watcher.ResultChan():
						if e.Object == nil {
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

						y, err := yaml.Marshal(pod)
						if err != nil {
							y = []byte(fmt.Sprintf("failed to marshal pod yaml: %v", err))
						}
						errCh <- fmt.Errorf("pod '%s' failed in phase '%s'\npod yaml: %s",
							pod.Name, pod.Status.Phase, y)
						watcher.Stop()
						return
					case <-ctx.Done():
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

		// we need to set reasonable number of services and instance numbers before decreasing the memory
		// by reasonable, it means the CP can support these numbers, and it should crash when memory is set to half
		It("should be OOM-killed without GOMEMLIMIT with half CP resource when deploy all services and instances", func() {
			By("Scale up the CP using full resources")
			adjustResource(cpu, memory, false, true)

			By("Deploy all the DPs")
			deployDPs()
			dpCh := make(chan bool)
			waitForDPs(dpCh)
			dpHealth := <-dpCh
			Expect(dpHealth).To(BeTrue(), "data planes should be run and available")

			By("Scale down the CP using 1/4 memory resources")
			adjustResource(cpu, memory/4, false, false)

			metricsCh := make(chan string)
			errCh := make(chan error)
			ctx, cancelMonitoring := context.WithTimeout(context.Background(), 10*time.Minute)
			watchControlPlane(ctx, metricsCh, errCh)
			go printCPMetrics(ctx, metricsCh)

			var err error
			select {
			case <-ctx.Done():
			case err = <-errCh:
			}
			cancelMonitoring()
			printUnavailablePods(cluster.GetTesting(), cluster.GetKubectlOptions(Config.KumaNamespace), metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", Config.KumaServiceName)})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("OOMKilled"), "control plane should crash with OOM Killed with half resource")

			adjustResource(cpu, memory, false, true)
		})

		It("should not crash when control plane has GOMEMLIMIT with half CP resource and deploy all services and instances", func() {
			By("Scale up the CP using full resources")
			adjustResource(cpu, memory, true, true)

			By("Deploy all the DPs")
			deployDPs()
			dpCh := make(chan bool)
			waitForDPs(dpCh)
			dpHealth := <-dpCh
			Expect(dpHealth).To(BeTrue(), "data planes should be run and available")

			By("Scale down the CP using 1/4 memory resources and GOMEMLIMIT env")
			adjustResource(cpu, memory/4, true, false)

			metricsCh := make(chan string)
			errCh := make(chan error)
			ctx, cancelMonitoring := context.WithTimeout(context.Background(), 10*time.Minute)
			watchControlPlane(ctx, metricsCh, errCh)
			go printCPMetrics(ctx, metricsCh)

			var err error
			select {
			case <-ctx.Done():
			case err = <-errCh:
			}
			cancelMonitoring()
			printUnavailablePods(cluster.GetTesting(), cluster.GetKubectlOptions(TestNamespace), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred(), "control plane should not crash")
		})
	})
}

func getEnvIndex(container *corev1.Container, envName string) int {
	for i, e := range container.Env {
		if e.Name == envName {
			return i
		}
	}
	return -1
}

func printUnavailablePods(t testing.TestingT, kubectlOptions *k8s.KubectlOptions, listOpts metav1.ListOptions) {
	pods, err := k8s.ListPodsE(t, kubectlOptions, listOpts)

	if err != nil {
		Logf("failed to list pods: %v\n", err)
		return
	}

	for _, pod := range pods {
		if k8s.IsPodAvailable(&pod) {
			continue
		}

		Logf("Pod %s: %s\n", pod.Name, pod.Status.Phase)
		for _, cts := range pod.Status.ContainerStatuses {
			Logf("Pod %s - %s: %s\n", pod.Name, cts.Name, cts.State.String())
		}
	}
}

func printCPMetrics(ctx context.Context, metricsCh chan string) {
	defer GinkgoRecover()

	var previousMetrics string
	for {
		select {
		case <-ctx.Done():
			if previousMetrics != "" {
				Logf("control plane metrics (last): %s\n", previousMetrics)
			}
			return
		case metrics := <-metricsCh:
			if previousMetrics == "" {
				Logf("control plane metrics (first): %s\n", metrics)
			}
			previousMetrics = metrics
		}
	}
}
