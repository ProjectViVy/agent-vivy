package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"

	"agent-vivy/internal/channelhost"
	"agent-vivy/internal/config"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
)

type stubChannelHost struct {
	client      *http.Client
	secretCalls int
	dialCalls   int
}

func (*stubChannelHost) ModuleID() string { return "kernel/channel-host" }
func (host *stubChannelHost) DialTLS(context.Context, string, string) (net.Conn, error) {
	host.dialCalls++
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}
func (host *stubChannelHost) Secret(name string) (string, error) {
	host.secretCalls++
	return "value:" + name, nil
}
func (host *stubChannelHost) HTTP() *http.Client   { return host.client }
func (*stubChannelHost) Settings() json.RawMessage { return json.RawMessage(`{}`) }
func (*stubChannelHost) PublishInbound(context.Context, channel.InboundMessage) error {
	return nil
}
func (*stubChannelHost) Media() channel.MediaStore { return nil }
func (*stubChannelHost) Logger() *slog.Logger      { return slog.Default() }

type stubChannelProvider struct{ id string }

func (p stubChannelProvider) Definition() channel.Definition { return channel.Definition{ID: p.id} }
func (stubChannelProvider) Construct(context.Context, channel.Host) (channel.Instance, error) {
	return stubChannelInstance{}, nil
}

type stubChannelInstance struct{}

func (stubChannelInstance) Start(context.Context) error { return nil }
func (stubChannelInstance) Stop(context.Context) error  { return nil }
func (stubChannelInstance) Send(context.Context, channel.OutboundMessage) ([]string, error) {
	return nil, nil
}

func TestBindChannelsUsesTypedProviders(t *testing.T) {
	providers := []channel.ChannelProvider{stubChannelProvider{id: "vivy.fake"}}
	got, err := bindChannels(providers, map[string][]module.GrantBinding{"vivy.fake": {{Name: module.GrantChannelPoll}}}, config.Channels{"fake": {}})
	if err != nil || len(got) != 1 || got[0].Name() != "fake" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if names := compiledChannelNames(providers); len(names) != 1 || names[0] != "fake" {
		t.Fatal(names)
	}
}

func TestBindChannelsRejectsUncompiledConfig(t *testing.T) {
	_, err := bindChannels([]channel.ChannelProvider{stubChannelProvider{id: "vivy.fake"}}, nil, config.Channels{"missing": {}})
	if err == nil || !strings.Contains(err.Error(), "not compiled") {
		t.Fatalf("err=%v", err)
	}
}

// capabilityAdapter implements two optional interfaces so the test can see
// them advertised through the full bind chain.
type capabilityAdapter struct{}

func (capabilityAdapter) Typing(_ context.Context, _ string) error { return nil }
func (capabilityAdapter) Health(_ context.Context) error           { return nil }

type capabilityInstance struct{}

func (capabilityInstance) Start(context.Context) error { return nil }
func (capabilityInstance) Stop(context.Context) error  { return nil }
func (capabilityInstance) Send(context.Context, channel.OutboundMessage) ([]string, error) {
	return nil, nil
}

type capabilityProvider struct{ id string }

func (p capabilityProvider) Definition() channel.Definition {
	return channel.Definition{ID: p.id, MaxMessageRunes: 1234}
}
func (capabilityProvider) Construct(context.Context, channel.Host) (channel.Instance, error) {
	return capabilityInstance{}, nil
}

// CapabilityTarget points Discover at the adapter's method set — the same
// typed-nil probe the five compiled channel providers use.
func (capabilityProvider) CapabilityTarget() any { return (*capabilityAdapter)(nil) }

// TestBindChannelsExposesAdapterCapabilities: a capability that only the
// adapter implements is advertised through the providerChannel wrapper, and
// the Definition's rune ceiling reaches the host's RunesLimiter probe.
func TestBindChannelsExposesAdapterCapabilities(t *testing.T) {
	bound, err := bindChannels([]channel.ChannelProvider{capabilityProvider{id: "vivy.fake"}}, nil, nil)
	if err != nil || len(bound) != 1 {
		t.Fatalf("bound=%v err=%v", bound, err)
	}
	caps := channelhost.Discover(bound[0])
	if !caps.Typing || !caps.Health {
		t.Fatalf("capabilities = %+v, want Typing and Health from the adapter", caps)
	}
	if caps.Edit || caps.Delete || caps.Reaction || caps.Placeholder || caps.Media ||
		caps.MediaStore || caps.Webhook || caps.Listen || caps.Stream {
		t.Fatalf("capabilities = %+v, want nothing beyond the adapter's own", caps)
	}
	limited, ok := bound[0].(channel.RunesLimiter)
	if !ok || limited.MaxMessageRunes() != 1234 {
		t.Fatalf("MaxMessageRunes probe = %v, want the Definition ceiling 1234", limited)
	}
}

func TestGrantedChannelHostEnforcesSecretAndNetworkConstraints(t *testing.T) {
	requests := 0
	base := &stubChannelHost{client: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}}
	host := grantedChannelHost{Host: base, moduleID: "vivy.fake", grants: []module.GrantBinding{
		{Name: module.GrantSecretRead, Constraints: map[string][]string{"names": {"FAKE_TOKEN"}}},
		{Name: module.GrantNetClient, Constraints: map[string][]string{"hosts": {"api.example.com"}, "schemes": {"https", "wss"}, "ports": {"443"}}},
	}}
	if _, err := host.Secret("OTHER_TOKEN"); !errors.Is(err, channel.ErrDenied) || base.secretCalls != 0 {
		t.Fatalf("secret error=%v calls=%d", err, base.secretCalls)
	}
	if got, err := host.Secret("FAKE_TOKEN"); err != nil || got != "value:FAKE_TOKEN" || base.secretCalls != 1 {
		t.Fatalf("secret=%q error=%v calls=%d", got, err, base.secretCalls)
	}
	response, err := host.HTTP().Get("https://api.example.com/v1")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if _, err := host.HTTP().Get("https://evil.example/v1"); !errors.Is(err, channel.ErrDenied) {
		t.Fatalf("unapproved network error=%v, want ErrDenied", err)
	}
	if requests != 1 {
		t.Fatalf("base transport requests=%d, want 1", requests)
	}
	connection, err := host.DialTLS(context.Background(), "tcp", "api.example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if _, err := host.DialTLS(context.Background(), "tcp", "evil.example:443"); !errors.Is(err, channel.ErrDenied) {
		t.Fatalf("unapproved dial error=%v, want ErrDenied", err)
	}
	if base.dialCalls != 1 {
		t.Fatalf("base dial calls=%d, want 1", base.dialCalls)
	}
}

func TestGrantedChannelHostWithoutNetworkGrantReturnsDeniedClient(t *testing.T) {
	host := grantedChannelHost{Host: &stubChannelHost{}, moduleID: "vivy.fake"}
	if _, err := host.HTTP().Get("https://api.example.com"); !errors.Is(err, channel.ErrDenied) {
		t.Fatalf("network error=%v, want ErrDenied", err)
	}
	if _, err := host.DialTLS(context.Background(), "tcp", "api.example.com:443"); !errors.Is(err, channel.ErrDenied) {
		t.Fatalf("dial error=%v, want ErrDenied", err)
	}
}
