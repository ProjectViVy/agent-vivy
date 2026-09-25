package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/compose"

	"agent-vivy/internal/testsupport"
)

type orchestrationProofInput struct {
	A nativeOrchestrationRequest
	B nativeOrchestrationRequest
}

type orchestrationProofJoinInput struct {
	A nativeOrchestrationResult
	B nativeOrchestrationResult
}

func TestOrchestrationNative(t *testing.T) {
	// Given: a real Service fixture is the lifecycle owner of two Workflow nodes.
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	workflow := compose.NewWorkflow[orchestrationProofInput, string]()
	workflow.AddLambdaNode("a", compose.InvokableLambda(svc.runNativeOrchestration)).
		AddInput(compose.START, compose.FromField("A"))
	workflow.AddLambdaNode("b", compose.InvokableLambda(svc.runNativeOrchestration)).
		AddInput(compose.START, compose.FromField("B"))
	workflow.AddLambdaNode("join", compose.InvokableLambda(func(_ context.Context, input orchestrationProofJoinInput) (string, error) {
		return input.A.Output + "+" + input.B.Output, nil
	})).
		AddInput("a", compose.ToField("A")).
		AddInput("b", compose.ToField("B"))
	workflow.End().AddInput("join")
	runnable, err := workflow.Compile(context.Background())
	if err != nil {
		t.Fatalf("compile native orchestration workflow: %v", err)
	}

	// When: Eino invokes both lifecycle nodes through the Service boundary.
	got, err := runnable.Invoke(context.Background(), orchestrationProofInput{
		A: nativeOrchestrationRequest{Task: "alpha"},
		B: nativeOrchestrationRequest{Task: "beta"},
	})

	// Then: the desired native lifecycle eventually returns both node outputs.
	if err != nil {
		if !errors.Is(err, ErrNativeOrchestrationUnimplemented) {
			t.Fatalf("invoke native orchestration workflow returned unexpected error: %v", err)
		}
		t.Fatalf("invoke native orchestration workflow: %v", err)
	}
	if got != "alpha+beta" {
		t.Fatalf("native orchestration result = %q, want %q", got, "alpha+beta")
	}
}
