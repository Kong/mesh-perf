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
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kumahq/kuma/v2/test/framework"
	"github.com/kumahq/kuma/v2/test/framework/portforward"

	"github.com/kong/mesh-perf/test/framework/silent_kubectl"
)

type PatchKind string

const (
	PortPrometheusServer           = 9090                // pod port (service exposes as 80)
	NamePrometheusServer           = "prometheus-server" // container name
	AppPrometheus                  = "prometheus"        // app label for port forwarding
	NameGrafana                    = "grafana"
	KindDeployment       PatchKind = "deployment"
	KindService          PatchKind = "service"
)

type Patcher func(kind PatchKind, name string, operations ...[]json.RawMessage) error

func NewPatcher(cluster framework.Cluster, namespace string, baseOperations ...[]json.RawMessage) Patcher {
	return func(kind PatchKind, name string, operations ...[]json.RawMessage) error {
		return ApplyJSONPatch(
			cluster,
			namespace,
			kind,
			name,
			slices.Concat(slices.Concat(baseOperations, operations)...),
		)
	}
}

func ApplyJSONPatch(cluster framework.Cluster, namespace string, kind PatchKind, name string, operations []json.RawMessage) error {
	patch, err := json.Marshal(operations)
	if err != nil {
		return err
	}

	return k8s.RunKubectlE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(namespace),
		"patch", string(kind), name, "--type", "json", "--patch", string(patch),
	)
}

func GrafanaServicePatch() []json.RawMessage {
	return []json.RawMessage{
		[]byte(`{"op": "replace", "path": "/spec/type", "value": "LoadBalancer"}`),
	}
}

func GrafanaDeploymentPatch() []json.RawMessage {
	return []json.RawMessage{
		[]byte(`{"op": "remove", "path": "/spec/template/metadata/labels/kuma.io~1sidecar-injection"}`),
	}
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

	if _, err := silent_kubectl.DoWithRetryE(
		cluster.GetTesting(),
		"Retrying copying Prometheus snapshot to "+src,
		10,
		8*time.Second,
		func() (string, error) {
			if err := k8s.RunKubectlE(
				cluster.GetTesting(),
				cluster.GetKubectlOptions(),
				"cp", src, dest, "-c", NamePrometheusServer, "--retries", "10",
			); err != nil {
				return "", err
			}
			return "Snapshot copied", nil
		},
	); err != nil {
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

type PromClient struct {
	queryClient v1.API
}

func NewPromClient(cluster *framework.K8sCluster, ns string) (*PromClient, error) {
	endpoint, err := GetPrometheusServerEndpoint(cluster, ns, PortPrometheusServer)
	if err != nil {
		return nil, err
	}

	client, err := api.NewClient(api.Config{Address: "http://" + endpoint})
	if err != nil {
		return nil, err
	}

	return &PromClient{queryClient: v1.NewAPI(client)}, nil
}

var ErrNoResults = errors.New("no results found for the query")

func (p *PromClient) QueryIntValue(ctx context.Context, query string) (int, error) {
	result, _, err := p.queryClient.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return 0, errors.New("unexpected query result type")
	}

	if len(vector) == 0 {
		return 0, ErrNoResults
	}

	return int(vector[0].Value), nil
}

// GetPrometheusServerEndpoint creates port forward to prometheus-server pod using component=server label
func GetPrometheusServerEndpoint(cluster *framework.K8sCluster, ns string, port int) (string, error) {
	// Find prometheus-server pod using component=server label
	pods, err := k8s.ListPodsE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(ns),
		metav1.ListOptions{
			LabelSelector: "component=server",
		},
	)
	if err != nil {
		return "", err
	}
	if len(pods) != 1 {
		return "", fmt.Errorf("expected 1 prometheus-server pod, got %d", len(pods))
	}
	podName := pods[0].Name

	spec := portforward.Spec{
		AppName:    podName,
		Namespace:  ns,
		RemotePort: port,
	}

	if cluster.GetPortForward(spec).Endpoint != "" {
		cluster.ClosePortForwards(spec)
	}

	return retry.DoWithRetryE(
		cluster.GetTesting(),
		"create port forward for prometheus-server",
		60,
		10*time.Second,
		func() (string, error) {
			fwd, err := cluster.PortForward(k8s.ResourceTypePod, podName, ns, port)
			if err != nil {
				return "", err
			}
			return fwd.Endpoint, nil
		},
	)
}

func GetApiServerEndpoint(cluster *framework.K8sCluster, ns, app string, port int) (string, error) {
	// The API Server or other control plane components may scale up after the cluster starts,
	// invalidating an existing port forward. To ensure a valid connection, always close any
	// existing port forward before creating a new one.
	spec := portforward.Spec{
		AppName:    app,
		Namespace:  ns,
		RemotePort: port,
	}

	if cluster.GetPortForward(spec).Endpoint != "" {
		cluster.ClosePortForwards(spec)
	}

	if _, err := cluster.PortForwardApp(spec); err != nil {
		return "", err
	}

	return retry.DoWithRetryE(
		cluster.GetTesting(),
		"create port forward for prometheus-pushgateway",
		60,
		10*time.Second,
		func() (string, error) {
			if _, err := cluster.PortForwardApp(spec); err != nil {
				return "", err
			}

			return cluster.GetPortForward(spec).Endpoint, nil
		},
	)
}
