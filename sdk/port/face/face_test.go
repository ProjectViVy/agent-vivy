package face

import (
	"context"
	"io"
	"strings"
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

func TestFaceOptionsInputIsOptional(t *testing.T) {
	// The restricted ACP Face consumes invocation stdin through Options.In.
	// The field must be io.Reader-typed and optional: every existing Host
	// implementation constructs Options without it.
	var opts Options
	if opts.In != nil {
		t.Fatal("zero-value Options must carry nil In")
	}
	var in io.Reader = strings.NewReader(`{"jsonrpc":"2.0"}`)
	opts.In = in
	if opts.In != in {
		t.Fatal("Options.In did not retain the io.Reader value")
	}
}
