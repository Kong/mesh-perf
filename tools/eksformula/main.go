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

	extras := 9        // additional pods required to run test that don't depend on number of nodes (Kuma CP, Prometheus, Grafana, etc.)
	extrasPerNode := 3 // additional system pods that running on each node (some AWS EKS specific pods)
	podsPerNode := 58  // we're using 't4g.2xlarge', it can run 58 pods, see full list here https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt

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
