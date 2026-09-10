package face

import (
	"context"
	"testing"
)

type testInstance struct{}

func (testInstance) Run(context.Context, Options) (Result, error) {
	return Result{Status: "completed"}, nil
}

func TestFaceInstancePreservesInvocationOptions(t *testing.T) {
	var instance Instance = testInstance{}
	result, err := instance.Run(context.Background(), Options{Prompt: "hi"})
	if err != nil || result.Status != "completed" {
		t.Fatalf("Run = %+v, %v", result, err)
	}
}
