package toolhost

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	porttool "agent-vivy/sdk/port/tool"
)

type testModuleHost struct{ id string }

func (h testModuleHost) ModuleID() string { return h.id }
func (h testModuleHost) InvokeTool(context.Context, string, json.RawMessage) (string, error) {
	return "", errors.New("unexpected nested tool invocation")
}

type testToolProvider struct {
	def    porttool.Definition
	result string
	seen   *string
}

func (p testToolProvider) Definition() porttool.Definition { return p.def }
func (p testToolProvider) Invoke(_ context.Context, host porttool.Host, _ json.RawMessage) (porttool.Result, error) {
	if p.seen != nil {
		*p.seen = host.ModuleID()
	}
	return porttool.Result{Text: p.result}, nil
}

func TestHostRejectsDuplicateStaticToolIDs(t *testing.T) {
	provider := testToolProvider{def: porttool.Definition{ID: "acme.echo"}}
	_, err := New(Config{Static: []StaticBinding{
		{OwnerID: "acme.one", Provider: provider, Host: testModuleHost{id: "acme.one"}},
		{OwnerID: "acme.two", Provider: provider, Host: testModuleHost{id: "acme.two"}},
	}})
	if !errors.Is(err, ErrDuplicateToolID) {
		t.Fatalf("New duplicate error = %v, want ErrDuplicateToolID", err)
	}
}

func TestHostInvokesStaticProviderWithOwningHost(t *testing.T) {
	seen := ""
	host, err := New(Config{Static: []StaticBinding{{
		OwnerID:  "acme.echo-provider",
		Provider: testToolProvider{def: porttool.Definition{ID: "acme.echo"}, result: "ok", seen: &seen},
		Host:     testModuleHost{id: "acme.echo-provider"},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	result, err := host.Invoke(context.Background(), Request{ID: "acme.echo", Args: json.RawMessage(`{"value":"hello"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "ok" {
		t.Fatalf("result = %q, want ok", result.Text)
	}
	if seen != "acme.echo-provider" {
		t.Fatalf("provider host = %q, want owner module host", seen)
	}
}

func TestHostListsStaticToolsDeterministically(t *testing.T) {
	host, err := New(Config{Static: []StaticBinding{
		{OwnerID: "z", Provider: testToolProvider{def: porttool.Definition{ID: "z.last"}}, Host: testModuleHost{id: "z"}},
		{OwnerID: "a", Provider: testToolProvider{def: porttool.Definition{ID: "a.first"}}, Host: testModuleHost{id: "a"}},
	}})
	if err != nil {
		t.Fatal(err)
	}

	defs := host.ListVisible()
	if len(defs) != 2 || defs[0].ID != "a.first" || defs[1].ID != "z.last" {
		t.Fatalf("ListVisible = %#v, want stable ID order", defs)
	}
}
