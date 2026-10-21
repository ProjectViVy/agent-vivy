package mcphost

import "testing"

func TestProjectedToolIDSeparatesInstanceAndRemoteSegments(t *testing.T) {
	pairs := [][2]string{{"a.b", "c"}, {"a", "b.c"}, {"a_b", "c"}, {"a", "b_c"}}
	seen := map[string][2]string{}
	for _, pair := range pairs {
		id := projectedToolID(pair[0], pair[1])
		if previous, ok := seen[id]; ok {
			t.Fatalf("distinct MCP tuples %q and %q collide at %q", previous, pair, id)
		}
		if id != projectedToolID(pair[0], pair[1]) {
			t.Fatal("MCP name is not stable")
		}
		seen[id] = pair
	}
}
