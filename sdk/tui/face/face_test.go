package face

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"agent-vivy/sdk/plugin"
)

type testEnv struct{ called bool }

func (e *testEnv) Call(context.Context, string, any) (json.RawMessage, error) {
	e.called = true
	return json.RawMessage(`{}`), nil
}

func (*testEnv) OnEvent(func(string, json.RawMessage)) {}

func TestKind(t *testing.T) {
	if got := New(plugin.FaceOptions{}).Kind(); got != Kind {
		t.Fatalf("kind = %q", got)
	}
}

func TestRejectsNonTerminalBeforeInitialize(t *testing.T) {
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	env := &testEnv{}
	_, err = New(plugin.FaceOptions{Out: out, Err: io.Discard}).Run(context.Background(), env)
	if err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("expected terminal error, got %v", err)
	}
	if env.called {
		t.Fatal("face initialized before validating its terminal")
	}
}
