package channel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	channelport "agent-vivy/sdk/port/channel"
)

var _ channelcontract.Factory = NewFactory()
var _ channelcontract.Owned = (*owned)(nil)

type providerProbe struct {
	starts   int
	stops    int
	settings json.RawMessage
	onStart  func()
}

func (*providerProbe) Definition() channelport.Definition {
	return channelport.Definition{ID: "vivy.fake"}
}

func (p *providerProbe) Construct(_ context.Context, host channelport.Host) (channelport.Instance, error) {
	p.settings = append(json.RawMessage(nil), host.Settings()...)
	return p, nil
}

func (p *providerProbe) Start(context.Context) error {
	p.starts++
	if p.onStart != nil {
		p.onStart()
	}
	return nil
}

func (p *providerProbe) Stop(context.Context) error {
	p.stops++
	return nil
}

func (*providerProbe) Send(context.Context, channelport.OutboundMessage) ([]string, error) {
	return nil, nil
}

type maintenanceProbe struct {
	calls  int
	cutoff time.Time
	err    error
	events *[]string
}

func (probe *maintenanceProbe) PruneChannelInboundEvents(_ context.Context, cutoff time.Time) (int, error) {
	probe.calls++
	probe.cutoff = cutoff
	if probe.events != nil {
		*probe.events = append(*probe.events, "prune")
	}
	return 1, probe.err
}

func validDeps(t *testing.T) channelcontract.Dependencies {
	t.Helper()
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "channel-owner.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return channelcontract.Dependencies{
		Journal:     backend,
		Messages:    backend,
		Sessions:    backend,
		Deliveries:  backend,
		Maintenance: backend,
		Approvals:   backend,
		Runs:        backend,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Run: func(_ context.Context, _ domain.SessionID, _ string, _ []domain.Attachment, _ *domain.Provenance, prepare channelcontract.PrepareRunCallback) (domain.RunID, error) {
			runID := domain.RunID("run-test")
			if err := prepare(runID); err != nil {
				return "", err
			}
			return runID, nil
		},
		DecideApproval: func(context.Context, string, string, string) error { return nil },
	}
}

func TestProcessAvailableRequiresDeliveryDependencies(t *testing.T) {
	deps := validDeps(t)
	deps.Deliveries = nil
	if _, err := NewFactory().Construct(context.Background(), deps, channelcontract.Selection{ProcessAvailable: true}); err == nil {
		t.Fatal("available process accepted missing durable delivery store")
	}
	instance, err := NewFactory().Construct(context.Background(), deps, channelcontract.Selection{ProcessAvailable: false})
	if err != nil {
		t.Fatalf("unavailable process rejected optional delivery store: %v", err)
	}
	if err := instance.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPreparedOwnerStartPrunesBeforeProviderStart(t *testing.T) {
	events := []string{}
	maintenance := &maintenanceProbe{events: &events}
	provider := &providerProbe{onStart: func() { events = append(events, "start") }}
	deps := validDeps(t)
	deps.Maintenance = maintenance
	instance, err := NewFactory().Construct(context.Background(), deps, channelcontract.Selection{
		Providers: []channelport.ChannelProvider{provider},
		Config: channelcontract.Config{"fake": {
			Enabled: true, AllowFrom: []string{"alice"},
		}},
		ProcessAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-30 * 24 * time.Hour)
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Add(-30 * 24 * time.Hour)
	t.Cleanup(func() { _ = instance.Close(context.Background()) })
	if !reflect.DeepEqual(events, []string{"prune", "start"}) {
		t.Fatalf("events = %v, want prune before start", events)
	}
	if maintenance.cutoff.Before(before) || maintenance.cutoff.After(after) {
		t.Fatalf("retention cutoff = %v, want between %v and %v", maintenance.cutoff, before, after)
	}
}

func TestPreparedOwnerStartContinuesAfterPruneFailure(t *testing.T) {
	provider := &providerProbe{}
	deps := validDeps(t)
	deps.Maintenance = &maintenanceProbe{err: errors.New("prune failed")}
	instance, err := NewFactory().Construct(context.Background(), deps, channelcontract.Selection{
		Providers: []channelport.ChannelProvider{provider},
		Config: channelcontract.Config{"fake": {
			Enabled: true, AllowFrom: []string{"alice"},
		}},
		ProcessAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = instance.Close(context.Background()) })
	if provider.starts != 1 {
		t.Fatalf("provider starts = %d, want 1 after non-fatal prune failure", provider.starts)
	}
}

func TestConstructDoesNotStartProviders(t *testing.T) {
	provider := &providerProbe{}
	instance, err := NewFactory().Construct(context.Background(), validDeps(t), channelcontract.Selection{
		Providers: []channelport.ChannelProvider{provider},
		Config: channelcontract.Config{"fake": {
			Enabled: true, AllowFrom: []string{"alice"},
		}},
		ProcessAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.starts != 0 {
		t.Fatalf("starts = %d, want 0", provider.starts)
	}
	if err := instance.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestUnavailableProcessKeepsCompiledInventoryWithoutStart(t *testing.T) {
	provider := &providerProbe{}
	maintenance := &maintenanceProbe{}
	deps := validDeps(t)
	deps.Maintenance = maintenance
	instance, err := NewFactory().Construct(context.Background(), deps, channelcontract.Selection{
		Providers:        []channelport.ChannelProvider{provider},
		ProcessAvailable: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := instance.Inspect()
	if !state.Compiled || state.ProcessAvailable || len(state.Providers) != 1 || provider.starts != 0 || maintenance.calls != 0 {
		t.Fatalf("state=%+v starts=%d prune_calls=%d", state, provider.starts, maintenance.calls)
	}
}

func TestOwnedStartAndCloseAreIdempotent(t *testing.T) {
	provider := &providerProbe{}
	instance, err := NewFactory().Construct(context.Background(), validDeps(t), channelcontract.Selection{
		Providers: []channelport.ChannelProvider{provider},
		Config: channelcontract.Config{"fake": {
			Enabled: true, AllowFrom: []string{"alice"},
		}},
		ProcessAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.starts != 1 || provider.stops != 1 {
		t.Fatalf("starts=%d stops=%d, want 1/1", provider.starts, provider.stops)
	}
	if retained := instance.(*owned).config; retained != nil {
		t.Fatalf("closed owner retained config: %+v", retained)
	}
}

func TestConstructRejectsInvalidOpaqueSettings(t *testing.T) {
	_, err := NewFactory().Construct(context.Background(), validDeps(t), channelcontract.Selection{
		Providers: []channelport.ChannelProvider{&providerProbe{}},
		Config: channelcontract.Config{"fake": {
			Settings: []byte(`{"unterminated"`),
		}},
	})
	if err == nil {
		t.Fatal("invalid opaque JSON must fail construction")
	}
}

func TestOpaqueSettingsPreserveJSONNumberPrecisionThroughProviderHost(t *testing.T) {
	provider := &providerProbe{}
	instance, err := NewFactory().Construct(context.Background(), validDeps(t), channelcontract.Selection{
		Providers: []channelport.ChannelProvider{provider},
		Config: channelcontract.Config{"fake": {
			Enabled: true, AllowFrom: []string{"alice"},
			Settings: json.RawMessage(`{"large":9007199254740993,"decimal":0.12345678901234567890123456789}`),
		}},
		ProcessAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = instance.Close(context.Background()) })
	if got, want := string(provider.settings), `{"decimal":0.12345678901234567890123456789,"large":9007199254740993}`; got != want {
		t.Fatalf("provider settings = %s, want %s", got, want)
	}
}
