package divacognitive

import (
	"testing"

	"github.com/coder/hnsw"
)

// Native memory correction removes an indexed revision before replacing it.
// An empty graph must remain searchable during that transition.
func TestRecallIndexSearchAfterDeletingLastVector(t *testing.T) {
	graph := hnsw.NewGraph[string]()
	vector := []float32{1, 0}
	graph.Add(hnsw.MakeNode("fixture-revision", vector))
	if !graph.Delete("fixture-revision") {
		t.Fatal("fixture revision was not deleted")
	}
	if got := graph.Search(vector, 1); len(got) != 0 {
		t.Fatalf("deleted revision returned from search: %+v", got)
	}
}
