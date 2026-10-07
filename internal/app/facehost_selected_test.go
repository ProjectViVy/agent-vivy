package app

import (
	"context"
	"io"
	"os"
	"sync/atomic"
	"testing"

	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	plugin "agent-vivy/sdk/port/face"
)

// The selected generation carries exactly one sealed Face Provider in its
// RuntimeAssembly — that datum, not a caller-supplied organ, is what the
// selected launch path may run (ACP-STDIO-FACE §294).

type selectedFakeProvider struct {
	constructed atomic.Int32
	gotHost     plugin.Host
	instance    *selectedFakeInstance
}

func (p *selectedFakeProvider) Definition() plugin.Definition {
	return plugin.Definition{ID: "projectvivy/acp", Kind: "stdio"}
}

func (p *selectedFakeProvider) Construct(_ context.Context, host plugin.Host) (plugin.Instance, error) {
	p.constructed.Add(1)
	p.gotHost = host
	return p.instance, nil
}

type selectedFakeInstance struct {
	ran  atomic.Int32
	opts plugin.Options
}

func (i *selectedFakeInstance) Run(_ context.Context, opts plugin.Options) (plugin.Result, error) {
	i.opts = opts
	i.ran.Add(1)
	return plugin.Result{Status: "completed"}, nil
}

func TestSelectedFaceUsesOneApp(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "selected-face-key")

	instance := &selectedFakeInstance{}
	provider := &selectedFakeProvider{instance: instance}
	assembly := genassembly.BuildDefault()
	assembly.Face = provider
	assembly.Manifest.Face = "projectvivy/acp"

	a, err := NewWithAssembly(context.Background(), newDeepSeekTestConfig(t), assembly, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = inR.Close(); _ = inW.Close() }()

	result, err := runAssemblyFace(context.Background(), a, plugin.Options{
		In:  inR,
		Out: io.Discard,
		Err: io.Discard,
	})
	if err != nil {
		t.Fatalf("run selected face: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("result status = %q, want completed", result.Status)
	}
	if got := provider.constructed.Load(); got != 1 {
		t.Fatalf("provider constructed %d times, want exactly once", got)
	}
	if got := instance.ran.Load(); got != 1 {
		t.Fatalf("instance ran %d times, want exactly once", got)
	}
	if provider.gotHost == nil || provider.gotHost.ModuleID() != "vivy/face-host" {
		t.Fatalf("provider received wrong host: %#v", provider.gotHost)
	}
	if instance.opts.In != inR {
		t.Fatal("selected face did not receive the caller's stdin stream")
	}

	headless := genassembly.BuildDefault()
	if headless.Face != nil {
		t.Fatal("default generation unexpectedly carries a face")
	}
	a2, err := NewWithAssembly(context.Background(), newDeepSeekTestConfig(t), headless, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose headless app: %v", err)
	}
	t.Cleanup(func() { _ = a2.Close() })
	if _, err := runAssemblyFace(context.Background(), a2, plugin.Options{Out: io.Discard, Err: io.Discard}); err == nil {
		t.Fatal("headless generation accepted a selected-face run")
	}
}
