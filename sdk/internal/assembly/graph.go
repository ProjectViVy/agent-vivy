package assembly

import (
	"fmt"
	"sort"
	"strings"
)

type dependencyGraph struct {
	nodes map[string]struct{}
	edges map[string]map[string]struct{}
}

func newDependencyGraph(nodes []string) *dependencyGraph {
	graph := &dependencyGraph{
		nodes: make(map[string]struct{}, len(nodes)),
		edges: make(map[string]map[string]struct{}, len(nodes)),
	}
	for _, node := range nodes {
		graph.nodes[node] = struct{}{}
	}
	return graph
}

func (graph *dependencyGraph) addEdge(before, after string) {
	if graph.edges[before] == nil {
		graph.edges[before] = make(map[string]struct{})
	}
	graph.edges[before][after] = struct{}{}
}

func (graph *dependencyGraph) order() ([]string, error) {
	indegree := make(map[string]int, len(graph.nodes))
	for node := range graph.nodes {
		indegree[node] = 0
	}
	for _, successors := range graph.edges {
		for successor := range successors {
			indegree[successor]++
		}
	}

	ready := make([]string, 0, len(indegree))
	for node, degree := range indegree {
		if degree == 0 {
			ready = append(ready, node)
		}
	}
	sort.Strings(ready)
	ordered := make([]string, 0, len(indegree))
	for len(ready) > 0 {
		node := ready[0]
		ready = ready[1:]
		ordered = append(ordered, node)
		for _, successor := range sortedSet(graph.edges[node]) {
			indegree[successor]--
			if indegree[successor] == 0 {
				ready = append(ready, successor)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) == len(indegree) {
		return ordered, nil
	}
	return nil, graph.cycleError()
}

func (graph *dependencyGraph) cycleError() error {
	state := make(map[string]uint8, len(graph.nodes))
	stack := make([]string, 0, len(graph.nodes))
	position := make(map[string]int, len(graph.nodes))
	var cycle []string

	var visit func(string) bool
	visit = func(node string) bool {
		state[node] = 1
		position[node] = len(stack)
		stack = append(stack, node)
		for _, successor := range sortedSet(graph.edges[node]) {
			switch state[successor] {
			case 0:
				if visit(successor) {
					return true
				}
			case 1:
				cycle = append([]string(nil), stack[position[successor]:]...)
				cycle = append(cycle, successor)
				return true
			}
		}
		stack = stack[:len(stack)-1]
		delete(position, node)
		state[node] = 2
		return false
	}

	for _, node := range sortedSet(graph.nodes) {
		if state[node] == 0 && visit(node) {
			return fmt.Errorf("dependency cycle: %s", strings.Join(cycle, " -> "))
		}
	}
	return fmt.Errorf("dependency cycle")
}

func sortedSet[T ~string](set map[T]struct{}) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, string(value))
	}
	sort.Strings(values)
	return values
}
