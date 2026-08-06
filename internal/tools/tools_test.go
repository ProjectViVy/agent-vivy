package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEchoInfoSpec(t *testing.T) {
	spec := NewEchoInfo().Spec()
	if spec.Name != EchoInfoName {
		t.Errorf("name = %q", spec.Name)
	}
	if !spec.Readonly {
		t.Error("echo_info must be readonly to auto-execute (D-012)")
	}
}

func TestEchoInfoRun(t *testing.T) {
	out, err := NewEchoInfo().InvokableRun(context.Background(),
		json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if out != "hello" {
		t.Errorf("out = %q, want verbatim echo", out)
	}
}

func TestEchoInfoRejectsBadArgs(t *testing.T) {
	cases := map[string]string{
		"missing text":   `{}`,
		"empty text":     `{"text":""}`,
		"unknown field":  `{"text":"x","extra":1}`,
		"not an object":  `"plain string"`,
		"malformed json": `{`,
		"oversized text": `{"text":"` + strings.Repeat("a", echoTextLimit+1) + `"}`,
	}
	tool := NewEchoInfo()
	for name, args := range cases {
		_, err := tool.InvokableRun(context.Background(), json.RawMessage(args))
		if err == nil {
			t.Errorf("%s: want error, got nil", name)
			continue
		}
		var argErr *ArgError
		if !errors.As(err, &argErr) {
			t.Errorf("%s: error %T is not a structured *ArgError", name, err)
		}
	}
}

func TestRegistryResolve(t *testing.T) {
	reg := Builtin()

	got, err := reg.Resolve([]string{EchoInfoName})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got) != 1 || got[0].Spec().Name != EchoInfoName {
		t.Errorf("Resolve = %+v", got)
	}

	if _, err := reg.Resolve([]string{"write_note"}); err == nil {
		t.Error("unknown tool must be a startup error")
	}

	if got, err := reg.Resolve(nil); err != nil || len(got) != 0 {
		t.Errorf("Resolve(nil) = %v, %v", got, err)
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("duplicate registration must panic")
		}
	}()
	NewRegistry(NewEchoInfo(), NewEchoInfo())
}
