package channel

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

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/channelhost"
	"agent-vivy/sdk/module"
	channelport "agent-vivy/sdk/port/channel"
)

type bindingStubHost struct {
	client       *http.Client
	secretCalls  int
	dialCalls    int
	inboundCalls int
}

func (*bindingStubHost) ModuleID() string { return "kernel/channel-host" }
func (host *bindingStubHost) DialTLS(context.Context, string, string) (net.Conn, error) {
	host.dialCalls++
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}
func (host *bindingStubHost) Secret(name string) (string, error) {
	host.secretCalls++
	return "value:" + name, nil
}
func (host *bindingStubHost) HTTP() *http.Client       { return host.client }
func (*bindingStubHost) Settings() json.RawMessage     { return json.RawMessage(`{}`) }
func (*bindingStubHost) Media() channelport.MediaStore { return nil }
func (*bindingStubHost) Logger() *slog.Logger          { return slog.Default() }
func (host *bindingStubHost) PublishInbound(context.Context, channelport.InboundMessage) error {
	host.inboundCalls++
	return nil
}

type bindingStubProvider struct{ id string }

func (p bindingStubProvider) Definition() channelport.Definition {
	return channelport.Definition{ID: p.id}
}
func (bindingStubProvider) Construct(context.Context, channelport.Host) (channelport.Instance, error) {
	return bindingStubInstance{}, nil
}

type bindingStubInstance struct{}

func (bindingStubInstance) Start(context.Context) error { return nil }
func (bindingStubInstance) Stop(context.Context) error  { return nil }
func (bindingStubInstance) Send(context.Context, channelport.OutboundMessage) ([]string, error) {
	return nil, nil
}

func TestBindProvidersUsesTypedProviders(t *testing.T) {
	providers := []channelport.ChannelProvider{bindingStubProvider{id: "vivy.fake"}}
	got, err := bindProviders(providers, map[string][]module.GrantBinding{
		"vivy.fake": {{Name: module.GrantChannelPoll}},
	}, channelcontract.Config{"fake": {}})
	if err != nil || len(got) != 1 || got[0].Name() != "fake" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if names := compiledProviderNames(providers); len(names) != 1 || names[0] != "fake" {
		t.Fatal(names)
	}
}

func TestBindProvidersRejectsDuplicateAndUncompiledNames(t *testing.T) {
	provider := bindingStubProvider{id: "vivy.fake"}
	if _, err := bindProviders([]channelport.ChannelProvider{provider, provider}, nil, nil); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate error=%v", err)
	}
	if _, err := bindProviders([]channelport.ChannelProvider{provider}, nil, channelcontract.Config{"missing": {}}); err == nil || !strings.Contains(err.Error(), "not compiled") {
		t.Fatalf("uncompiled error=%v", err)
	}
}

func TestBindProvidersDefensivelyCopiesGrantConstraints(t *testing.T) {
	bindings := map[string][]module.GrantBinding{
		"vivy.fake": {{Name: module.GrantSecretRead, Constraints: map[string][]string{"names": {"TOKEN"}}}},
	}
	got, err := bindProviders([]channelport.ChannelProvider{bindingStubProvider{id: "vivy.fake"}}, bindings, nil)
	if err != nil {
		t.Fatal(err)
	}
	bindings["vivy.fake"][0].Constraints["names"][0] = "MUTATED"
	bound := got[0].(*providerChannel)
	if bound.grants[0].Constraints["names"][0] != "TOKEN" {
		t.Fatalf("grant constraints changed through caller alias: %#v", bound.grants)
	}
}

type capabilityAdapter struct{}

func (*capabilityAdapter) Typing(context.Context, string) error { return nil }
func (*capabilityAdapter) Health(context.Context) error         { return nil }

type capabilityInstance struct{}

func (capabilityInstance) Start(context.Context) error { return nil }
func (capabilityInstance) Stop(context.Context) error  { return nil }
func (capabilityInstance) Send(context.Context, channelport.OutboundMessage) ([]string, error) {
	return nil, nil
}

type capabilityProvider struct{}

func (capabilityProvider) Definition() channelport.Definition {
	return channelport.Definition{ID: "vivy.capability", MaxMessageRunes: 1234}
}

func (capabilityProvider) Construct(context.Context, channelport.Host) (channelport.Instance, error) {
	return capabilityInstance{}, nil
}

func (capabilityProvider) CapabilityTarget() any { return (*capabilityAdapter)(nil) }

func TestBindProvidersPreservesCapabilityTarget(t *testing.T) {
	bound, err := BindProviders([]channelport.ChannelProvider{capabilityProvider{}}, nil, nil)
	if err != nil || len(bound) != 1 {
		t.Fatalf("bound = %#v, err = %v", bound, err)
	}
	caps := channelhost.Discover(bound[0])
	if !caps.Typing || !caps.Health {
		t.Fatal(caps)
	}
	limited := bound[0].(channelport.RunesLimiter)
	if limited.MaxMessageRunes() != 1234 {
		t.Fatal(limited.MaxMessageRunes())
	}
}

func TestGrantedChannelHostEnforcesSecretAndNetworkConstraints(t *testing.T) {
	requests := 0
	base := &bindingStubHost{client: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}}
	host := grantedChannelHost{Host: base, moduleID: "vivy.fake", grants: []module.GrantBinding{
		{Name: module.GrantSecretRead, Constraints: map[string][]string{"names": {"FAKE_TOKEN"}}},
		{Name: module.GrantNetClient, Constraints: map[string][]string{"hosts": {"api.example.com"}, "schemes": {"https", "wss"}, "ports": {"443"}}},
	}}
	if _, err := host.Secret("OTHER_TOKEN"); !errors.Is(err, channelport.ErrDenied) || base.secretCalls != 0 {
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
	if _, err := host.HTTP().Get("https://evil.example/v1"); !errors.Is(err, channelport.ErrDenied) {
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
	if _, err := host.DialTLS(context.Background(), "tcp", "evil.example:443"); !errors.Is(err, channelport.ErrDenied) {
		t.Fatalf("unapproved dial error=%v, want ErrDenied", err)
	}
}

func TestGrantedChannelHostRequiresInboundGrant(t *testing.T) {
	base := &bindingStubHost{}
	without := grantedChannelHost{Host: base, moduleID: "vivy.fake"}
	if err := without.PublishInbound(context.Background(), channelport.InboundMessage{}); !errors.Is(err, channelport.ErrDenied) {
		t.Fatalf("without inbound grant error=%v", err)
	}
	with := grantedChannelHost{Host: base, moduleID: "vivy.fake", grants: []module.GrantBinding{{Name: module.GrantChannelPoll}}}
	if err := with.PublishInbound(context.Background(), channelport.InboundMessage{}); err != nil {
		t.Fatal(err)
	}
	if base.inboundCalls != 1 {
		t.Fatalf("inbound calls=%d, want 1", base.inboundCalls)
	}
}

func TestGrantedChannelHostWithoutNetworkGrantReturnsDeniedClient(t *testing.T) {
	host := grantedChannelHost{Host: &bindingStubHost{}, moduleID: "vivy.fake"}
	if _, err := host.HTTP().Get("https://api.example.com"); !errors.Is(err, channelport.ErrDenied) {
		t.Fatalf("network error=%v, want ErrDenied", err)
	}
	if _, err := host.DialTLS(context.Background(), "tcp", "api.example.com:443"); !errors.Is(err, channelport.ErrDenied) {
		t.Fatalf("dial error=%v, want ErrDenied", err)
	}
}
