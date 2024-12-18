package k8s_test

import (
	"bytes"
	"testing"

	"github.com/kong/mesh-perf/pkg/graph/apis"
	"github.com/kong/mesh-perf/pkg/graph/generators/k8s"
)

func TestSimple(t *testing.T) {
	encoder, err := k8s.NewGenerator(k8s.WithNamespace("foo"), k8s.WithImage("nginx"), k8s.WithPort(8080))
	if err != nil {
		t.Fatal("failed creating a simple generator", err)
	}
	buf := bytes.NewBuffer([]byte{})
	err = encoder.Apply(buf, apis.ServiceGraph{
		Services: []apis.Service{
			{Replicas: 2, Edges: []int{1, 2}, Idx: 0},
			{Replicas: 2, Edges: []int{2}, Idx: 1},
			{Replicas: 2, Edges: []int{3}, Idx: 2},
			{Replicas: 2, Edges: []int{}, Idx: 3},
		},
	})
	if err != nil {
		t.Error("failed", err)
	}
	println(buf.String())
}
