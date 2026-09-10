package toolhost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-vivy/sdk/port/pretool"
)

type pretoolFunc struct {
	id string
	fn func(context.Context, pretool.Request) (pretool.Decision, error)
}

func (provider pretoolFunc) ID() string { return provider.id }
func (provider pretoolFunc) Evaluate(ctx context.Context, request pretool.Request) (pretool.Decision, error) {
	return provider.fn(ctx, request)
}

func TestRewriteRevalidatesSchemaPolicyAndGrant(t *testing.T) {
	host, err := New(Config{Middleware: []pretool.Provider{
		pretoolFunc{id: "acme/rewrite", fn: func(context.Context, pretool.Request) (pretool.Decision, error) {
			return pretool.Decision{Kind: pretool.RewriteArgs, Arguments: json.RawMessage(`{"value":"rewritten"}`)}, nil
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err = host.ApplyMiddleware(context.Background(), Request{ID: "acme.echo", Args: json.RawMessage(`{"value":"original"}`)}, func(_ context.Context, request Request) error {
		calls++
		if string(request.Args) != `{"value":"rewritten"}` {
			t.Fatalf("revalidator args = %s", request.Args)
		}
		return errors.New("grant recheck denied")
	})
	if err == nil || !strings.Contains(err.Error(), "grant recheck denied") {
		t.Fatalf("ApplyMiddleware error = %v, want revalidation failure", err)
	}
	if calls != 1 {
		t.Fatalf("revalidation calls = %d, want 1", calls)
	}
}

func TestMiddlewareCannotRewriteToolIdentity(t *testing.T) {
	host, err := New(Config{Middleware: []pretool.Provider{
		pretoolFunc{id: "acme/mutate-request", fn: func(_ context.Context, request pretool.Request) (pretool.Decision, error) {
			request.ToolID = "evil.tool"
			request.Arguments[0] = 'X'
			return pretool.Decision{Kind: pretool.Pass}, nil
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.ApplyMiddleware(context.Background(), Request{ID: "safe.tool", Args: json.RawMessage(`{"x":1}`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ToolID != "safe.tool" || string(result.Arguments) != `{"x":1}` {
		t.Fatalf("middleware mutated request identity or args: %#v", result)
	}
}

func TestMiddlewareTimeoutFailsClosed(t *testing.T) {
	host, err := New(Config{
		MiddlewareTimeout: time.Millisecond,
		Middleware: []pretool.Provider{
			pretoolFunc{id: "acme/slow", fn: func(ctx context.Context, _ pretool.Request) (pretool.Decision, error) {
				<-ctx.Done()
				return pretool.Decision{}, ctx.Err()
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.ApplyMiddleware(context.Background(), Request{ID: "safe.tool", Args: json.RawMessage(`{}`)}, nil)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrMiddlewareFailed) {
		t.Fatalf("timeout error = %v, want deadline + ErrMiddlewareFailed", err)
	}
}

func TestMiddlewarePanicAndInvalidDecisionFailClosed(t *testing.T) {
	for _, provider := range []pretool.Provider{
		pretoolFunc{id: "acme/panic", fn: func(context.Context, pretool.Request) (pretool.Decision, error) {
			panic("boom")
		}},
		pretoolFunc{id: "acme/invalid", fn: func(context.Context, pretool.Request) (pretool.Decision, error) {
			return pretool.Decision{Kind: pretool.Kind("execute")}, nil
		}},
	} {
		host, err := New(Config{Middleware: []pretool.Provider{provider}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := host.ApplyMiddleware(context.Background(), Request{ID: "safe.tool", Args: json.RawMessage(`{}`)}, nil); !errors.Is(err, ErrMiddlewareFailed) {
			t.Fatalf("provider %s error = %v, want ErrMiddlewareFailed", provider.ID(), err)
		}
	}
}

func TestMiddlewareAppliesMultipleRewritesInRecipeOrder(t *testing.T) {
	seen := []string{}
	host, err := New(Config{Middleware: []pretool.Provider{
		pretoolFunc{id: "first", fn: func(_ context.Context, request pretool.Request) (pretool.Decision, error) {
			seen = append(seen, string(request.Arguments))
			return pretool.Decision{Kind: pretool.RewriteArgs, Arguments: json.RawMessage(`{"step":1}`)}, nil
		}},
		pretoolFunc{id: "second", fn: func(_ context.Context, request pretool.Request) (pretool.Decision, error) {
			seen = append(seen, string(request.Arguments))
			return pretool.Decision{Kind: pretool.RewriteArgs, Arguments: json.RawMessage(`{"step":2}`)}, nil
		}},
		pretoolFunc{id: "approval", fn: func(_ context.Context, request pretool.Request) (pretool.Decision, error) {
			seen = append(seen, string(request.Arguments))
			return pretool.Decision{Kind: pretool.RequireApproval, ApprovalClass: "external-effect"}, nil
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	revalidations := 0
	result, err := host.ApplyMiddleware(context.Background(), Request{ID: "safe.tool", Args: json.RawMessage(`{"step":0}`)}, func(context.Context, Request) error {
		revalidations++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(seen, "|"), `{"step":0}|{"step":1}|{"step":2}`; got != want {
		t.Fatalf("middleware order = %s, want %s", got, want)
	}
	if string(result.Arguments) != `{"step":2}` || len(result.ApprovalClasses) != 1 || result.ApprovalClasses[0] != "external-effect" {
		t.Fatalf("middleware result = %#v", result)
	}
	if revalidations != 2 {
		t.Fatalf("revalidations = %d, want 2", revalidations)
	}
}
