package fakeservice_test

import (
	"bytes"
	"testing"

	"github.com/kong/mesh-perf/pkg/graph/apis"
	"github.com/kong/mesh-perf/pkg/graph/generators/k8s"
	"github.com/kong/mesh-perf/pkg/graph/generators/k8s/fakeservice"
)

func TestSimple(t *testing.T) {
	opts := fakeservice.GeneratorOpts("")
	opts = append(opts, k8s.WithNamespace("foo"))
	encoder, err := k8s.NewGenerator(opts...)
	if err != nil {
		t.Error("failed", err)
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
