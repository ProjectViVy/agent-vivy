package channelcontract_test

import (
	"context"
	"testing"

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/rpc"
)

type fakeOwned struct{}

func (*fakeOwned) Start(context.Context) error { return nil }
func (*fakeOwned) Ready(context.Context) error { return nil }
func (*fakeOwned) Stop(context.Context) error  { return nil }
func (*fakeOwned) Close(context.Context) error { return nil }
func (*fakeOwned) OnRunEvent(context.Context, domain.RunEvent) {
}
func (*fakeOwned) Deliver(context.Context, string, string, string) error { return nil }
func (*fakeOwned) RPCBindings() []rpc.MethodBinding                      { return nil }
func (*fakeOwned) Inspect() channelcontract.State                        { return channelcontract.State{} }

type fakeFactory struct{}

func (fakeFactory) Construct(context.Context, channelcontract.Dependencies, channelcontract.Selection) (channelcontract.Owned, error) {
	return &fakeOwned{}, nil
}

var _ channelcontract.Owned = (*fakeOwned)(nil)
var _ channelcontract.Factory = fakeFactory{}

func TestSelectionAllowsZeroProviders(t *testing.T) {
	got := channelcontract.Selection{Config: channelcontract.Config{}}
	if len(got.Providers) != 0 {
		t.Fatalf("providers = %d, want 0", len(got.Providers))
	}
}

func TestStateKeepsCompiledAndProcessAvailabilitySeparate(t *testing.T) {
	got := channelcontract.State{Compiled: true, ProcessAvailable: false}
	if !got.Compiled || got.ProcessAvailable {
		t.Fatalf("state = %+v, want compiled but process unavailable", got)
	}
}
