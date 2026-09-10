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
