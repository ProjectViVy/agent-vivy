package channel

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

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
	return nil
}

func (p *providerProbe) Stop(context.Context) error {
	p.stops++
	return nil
}

func (*providerProbe) Send(context.Context, channelport.OutboundMessage) ([]string, error) {
	return nil, nil
}

func validDeps(t *testing.T) channelcontract.Dependencies {
	t.Helper()
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "channel-owner.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return channelcontract.Dependencies{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Run: func(context.Context, domain.SessionID, string, *domain.Provenance) (domain.RunID, error) {
			return "run-test", nil
		},
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
	instance, err := NewFactory().Construct(context.Background(), validDeps(t), channelcontract.Selection{
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
	if !state.Compiled || state.ProcessAvailable || len(state.Providers) != 1 || provider.starts != 0 {
		t.Fatalf("state=%+v starts=%d", state, provider.starts)
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
