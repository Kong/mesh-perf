package framework

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/onsi/ginkgo/v2"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kumahq/kuma/test/framework"
)

const (
	NamePrometheusServer           = "prometheus-server"
	NamePrometheusKubeStateMetrics = "prometheus-kube-state-metrics"
	NameGrafana                    = "grafana"
)

const (
	namePVC80GiPrometheus      = NamePrometheusServer + "-80"
	nameNodeGroupObservability = "observability"
)

var toleration = corev1.Toleration{
	Key:      "ObservabilityOnly",
	Operator: corev1.TolerationOpExists,
	Effect:   corev1.TaintEffectNoSchedule,
}

func PVC80GiPrometheus(ns string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namePVC80GiPrometheus,
			Namespace: ns,
			Labels: map[string]string{
				"app":       "prometheus",
				"component": "server",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("80Gi"),
				},
			},
		},
	}
}

type Patcher func(deployment string, operations ...[]json.RawMessage) error

func NewPatcher(cluster framework.Cluster, namespace string, baseOperations ...[]json.RawMessage) Patcher {
	return func(deployment string, operations ...[]json.RawMessage) error {
		return ApplyJSONPatch(
			cluster,
			namespace,
			deployment,
			slices.Concat(slices.Concat(baseOperations, operations)...),
		)
	}
}

func ApplyJSONPatch(cluster framework.Cluster, namespace, deployment string, operations []json.RawMessage) error {
	patch, err := json.Marshal(operations)
	if err != nil {
		return err
	}

	return k8s.RunKubectlE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(namespace),
		"patch", "deployment", deployment, "--type", "json", "--patch", string(patch),
	)
}

func EnablePrometheusAdminAPIPatch() []json.RawMessage {
	return []json.RawMessage{
		[]byte(`{"op": "add", "path": "/spec/template/spec/containers/1/args/-", "value": "--storage.tsdb.no-lockfile"}`),
		[]byte(`{"op": "add", "path": "/spec/template/spec/containers/1/args/-", "value": "--web.enable-admin-api"}`),
		[]byte(`{"op": "remove", "path": "/spec/template/metadata/labels/kuma.io~1sidecar-injection"}`),
		[]byte(`{"op": "replace", "path": "/spec/strategy/rollingUpdate"}`),
		[]byte(`{"op": "replace", "path": "/spec/strategy/type", "value": "Recreate"}`),
	}
}

func SetPrometheusResourcesPatch() []json.RawMessage {
	return []json.RawMessage{
		[]byte(`{"op":"add","path":"/spec/template/spec/containers/1/resources/requests","value":{"cpu":"1","memory":"1Gi"}}`),
	}
}

func SetObservabilityTolerations() []json.RawMessage {
	tolerations, err := json.Marshal([]corev1.Toleration{toleration})
	if err != nil {
		tolerations = []byte("[]")
		ginkgo.GinkgoWriter.Println("Unable to marshal observability tolerations", err.Error())
	}

	return []json.RawMessage{
		[]byte(fmt.Sprintf(`{"op": "add", "path": "/spec/template/spec/tolerations", "value": %s}`, tolerations)),
		[]byte(fmt.Sprintf(`{"op": "add", "path": "/spec/template/spec/nodeSelector", "value": {"NodeGroup": %q}}`, nameNodeGroupObservability)),
	}
}

func SetPrometheusPVC80GiPatch() []json.RawMessage {
	return []json.RawMessage{
		[]byte(fmt.Sprintf(`{"op": "replace", "path": "/spec/template/spec/volumes/1/persistentVolumeClaim/claimName", "value": %q}`, namePVC80GiPrometheus)),
	}
}

// SavePrometheusSnapshot triggers tsdb snapshot and copies it from kube container to hostPath
func SavePrometheusSnapshot(cluster framework.Cluster, namespace string, hostPath string) error {
	// get pod name
	pods, err := k8s.ListPodsE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(namespace),
		metav1.ListOptions{
			LabelSelector: "component=server",
		},
	)
	if err != nil {
		return err
	}
	if len(pods) != 1 {
		return fmt.Errorf("expected %d pods, got %d", 1, len(pods))
	}
	podName := pods[0].Name

	// save snapshot
	out, err := k8s.RunKubectlAndGetOutputE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(namespace),
		"exec", podName, "-c", NamePrometheusServer, "--", "sh", "-c",
		`wget -qO- --post-data='{}' http://localhost:9090/api/v1/admin/tsdb/snapshot`,
	)
	if err != nil {
		return err
	}
	var resp promSnapshotResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return err
	}
	if resp.Status != "success" {
		return fmt.Errorf("invalid status %s", resp.Status)
	}

	// extract snapshot
	src := namespace + "/" + podName + ":" + "/data/snapshots/" + resp.Data.Name
	dest := hostPath + "/" + resp.Data.Name
	err = k8s.RunKubectlE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(),
		"cp", src, dest, "-c", NamePrometheusServer, "--retries", "10",
	)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dest); err != nil {
		return errors.New("file was not copied")
	}
	return nil
}

type promSnapshotResponse struct {
	Status string `json:"status"`
	Data   struct {
		Name string `json:"name"`
	} `json:"data"`
}

func PortForwardPrometheusServer(cluster *framework.K8sCluster, ns string) error {
	return cluster.PortForwardService(NamePrometheusServer, ns, 9090)
}

type PromClient struct {
	queryClient v1.API
}

func NewPromClient(url string) (*PromClient, error) {
	client, err := api.NewClient(api.Config{
		Address: url,
	})

	if err != nil {
		return nil, err
	}

	return &PromClient{
		queryClient: v1.NewAPI(client),
	}, nil
}

func (p *PromClient) QueryIntValue(query string) (int, error) {
	result, _, err := p.queryClient.Query(context.Background(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return 0, errors.New("unexpected query result type")
	}

	if len(vector) == 0 {
		return 0, fmt.Errorf("no results found for the query: %s", query)
	}

	return int(vector[0].Value), nil
}
