package acp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/sdk/module"
	faceport "agent-vivy/sdk/port/face"
)

// fakeHost is the minimal face.Host stand-in: it records Control calls,
// captures the event handler, and lets tests stub responses.
type fakeHost struct {
	mu      sync.Mutex
	calls   []fakeCall
	handler func(string, json.RawMessage)
	callFn  func(ctx context.Context, method string, params any) (json.RawMessage, error)
}

type fakeCall struct {
	method string
	params any
}

func (f *fakeHost) ModuleID() string { return "vivy/face-host" }

func (f *fakeHost) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	f.mu.Lock()
	f.calls = append(f.calls, fakeCall{method: method, params: params})
	f.mu.Unlock()
	if f.callFn != nil {
		return f.callFn(ctx, method, params)
	}
	return nil, errors.New("fakehost: control plane not stubbed")
}

func (f *fakeHost) OnEvent(h func(string, json.RawMessage)) {
	f.mu.Lock()
	f.handler = h
	f.mu.Unlock()
}

func (f *fakeHost) emit(method string, params json.RawMessage) {
	f.mu.Lock()
	h := f.handler
	f.mu.Unlock()
	if h != nil {
		h(method, params)
	}
}

func (f *fakeHost) callCount(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c.method == method {
			n++
		}
	}
	return n
}

func (f *fakeHost) callParams(t *testing.T, method string, out any) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c.method == method {
			b, err := json.Marshal(c.params)
			if err != nil {
				t.Fatalf("marshal %s params: %v", method, err)
			}
			if err := json.Unmarshal(b, out); err != nil {
				t.Fatalf("decode %s params: %v", method, err)
			}
			return
		}
	}
	t.Fatalf("no %s call recorded", method)
}

func TestModuleIsPure(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New returned nil")
	}
	d := m.Descriptor()
	if d.APIVersion != module.APIVersionV1 {
		t.Fatalf("apiVersion = %q", d.APIVersion)
	}
	if d.Module.ID != "projectvivy/acp" || d.Module.Version == "" {
		t.Fatalf("module identity = %+v", d.Module)
	}
	if len(d.Provides) != 1 || d.Provides[0].Port != "std/face@v1" || d.Provides[0].ID != "projectvivy.acp" {
		t.Fatalf("provides = %+v", d.Provides)
	}
	if len(d.Requires) != 1 || d.Requires[0].Port != "core/face-host@v1" || d.Requires[0].Provider != "vivy/face-host" {
		t.Fatalf("requires = %+v", d.Requires)
	}
	if len(d.RequestedGrants) != 1 || d.RequestedGrants[0] != module.GrantRPCClient {
		t.Fatalf("requestedGrants = %+v", d.RequestedGrants)
	}
	if d.Lifecycle.Scope != module.ScopeGeneration {
		t.Fatalf("scope = %q", d.Lifecycle.Scope)
	}
	if d.Source.Ref != "repo:plugins/acp" || len(d.Source.SHA256) != 64 {
		t.Fatalf("source = %+v", d.Source)
	}

	// Provider construction is pure: definitions and per-Host instances
	// carry no I/O and no shared state between instances.
	p := NewProvider()
	def := p.Definition()
	if def.ID != "projectvivy.acp" || def.Kind != "acp" {
		t.Fatalf("definition = %+v", def)
	}
	a, err := p.Construct(context.Background(), &fakeHost{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	b, err := p.Construct(context.Background(), &fakeHost{})
	if err != nil {
		t.Fatalf("construct second: %v", err)
	}
	if a == b {
		t.Fatal("Construct returned the same instance for distinct calls")
	}
}

func TestProviderRequiresStreams(t *testing.T) {
	p := NewProvider()
	if _, err := p.Construct(context.Background(), nil); err == nil {
		t.Fatal("Construct with nil Host accepted")
	}
	f, err := p.Construct(context.Background(), &fakeHost{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	discardIn := strings.NewReader("")
	var out strings.Builder
	cases := []struct {
		name string
		opts faceport.Options
	}{
		{"nil input", faceport.Options{In: nil, Out: &out, Err: &out}},
		{"nil output", faceport.Options{In: discardIn, Out: nil, Err: &out}},
		{"nil error", faceport.Options{In: discardIn, Out: &out, Err: nil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := f.Run(ctx, tc.opts); err == nil {
				t.Fatal("Run accepted a nil stream")
			}
		})
	}
}
