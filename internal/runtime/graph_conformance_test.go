package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/compose"

	"agent-vivy/internal/tools"
)

// GraphTool is deliberately represented only by this conformance fixture. It
// proves nested Eino workflow behavior without adding a GraphTool to Vivy's
// production registry or public tool catalog.
func TestGraphToolConformanceNestedWorkflow(t *testing.T) {
	subgraph := compose.NewGraph[string, string]()
	if err := subgraph.AddLambdaNode("inner", compose.InvokableLambda(func(_ context.Context, input string) (string, error) {
		return input + ":inner", nil
	})); err != nil {
		t.Fatalf("add inner node: %v", err)
	}
	if err := subgraph.AddEdge(compose.START, "inner"); err != nil {
		t.Fatalf("inner start: %v", err)
	}
	if err := subgraph.AddEdge("inner", compose.END); err != nil {
		t.Fatalf("inner end: %v", err)
	}

	root := compose.NewGraph[string, string]()
	if err := root.AddGraphNode("nested", subgraph); err != nil {
		t.Fatalf("add nested graph: %v", err)
	}
	if err := root.AddLambdaNode("outer", compose.InvokableLambda(func(_ context.Context, input string) (string, error) {
		return input + ":outer", nil
	})); err != nil {
		t.Fatalf("add outer node: %v", err)
	}
	if err := root.AddEdge(compose.START, "nested"); err != nil {
		t.Fatalf("root start: %v", err)
	}
	if err := root.AddEdge("nested", "outer"); err != nil {
		t.Fatalf("nested to outer: %v", err)
	}
	if err := root.AddEdge("outer", compose.END); err != nil {
		t.Fatalf("root end: %v", err)
	}
	runnable, err := root.Compile(context.Background())
	if err != nil {
		t.Fatalf("compile nested workflow: %v", err)
	}
	got, err := runnable.Invoke(context.Background(), "start")
	if err != nil || got != "start:inner:outer" {
		t.Fatalf("nested result = %q/%v", got, err)
	}
}

func TestGraphToolConformanceCancellationAndProductionBoundary(t *testing.T) {
	graph := compose.NewGraph[string, string]()
	if err := graph.AddLambdaNode("cancel", compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})); err != nil {
		t.Fatalf("add cancellation node: %v", err)
	}
	_ = graph.AddEdge(compose.START, "cancel")
	_ = graph.AddEdge("cancel", compose.END)
	runnable, err := graph.Compile(context.Background())
	if err != nil {
		t.Fatalf("compile cancellation graph: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = runnable.Invoke(ctx, "cancel")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}

	registry := tools.Builtin(nil)
	for _, name := range []string{"graph_tool", "browseruse", "browser_use"} {
		if _, err := registry.Resolve([]string{name}); err == nil {
			t.Fatalf("excluded production tool %q is registered", name)
		}
	}
}
