package modules

import (
	"context"
	"errors"
	"reflect"
	"testing"

	maskmodule "agent-vivy/internal/modules/masks"
)

type lifecycleProbe struct {
	id           string
	log          *[]string
	startErr     error
	readyErr     error
	stopErr      error
	closeErr     error
	checkContext bool
}

func (probe *lifecycleProbe) record(action string) {
	*probe.log = append(*probe.log, action+":"+probe.id)
}
func (probe *lifecycleProbe) Start(ctx context.Context) error {
	probe.record("start")
	if probe.checkContext {
		return ctx.Err()
	}
	return probe.startErr
}
func (probe *lifecycleProbe) Ready(context.Context) error {
	probe.record("ready")
	return probe.readyErr
}
func (probe *lifecycleProbe) Stop(context.Context) error { probe.record("stop"); return probe.stopErr }
func (probe *lifecycleProbe) Close(context.Context) error {
	probe.record("close")
	return probe.closeErr
}

func TestGenerationStartupFailureRollsBackInReverseAndPreservesCauses(t *testing.T) {
	var log []string
	readyCause := errors.New("ready failed")
	stopCause := errors.New("stop failed")
	owners := []*lifecycleProbe{
		{id: "one", log: &log},
		{id: "two", log: &log, readyErr: readyCause, stopErr: stopCause},
		{id: "three", log: &log},
	}
	_, err := Start(context.Background(), instances(owners...))
	if !errors.Is(err, readyCause) || !errors.Is(err, stopCause) {
		t.Fatalf("Start() error = %v; cause chain was not preserved", err)
	}
	want := []string{"start:one", "ready:one", "start:two", "ready:two", "stop:two", "stop:one", "close:three", "close:two", "close:one"}
	if !reflect.DeepEqual(log, want) {
		t.Fatalf("lifecycle = %v, want %v", log, want)
	}
}

func TestMaskGenerationStartupFailureRollsBackLabeledOwnerExactlyOnce(t *testing.T) {
	var log []string
	maskModule := maskmodule.NewModule()
	maskDescriptor := maskModule.Descriptor()
	maskOwner, err := maskModule.Construct(context.Background(), maskLifecycleHost{id: maskDescriptor.Module.ID})
	if err != nil {
		t.Fatal(err)
	}
	startCause := errors.New("mask owner start failed")
	owners := []Instance{
		&countedMaskInstance{id: maskDescriptor.Module.ID, inner: maskOwner, log: &log},
		&lifecycleProbe{id: "mask-failing", log: &log, startErr: startCause},
		&lifecycleProbe{id: "mask-after", log: &log},
	}
	_, err = Start(context.Background(), owners)
	if !errors.Is(err, startCause) {
		t.Fatalf("mask Start() error = %v, want %v", err, startCause)
	}
	want := []string{
		"start:" + maskDescriptor.Module.ID, "ready:" + maskDescriptor.Module.ID, "start:mask-failing",
		"stop:mask-failing", "stop:" + maskDescriptor.Module.ID,
		"close:mask-after", "close:mask-failing", "close:" + maskDescriptor.Module.ID,
	}
	if !reflect.DeepEqual(log, want) {
		t.Fatalf("mask rollback lifecycle = %v, want %v", log, want)
	}
	for _, action := range []string{"stop:" + maskDescriptor.Module.ID, "stop:mask-failing", "close:" + maskDescriptor.Module.ID, "close:mask-failing", "close:mask-after"} {
		count := 0
		for _, event := range log {
			if event == action {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("mask rollback action %q count = %d, want 1", action, count)
		}
	}
}

type maskLifecycleHost struct{ id string }

func (host maskLifecycleHost) ModuleID() string { return host.id }

type countedMaskInstance struct {
	id    string
	inner Instance
	log   *[]string
}

func (instance *countedMaskInstance) record(action string) {
	*instance.log = append(*instance.log, action+":"+instance.id)
}

func (instance *countedMaskInstance) Start(ctx context.Context) error {
	instance.record("start")
	return instance.inner.Start(ctx)
}

func (instance *countedMaskInstance) Ready(ctx context.Context) error {
	instance.record("ready")
	return instance.inner.Ready(ctx)
}

func (instance *countedMaskInstance) Stop(ctx context.Context) error {
	instance.record("stop")
	return instance.inner.Stop(ctx)
}

func (instance *countedMaskInstance) Close(ctx context.Context) error {
	instance.record("close")
	return instance.inner.Close(ctx)
}

func TestGenerationCloseIsReverseAndIdempotent(t *testing.T) {
	var log []string
	owners := []*lifecycleProbe{{id: "one", log: &log}, {id: "two", log: &log}}
	generation, err := Start(context.Background(), instances(owners...))
	if err != nil {
		t.Fatal(err)
	}
	log = nil
	if err := generation.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := generation.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop:two", "close:two", "stop:one", "close:one"}
	if !reflect.DeepEqual(log, want) {
		t.Fatalf("close lifecycle = %v, want %v", log, want)
	}
}

func TestGenerationLifecyclePropagatesDeadlineCause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var log []string
	_, err := Start(ctx, instances(&lifecycleProbe{id: "deadline", log: &log, checkContext: true}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() error = %v, want context cancellation in cause chain", err)
	}
	want := []string{"start:deadline", "stop:deadline", "close:deadline"}
	if !reflect.DeepEqual(log, want) {
		t.Fatalf("deadline rollback = %v, want %v", log, want)
	}
}

func instances(probes ...*lifecycleProbe) []Instance {
	out := make([]Instance, len(probes))
	for index, probe := range probes {
		out[index] = probe
	}
	return out
}
