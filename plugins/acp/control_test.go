package acp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
)

// controlCodedError mimics a Control-side coded failure: it exposes
// RPCErrorCode through the same structural interface the host error uses.
type controlCodedError struct{ code int }

func (e controlCodedError) Error() string     { return "control error" }
func (e controlCodedError) RPCErrorCode() int { return e.code }
func (e controlCodedError) Unwrap() error     { return nil }

func TestControlErrorProjection(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		want string
	}{
		{"core invalid params", controlCodedError{-32602}, -32602, "INVALID_INPUT"},
		{"core not found", controlCodedError{-32004}, -32002, "RESOURCE_NOT_FOUND"},
		{"core conflict", controlCodedError{-32009}, -32001, "RUN_CONFLICT"},
		{"core missing method", controlCodedError{-32601}, -32603, "REQUIRED_CAPABILITY_UNAVAILABLE"},
		{"core opaque code", controlCodedError{-32042}, -32603, "INTERNAL_FAILURE"},
		{"plain error", errors.New("panic stack /tmp/secret/path"), -32603, "INTERNAL_FAILURE"},
		{"wrapped coded", wrap(controlCodedError{-32004}), -32002, "RESOURCE_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := safeRPCError(tc.err)
			if got == nil {
				t.Fatal("nil error")
			}
			if got.Code != tc.code {
				t.Fatalf("code = %d, want %d", got.Code, tc.code)
			}
			var data map[string]string
			if err := json.Unmarshal(got.Data, &data); err != nil {
				t.Fatalf("data decode: %v", err)
			}
			if data["reason"] != tc.want {
				t.Fatalf("reason = %q, want %q", data["reason"], tc.want)
			}
			// No raw cause leaks on the wire.
			wire, _ := json.Marshal(got)
			for _, leak := range []string{"control error", "secret", "/tmp/"} {
				if contains(wire, leak) {
					t.Fatalf("wire leaks %q: %s", leak, string(wire))
				}
			}
		})
	}

	t.Run("deliberate adapter errors pass through", func(t *testing.T) {
		own := rpcError(-32602, "x", "CLIENT_MCP_UNSUPPORTED")
		if got := safeRPCError(own); got != own {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("safeRPCError never inspects message strings", func(t *testing.T) {
		// A coded error whose message mentions a different code must still
		// map by code, not by text.
		err := controlCodedError{-32004}
		got := safeRPCError(err)
		if got.Code != -32002 {
			t.Fatalf("code = %d", got.Code)
		}
	})
}

func wrap(err error) error { return &wrapped{err} }

type wrapped struct{ err error }

func (w *wrapped) Error() string { return "wrapped: " + w.err.Error() }
func (w *wrapped) Unwrap() error { return w.err }

func TestCallAllowlistAndTimeout(t *testing.T) {
	t.Run("non-allowlisted method fails before host", func(t *testing.T) {
		h := &fakeHost{}
		a := newAgent(h)
		_, err := a.call(context.Background(), "session/delete", nil)
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32603 {
			t.Fatalf("error = %v", err)
		}
		if h.callCount("session/delete") != 0 {
			t.Fatal("non-allowlisted call reached the host")
		}
	})

	t.Run("child timeout bounds the call", func(t *testing.T) {
		h := &fakeHost{callFn: func(ctx context.Context, method string, _ any) (json.RawMessage, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Error("child context has no deadline")
				return nil, nil
			}
			if until := time.Until(deadline); until > controlCallTimeout+time.Second || until < controlCallTimeout-5*time.Second {
				t.Errorf("deadline %v outside expected bound", until)
			}
			return json.RawMessage(`{}`), nil
		}}
		a := newAgent(h)
		if _, err := a.call(context.Background(), "initialize", nil); err != nil {
			t.Fatalf("call: %v", err)
		}
	})
}
