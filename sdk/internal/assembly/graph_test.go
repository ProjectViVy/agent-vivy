package assembly

import (
	"reflect"
	"strings"
	"testing"
)

func TestGraphProducesStableTopologicalOrder(t *testing.T) {
	graph := newDependencyGraph([]string{"fixture/c", "fixture/a", "fixture/b"})
	graph.addEdge("fixture/a", "fixture/c")
	graph.addEdge("fixture/b", "fixture/c")

	got, err := graph.order()
	if err != nil {
		t.Fatalf("order() error = %v", err)
	}
	want := []string{"fixture/a", "fixture/b", "fixture/c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order() = %v, want %v", got, want)
	}
}

func TestGraphReportsDeterministicCycle(t *testing.T) {
	graph := newDependencyGraph([]string{"fixture/b", "fixture/a"})
	graph.addEdge("fixture/b", "fixture/a")
	graph.addEdge("fixture/a", "fixture/b")

	_, err := graph.order()
	if err == nil || !strings.Contains(err.Error(), "dependency cycle: fixture/a -> fixture/b -> fixture/a") {
		t.Fatalf("order() error = %v, want canonical cycle", err)
	}
}
