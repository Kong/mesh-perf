// nolint:all  // code here is extracted and revised from github.com/gruntwork-io/terratest, this package should be removed once https://github.com/gruntwork-io/terratest/pull/1384 is merged
package silent_kubectl

import (
	"context"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func WaitUntilNumPodsCreatedE(
	t testing.TestingT,
	options *k8s.KubectlOptions,
	filters metav1.ListOptions,
	desiredCount int,
	retries int,
	sleepBetweenRetries time.Duration,
) error {
	retryMsg := fmt.Sprintf("Wait for num pods created to match desired count %d.", desiredCount)
	logger.Log(t, retryMsg)
	message, err := DoWithRetryE(
		t,
		retryMsg,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			pods, err := ListPodsE(t, options, filters)
			if err != nil {
				return "", err
			}
			if len(pods) != desiredCount {
				return "", k8s.DesiredNumberOfPodsNotCreated{Filter: filters, DesiredCount: desiredCount}
			}
			return "Desired number of Pods created", nil
		},
	)
	if err != nil {
		logger.Logf(t, "Timedout waiting for the desired number of Pods to be created: %s", err)
		return err
	}
	logger.Logf(t, message)
	return nil
}

func ListPodsE(t testing.TestingT, options *k8s.KubectlOptions, filters metav1.ListOptions) ([]corev1.Pod, error) {
	clientset, err := GetKubernetesClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}

	resp, err := clientset.CoreV1().Pods(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func WaitUntilPodAvailableE(t testing.TestingT, options *k8s.KubectlOptions, podName string, retries int, sleepBetweenRetries time.Duration) error {
	retryMsg := fmt.Sprintf("Wait for pod %s to become ready.", podName)
	logger.Log(t, retryMsg)
	message, err := DoWithRetryE(
		t,
		retryMsg,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			pod, err := GetPodE(t, options, podName)
			if err != nil {
				return "", err
			}
			if !k8s.IsPodAvailable(pod) {
				return "", k8s.NewPodNotAvailableError(pod)
			}
			return fmt.Sprintf("Pod %s is now available", podName), nil
		},
	)
	if err != nil {
		logger.Log(t, fmt.Sprintf("Timedout waiting for Pod %s to be provisioned: %s", podName, err))
		return err
	}
	logger.Logf(t, message)
	return nil
}

// GetPodE returns a Kubernetes pod resource in the provided namespace with the given name.
func GetPodE(t testing.TestingT, options *k8s.KubectlOptions, podName string) (*corev1.Pod, error) {
	clientset, err := GetKubernetesClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.CoreV1().Pods(options.Namespace).Get(context.Background(), podName, metav1.GetOptions{})
}
