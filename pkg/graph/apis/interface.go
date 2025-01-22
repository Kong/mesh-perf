package apis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Service struct {
	Idx      int   `yaml:"idx" json:"idx"`
	Edges    []int `yaml:"edges" json:"edges"`
	Replicas int   `yaml:"replicas" json:"replicas"`
}

type ServiceGraph struct {
	Services         []Service `yaml:"services" json:"services"`
	GenerationParams string    `yaml:"generationParams" json:"generationParams"`
}

func (g ServiceGraph) Validate() error {
	// Check first that all indexes correspond to array idx
	for i, srv := range g.Services {
		if i != srv.Idx {
			return fmt.Errorf("service's Idx:%d doesn't refer to its position in the service array: %d", i, srv.Idx)
		}
		for _, edge := range srv.Edges {
			if edge >= len(g.Services) || edge < 0 {
				return fmt.Errorf("service's Idx:%d has edge '%d' that is not an actual service", i, edge)
			}
		}
	}
	// Check for cycles
	permanentMark := map[int]struct{}{}
	temporaryMark := map[int]struct{}{}
	var visit func(n int) error
	visit = func(n int) error {
		if _, exists := permanentMark[n]; exists {
			return nil
		}
		if _, exists := temporaryMark[n]; exists {
			return errors.New("cycle detected")
		}
		temporaryMark[n] = struct{}{}

		for _, edge := range g.Services[n].Edges {
			if err := visit(edge); err != nil {
				return err
			}
		}
		delete(temporaryMark, n)
		permanentMark[n] = struct{}{}
		return nil
	}
	for i := range g.Services {
		if _, exists := permanentMark[i]; exists {
			continue
		}
		if err := visit(i); err != nil {
			return err
		}
	}
	return nil
}

// Generator generates the graph is a custom format
type Generator interface {
	Apply(writer io.Writer, svc ServiceGraph) error
}

// GeneratorFunc a shorthand for a regular generator
type GeneratorFunc func(writer io.Writer, svc ServiceGraph) error

func (f GeneratorFunc) Apply(writer io.Writer, svc ServiceGraph) error {
	return f(writer, svc)
}

// DotGenerator outputs the service graph in dot format.
var DotGenerator = GeneratorFunc(func(writer io.Writer, s ServiceGraph) error {
	var allEdges []string
	for _, srv := range s.Services {
		for _, other := range srv.Edges {
			allEdges = append(allEdges, fmt.Sprintf("%d -> %d;", srv.Idx, other))
		}
	}
	_, err := fmt.Fprintf(writer, "digraph{\n%s\n}\n", strings.Join(allEdges, "\n"))
	return err
})

// MermaidGenerator outputs the service graph in mermaid format (this is rendered in github flavoured Markdown).
var MermaidGenerator = GeneratorFunc(func(writer io.Writer, s ServiceGraph) error {
	var allEdges []string
	for _, srv := range s.Services {
		allEdges = append(allEdges, fmt.Sprintf("\t%d(%d replicas:%d);", srv.Idx, srv.Idx, srv.Replicas))
		for _, other := range srv.Edges {
			allEdges = append(allEdges, fmt.Sprintf("\t%d --> %d;", srv.Idx, other))
		}
	}
	_, err := fmt.Fprintf(writer, "graph TD;\n%s\n\n", strings.Join(allEdges, "\n"))
	return err
})

// JsonGenerator outputs the service graph in json
var JsonGenerator = GeneratorFunc(func(writer io.Writer, svc ServiceGraph) error {
	return json.NewEncoder(writer).Encode(svc)
})
