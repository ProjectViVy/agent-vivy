package channelcontract_test

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/rpccontract"
	"agent-vivy/internal/storage"
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

func TestSelectionCarriesProcessAvailability(t *testing.T) {
	selection := channelcontract.Selection{ProcessAvailable: true}
	if !selection.ProcessAvailable {
		t.Fatal("ProcessAvailable = false, want true")
	}
}

func TestInvalidSettingsMarkerPreservesMessageAndIdentity(t *testing.T) {
	want := errors.New("allow_from wildcard is forbidden")
	got := channelcontract.MarkInvalidSettings(want)
	if got.Error() != want.Error() || !errors.Is(got, channelcontract.ErrInvalidSettings) {
		t.Fatalf("marked error = %v", got)
	}
	if channelcontract.MarkInvalidSettings(nil) != nil {
		t.Fatal("MarkInvalidSettings(nil) must return nil")
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

func TestDependenciesCarryFocusedAuthoritiesAndPreparedRun(t *testing.T) {
	settings := fakeSettingsAccess{writable: true, frozen: true}
	changed := false
	var deliveries storage.ChannelDeliveryStore
	var maintenance storage.ChannelMaintenanceStore
	var approvals storage.ApprovalStore
	var runs storage.RunStore
	prepared := domain.RunID("")
	decided := false
	deps := channelcontract.Dependencies{
		Settings:    settings,
		Deliveries:  deliveries,
		Maintenance: maintenance,
		Approvals:   approvals,
		Runs:        runs,
		Run: func(_ context.Context, _ domain.SessionID, _ string, attachments []domain.Attachment, _ *domain.Provenance, prepare channelcontract.PrepareRunCallback) (domain.RunID, error) {
			if len(attachments) != 1 || attachments[0].Name != "image.png" {
				t.Fatalf("attachments = %#v, want image.png", attachments)
			}
			runID := domain.RunID("run-prepared")
			if err := prepare(runID); err != nil {
				return "", err
			}
			return runID, nil
		},
		DecideApproval: func(_ context.Context, approvalID, decision, actor string) error {
			if approvalID != "approval-1" || decision != "approved" || actor != "channel:fake:alice" {
				t.Fatalf("decision = %q/%q/%q", approvalID, decision, actor)
			}
			decided = true
			return nil
		},
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
	runID, err := deps.Run(
		context.Background(),
		"session-1",
		"hello",
		[]domain.Attachment{{Name: "image.png", MimeType: "image/png", Data: []byte("png")}},
		&domain.Provenance{Source: domain.SourceChannel, Channel: "fake"},
		func(runID domain.RunID) error {
			prepared = runID
			return nil
		},
	)
	if err != nil || runID != "run-prepared" || prepared != runID {
		t.Fatalf("run = %q prepared = %q err = %v", runID, prepared, err)
	}
	if err := deps.DecideApproval(context.Background(), "approval-1", "approved", "channel:fake:alice"); err != nil {
		t.Fatal(err)
	}
	if !decided {
		t.Fatal("approval callback was not preserved")
	}
}
