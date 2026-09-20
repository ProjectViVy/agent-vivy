package channelcontract_test

import (
	"context"
	"testing"

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/rpccontract"
)

type fakeOwned struct{}

func (*fakeOwned) Start(context.Context) error { return nil }
func (*fakeOwned) Ready(context.Context) error { return nil }
func (*fakeOwned) Stop(context.Context) error  { return nil }
func (*fakeOwned) Close(context.Context) error { return nil }
func (*fakeOwned) OnRunEvent(context.Context, domain.RunEvent) {
}
func (*fakeOwned) Deliver(context.Context, string, string, string) error { return nil }
func (*fakeOwned) RPCBindings() []rpccontract.MethodBinding              { return nil }
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

type fakeSettingsAccess struct {
	writable bool
	frozen   bool
}

func (f fakeSettingsAccess) Read(context.Context) (channelcontract.Settings, error) {
	return channelcontract.Settings{}, nil
}

func (f fakeSettingsAccess) Update(context.Context, channelcontract.ChannelOverlay) (channelcontract.Settings, error) {
	return channelcontract.Settings{}, nil
}

func (f fakeSettingsAccess) Writable() bool { return f.writable }
func (f fakeSettingsAccess) Frozen() bool   { return f.frozen }

func TestDependenciesCarryFocusedSettingsAuthority(t *testing.T) {
	settings := fakeSettingsAccess{writable: true, frozen: true}
	changed := false
	deps := channelcontract.Dependencies{
		Settings: settings,
		OnSettingsChanged: func() {
			changed = true
		},
	}

	if deps.Settings == nil || !deps.Settings.Writable() || !deps.Settings.Frozen() {
		t.Fatalf("settings authority = %#v, want writable frozen access", deps.Settings)
	}
	deps.OnSettingsChanged()
	if !changed {
		t.Fatal("settings change callback was not preserved")
	}
}
