package framework

import (
	"encoding/json"
	"os"

	"github.com/go-errors/errors"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/kumahq/kuma/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EnablePrometheusAdminAPI(namespace string, cluster framework.Cluster) error {
	// because we install from install observability, we cannot set HELM values to enable admin API.
	// Additionally, we need to set "--storage.tsdb.no-lockfile", otherwise we cannot do rolling restart
	return k8s.RunKubectlE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(namespace),
		"patch", "deployment", "prometheus-server", "--type", "json",
		"-p", `[{"op": "add", "path": "/spec/template/spec/containers/1/args/-", "value": "--storage.tsdb.no-lockfile"},{"op": "add", "path": "/spec/template/spec/containers/1/args/-", "value": "--web.enable-admin-api"},{"op": "replace", "path": "/spec/strategy/rollingUpdate"},{"op": "replace", "path": "/spec/strategy/type", "value": "Recreate"}]`,
	)
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
		return errors.Errorf("expected %d pods, got %d", 1, len(pods))
	}
	podName := pods[0].Name

	// save snapshot
	out, err := k8s.RunKubectlAndGetOutputE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(namespace),
		"exec", podName, "-c", "prometheus-server", "--", "sh", "-c",
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
		return errors.Errorf("invalid status %s", resp.Status)
	}

	// extract snapshot
	src := namespace + "/" + podName + ":" + "/data/snapshots/" + resp.Data.Name
	dest := hostPath + "/" + resp.Data.Name
	err = k8s.RunKubectlE(
		cluster.GetTesting(),
		cluster.GetKubectlOptions(),
		"cp", src, dest, "-c", "prometheus-server",
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
