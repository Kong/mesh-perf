package k8s_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/testing"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kumahq/kuma/v2/pkg/config/core"
	. "github.com/kumahq/kuma/v2/test/framework"

	"github.com/kong/mesh-perf/pkg/graph/apis"
	graph_k8s "github.com/kong/mesh-perf/pkg/graph/generators/k8s"
	"github.com/kong/mesh-perf/pkg/graph/generators/k8s/fakeservice"
	"github.com/kong/mesh-perf/test/framework"
	"github.com/kong/mesh-perf/test/framework/silent_kubectl"
)

func ResourceLimits() {
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
			Setup(cluster)
		Expect(err).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		Expect(framework.PushReportSpecMetric(cluster, obsNamespace, 1)).To(Succeed())
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
		var maxMemory int
		var minimalMemoryRequired int
		var ranDuraionBeforeOOM time.Duration
		var numServices = 5
		var instancesPerService = 1

		adjustResource := func(miliCPU, memMega int, addGoMemLimitEnv bool, waitForComplete bool) {
			GinkgoHelper()
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

			Expect(k8s.RunKubectlE(
				cluster.GetTesting(),
				kumaNsOptions,
				"patch",
				"deployment",
				Config.KumaServiceName,
				"--type=json",
				"--patch",
				fmt.Sprintf("[%s]", strings.Join(patchJson, ",")),
			)).To(Succeed())

			if waitForComplete {
				Expect(k8s.RunKubectlE(
					cluster.GetTesting(),
					kumaNsOptions,
					"rollout",
					"status",
					"deployment",
					Config.KumaServiceName,
				)).To(Succeed())

				time.Sleep(3 * time.Second)

				Expect(k8s.WaitUntilDeploymentAvailableE(
					cluster.GetTesting(),
					kumaNsOptions,
					Config.KumaServiceName,
					12,
					10*time.Second,
				)).To(Succeed())

				output, err := retry.DoWithRetryE(
					cluster.GetTesting(),
					"wait for control plane to be ready",
					30,
					10*time.Second,
					func() (string, error) {
						logs := cluster.GetKumaCPLogs()

						for _, log := range logs {
							if strings.Contains(log, fmt.Sprintf("successfully acquired lease %s/cp-leader-lease", Config.KumaNamespace)) {
								return "adjusting control plane resource limits completed", nil
							}
						}

						return "", errors.New("control plane is not ready - leader is missing")
					},
				)
				Expect(err).ToNot(HaveOccurred())
				Logf(output)
			}
		}

		deployDPs := func() {
			GinkgoHelper()

			Logf("deploying %d services and %d instances per service\n", numServices, instancesPerService)

			svcGraph := apis.GenerateRandomMesh(872835240, numServices, 50, instancesPerService, instancesPerService)

			buffer := bytes.Buffer{}
			fakeServiceRegistry := "nicholasjackson"
			if containerRegistry != "" {
				fakeServiceRegistry = containerRegistry
			}
			opts := append(
				fakeservice.GeneratorOpts(
					fakeservice.WithRegistry(fakeServiceRegistry),
					fakeservice.WithReachableBackends(),
				),
				graph_k8s.WithNamespace(TestNamespace),
				graph_k8s.SkipNamespaceCreation(),
			)

			generator, err := graph_k8s.NewGenerator(opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(generator.Apply(&buffer, svcGraph)).To(Succeed())
			Expect(cluster.Install(YamlK8s(buffer.String()))).To(Succeed())
		}

		waitForDPs := func(ret chan<- bool) {
			GinkgoHelper()

			Logf("waiting for the data planes to become ready\n")
			go func() {
				defer GinkgoRecover()

				expectedNumOfPods := numServices * instancesPerService
				optsCopy := *cluster.GetKubectlOptions(TestNamespace)
				optsCopy.Logger = logger.Discard
				createErr := silent_kubectl.WaitUntilNumPodsCreatedE(cluster.GetTesting(), &optsCopy,
					metav1.ListOptions{}, expectedNumOfPods, 100, 8*time.Second)
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

		watchControlPlane := func(ctx context.Context, errCh chan<- error) {
			GinkgoHelper()

			Logf("monitoring health of control plane pods\n")

			clientset, err := silent_kubectl.GetKubernetesClientFromOptionsE(cluster.GetTesting(), cluster.GetKubectlOptions(Config.KumaNamespace))
			Expect(err).ToNot(HaveOccurred())

			watcher, err := clientset.CoreV1().Pods(Config.KumaNamespace).Watch(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s", Config.KumaServiceName),
			})
			Expect(err).ToNot(HaveOccurred())

			go func() {
				defer GinkgoRecover()

				for {
					select {
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

		scaleCPToOOMKilled := func(memory int, addGoMemLimit bool) (time.Duration, error) {
			GinkgoHelper()

			By("Scale up the CP using full resources")
			adjustResource(cpu, maxMemory, true, true)

			By("Deploy all the DPs")
			deployDPs()
			dpCh := make(chan bool)
			waitForDPs(dpCh)
			dpHealth := <-dpCh
			Expect(dpHealth).To(BeTrue(), "data planes should be run and available")

			By(fmt.Sprintf("Scale down the CP using less memory resources: %dMi", memory))
			timeStartPatch := time.Now()
			adjustResource(cpu, memory, true, false)

			errCh := make(chan error)
			ctx, cancelMonitoring := context.WithTimeout(context.Background(), 10*time.Minute)
			watchControlPlane(ctx, errCh)

			var err error
			select {
			case <-ctx.Done():
			case err = <-errCh:
			}
			cancelMonitoring()

			printUnavailablePods(cluster.GetTesting(), cluster.GetKubectlOptions(TestNamespace), metav1.ListOptions{})
			runDuration := time.Since(timeStartPatch)
			return runDuration, err
		}

		BeforeAll(func() {
			cpuLimit := requireVar("PERF_LIMIT_MILLI_CPU")
			i, err := strconv.Atoi(cpuLimit)
			Expect(err).ToNot(HaveOccurred(), "invalid value of PERF_LIMIT_MILLI_CPU")
			cpu = i

			memLimit := requireVar("PERF_LIMIT_MEGA_MEMORY")
			i, err = strconv.Atoi(memLimit)
			Expect(err).ToNot(HaveOccurred(), "invalid value of PERF_LIMIT_MEGA_MEMORY")
			maxMemory = i

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
		It("should load and get the minimal memory requirement", func() {
			By("Scale up the CP using full resources")
			adjustResource(cpu, maxMemory, false, true)

			By("Deploy all the DPs")
			deployDPs()
			dpCh := make(chan bool)
			waitForDPs(dpCh)
			dpHealth := <-dpCh
			Expect(dpHealth).To(BeTrue(), "data planes should be run and available")

			lastMemory := maxMemory
			knownUpperBound := maxMemory
			knownLowerBound := 0
			nextAction := "decrease"

			for {
				var currentMemory int
				var adjustment int
				switch nextAction {
				case "increase":
					adjustment = (knownUpperBound - lastMemory) / 2
					currentMemory = lastMemory + adjustment
				case "decrease":
					adjustment = (lastMemory - knownLowerBound) / 2
					currentMemory = lastMemory - adjustment
				}
				if adjustment <= 20 {
					// for decreasing, we want to make sure minimal required memory is actually smaller than the "good" one
					if nextAction == "decrease" {
						lastMemory = currentMemory
					}
					Logf("There is no more space to %s, so %d is the target usage", nextAction, lastMemory)
					break
				}
				lastMemory = currentMemory

				By(fmt.Sprintf("Trying to use memory %dMi on the control plane", currentMemory))
				adjustResource(cpu, currentMemory, false, false)

				errCh := make(chan error)
				ctx, cancelMonitoring := context.WithTimeout(context.Background(), 5*time.Minute)
				watchControlPlane(ctx, errCh)

				var err error
				select {
				case <-ctx.Done():
				case err = <-errCh:
				}
				cancelMonitoring()

				if err != nil {
					nextAction = "increase"
					if strings.Contains(err.Error(), "OOMKilled") {
						knownLowerBound = currentMemory
						Logf("Memory %dMi triggered OOMKilled", currentMemory)
					} else {
						Fail(fmt.Sprintf("control plane failed due to unexpected error: %v", err))
						return
					}
				} else {
					knownUpperBound = currentMemory
					nextAction = "decrease"
					Logf("Memory %dMi worked well, will try another round with less memory", currentMemory)
				}

				By(fmt.Sprintf("Restore to stable state using memory %dMi", maxMemory))
				adjustResource(cpu, maxMemory, false, true)
			}

			minimalMemoryRequired = lastMemory
			Expect(minimalMemoryRequired).To(BeNumerically(">", 0), "minimal memory should be greater than 0")
			Expect(minimalMemoryRequired).To(BeNumerically("<", maxMemory), "memory limit should fulfill CP resource requirement")
		})

		// we need to set reasonable number of services and instance numbers before decreasing the memory
		// by reasonable, it means the CP can support these numbers, and it should crash when memory is set to half
		It("should be OOM-killed without GOMEMLIMIT", func() {
			Expect(minimalMemoryRequired).To(BeNumerically(">", 0), "load and get the minimal memory requirement should before this test")

			runDuration, err := scaleCPToOOMKilled(minimalMemoryRequired, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("OOMKilled"), "control plane should crash with OOM Killed")
			ranDuraionBeforeOOM = runDuration
		})

		It("should crash slower or not crash when control plane has GOMEMLIMIT", func() {
			Expect(ranDuraionBeforeOOM).To(BeNumerically(">", 0), "control plane should crash with OOM Killed before this test")

			runDuration, err := scaleCPToOOMKilled(minimalMemoryRequired, true)

			if err == nil {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err.Error()).To(ContainSubstring("OOMKilled"), "control plane may crash with OOM Killed")
				Expect(runDuration-ranDuraionBeforeOOM).To(BeNumerically(">", 120*time.Second),
					"control plane should crash slower than the case of no GOMEMLIMIT")
			}
		})
	}, Ordered)
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
