package channelhost

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	plugin "agent-vivy/sdk/port/channel"
)

// healthStub is a fake channel with a scripted Health answer.
type healthStub struct {
	*fake.Channel
	err error
}

func (s *healthStub) Health(context.Context) error { return s.err }

func newHealthHost(t *testing.T, ch plugin.Channel, envelope config.ChannelEnvelope) *Host {
	t.Helper()
	backend := openBackend(t)
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run: func(context.Context, domain.SessionID, string, *domain.Provenance) (domain.RunID, error) {
			return "run-health", nil
		},
		Channels: []plugin.Channel{ch},
		Config:   config.Channels{"fake": envelope},
		Logger:   testLogger(),
	})
	return host
}

// TestInspectProbesStartedHealth: the inspect surface probes the Health
// face of started adapters and classifies the answer; channels that are
// not started or have no Health face report nil.
func TestInspectProbesStartedHealth(t *testing.T) {
	envelope := config.ChannelEnvelope{Enabled: true, AllowFrom: []string{"alice"}}

	classify := func(ch plugin.Channel) *ChannelHealth {
		host := newHealthHost(t, ch, envelope)
		if err := host.StartAll(context.Background()); err != nil {
			t.Fatalf("start all: %v", err)
		}
		statuses := host.Inspect()
		if len(statuses) != 1 {
			t.Fatalf("inspect = %+v, want one entry", statuses)
		}
		return statuses[0].Health
	}

	// Healthy adapter.
	if got := classify(func() plugin.Channel {
		stub := &healthStub{Channel: fake.New()}
		return stub
	}()); got == nil || !got.OK {
		t.Fatalf("healthy probe = %+v, want OK", got)
	}

	// Classified dead condition (credentials revoked).
	dead := &healthStub{Channel: fake.New(), err: &plugin.HealthError{Class: plugin.ClassDead, Err: errors.New("token revoked")}}
	if got := classify(dead); got == nil || got.OK || got.Class != string(plugin.ClassDead) || got.Detail != "token revoked" {
		t.Fatalf("dead probe = %+v, want classified dead", got)
	}

	// Plain error defaults to temporary — the supervised-ear assumption.
	plain := &healthStub{Channel: fake.New(), err: errors.New("dial refused")}
	if got := classify(plain); got == nil || got.OK || got.Class != string(plugin.ClassTemporary) {
		t.Fatalf("plain probe = %+v, want default temporary", got)
	}

	// A started adapter without the Health face probes to nil.
	bare := fake.New()
	if got := classify(bare); got != nil {
		t.Fatalf("bare probe = %+v, want nil", got)
	}

	// A channel that never started is never probed.
	deadEar := &healthStub{Channel: fake.New(), err: &plugin.HealthError{Class: plugin.ClassDead, Err: errors.New("ignored")}}
	host := newHealthHost(t, deadEar, config.ChannelEnvelope{Enabled: false})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	statuses := host.Inspect()
	if len(statuses) != 1 || statuses[0].Started || statuses[0].Health != nil {
		t.Fatalf("not-started probe = %+v, want nil health", statuses)
	}
}
