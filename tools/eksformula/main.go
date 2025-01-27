package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
)

func main() {
	services := requireIntVar("PERF_TEST_NUM_SERVICES")
	instancesPerService := requireIntVar("PERF_TEST_INSTANCES_PER_SERVICE")

	// Additional pods required to run the test that do not depend on the number of nodes.
	// These include control plane components like Kuma CP, Prometheus, Grafana, etc.
	extras := 9

	// Additional system pods running on each node, including AWS EKS-specific pods.
	// These pods include aws-node (75m CPU), ebs-csi-node (30m CPU), and kube-proxy (100m CPU).
	// Since we are currently limited by CPU and not by the number of pods per node,
	// these system pods require a total of 205m CPU per node. While technically
	// up to 5 more pods could fit on a node, this CPU constraint results in accounting
	// for 2 additional pods per node in our calculation.
	extrasPerNode := 2

	// We're using the 't4g.2xlarge' instance type, which can run up to 58 pods per node.
	// See the full list of instance types and their pod limits here:
	// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt
	// Each application pod along with its kuma-dp sidecar requests 150m CPU.
	// This means a maximum of 53 application pods can run on a single node.
	podsPerNode := 53

	fmt.Print(math.Ceil(float64(services*instancesPerService+extras) / float64(podsPerNode-extrasPerNode)))
}

func requireIntVar(key string) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("couldn't lookup value %s", key))
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		panic(err)
	}

	return i
}
