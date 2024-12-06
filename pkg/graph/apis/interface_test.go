package apis_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kong/mesh-perf/pkg/graph/apis"
)

func TestValidate(t *testing.T) {
	type testCase struct {
		desc  string
		given apis.ServiceGraph
		then  error
	}
	tests := []testCase{
		{
			desc: "Empty graph",
			given: apis.ServiceGraph{
				Services: []apis.Service{},
			},
			then: nil,
		},
		{
			desc: "Line graph",
			given: apis.ServiceGraph{
				Services: []apis.Service{
					{Idx: 0, Edges: []int{1}, Replicas: 2},
					{Idx: 1, Edges: []int{2}, Replicas: 2},
					{Idx: 2, Edges: []int{}, Replicas: 2},
				},
			},
			then: nil,
		},
		{
			desc: "Loop graph",
			given: apis.ServiceGraph{
				Services: []apis.Service{
					{Idx: 0, Edges: []int{1}, Replicas: 2},
					{Idx: 1, Edges: []int{2}, Replicas: 2},
					{Idx: 2, Edges: []int{0}, Replicas: 2},
				},
			},
			then: errors.New("cycle detected"),
		},
		{
			desc: "Complex loop",
			given: apis.ServiceGraph{
				Services: []apis.Service{
					{Idx: 0, Edges: []int{1}, Replicas: 2},
					{Idx: 1, Edges: []int{2}, Replicas: 2},
					{Idx: 2, Edges: []int{3, 4, 5}, Replicas: 2},
					{Idx: 3, Edges: []int{4}, Replicas: 2},
					{Idx: 4, Edges: []int{0}, Replicas: 2},
					{Idx: 5, Edges: []int{0}, Replicas: 2},
				},
			},
			then: errors.New("cycle detected"),
		},

		{
			desc: "Invalid index",
			given: apis.ServiceGraph{
				Services: []apis.Service{
					{Idx: 0, Edges: []int{1}, Replicas: 2},
				},
			},
			then: errors.New("service's Idx:0 has edge '1' that is not an actual service"),
		},
	}
	for _, tc := range tests {
		got := tc.given.Validate()
		if !reflect.DeepEqual(tc.then, got) {
			t.Fatalf("test: %s, expected: %v, got: %v", tc.desc, tc.then, got)
		}
	}
}
