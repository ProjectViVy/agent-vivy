package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
)

func bindChannels(providers []channel.ChannelProvider, grants map[string][]module.GrantBinding, configured config.Channels) ([]channel.Channel, error) {
	byName := make(map[string]bool)
	out := make([]channel.Channel, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		definition := provider.Definition()
		name := strings.TrimPrefix(definition.ID, "vivy.")
		if name == "" {
			return nil, fmt.Errorf("app: channel provider %q has no name", definition.ID)
		}
		if byName[name] {
			return nil, fmt.Errorf("app: duplicate channel provider name %q", name)
		}
		byName[name] = true
		out = append(out, &providerChannel{name: name, provider: provider, grants: cloneGrantBindings(grants[definition.ID]), maxRunes: definition.MaxMessageRunes})
	}
	for name := range configured {
		if !byName[name] {
			return nil, fmt.Errorf("app: channels.%s is not compiled into this generation", name)
		}
	}
	return out, nil
}

// BindChannels exposes the internal ChannelHost binding boundary to the SDK
// conformance suite. Product composition uses the same implementation.
func BindChannels(providers []channel.ChannelProvider, grants map[string][]module.GrantBinding, configured config.Channels) ([]channel.Channel, error) {
	return bindChannels(providers, grants, configured)
}

func compiledChannelNames(providers []channel.ChannelProvider) []string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		if provider != nil {
			names = append(names, strings.TrimPrefix(provider.Definition().ID, "vivy."))
		}
	}
	sort.Strings(names)
	return names
}

type providerChannel struct {
	name     string
	provider channel.ChannelProvider
	grants   []module.GrantBinding
	instance channel.Instance
	maxRunes int
}

func (c *providerChannel) Name() string { return c.name }
func (c *providerChannel) Grants() []module.Grant {
	out := make([]module.Grant, 0, len(c.grants))
	for _, grant := range c.grants {
		out = append(out, grant.Name)
	}
	return out
}
func (c *providerChannel) Start(ctx context.Context, host channel.Host) error {
	instance, err := c.provider.Construct(ctx, grantedChannelHost{Host: host, moduleID: c.provider.Definition().ID, grants: c.grants})
	if err != nil {
		return err
	}
	if err := instance.Start(ctx); err != nil {
		_ = instance.Stop(ctx)
		return err
	}
	c.instance = instance
	return nil
}
func (c *providerChannel) Stop(ctx context.Context) error {
	if c.instance == nil {
		return nil
	}
	instance := c.instance
	c.instance = nil
	return instance.Stop(ctx)
}
func (c *providerChannel) Send(ctx context.Context, msg channel.OutboundMessage) ([]string, error) {
	if c.instance == nil {
		return nil, fmt.Errorf("channel %s is not started", c.name)
	}
	return c.instance.Send(ctx, msg)
}
func (c *providerChannel) MaxMessageRunes() int { return c.maxRunes }

func cloneGrantBindings(bindings []module.GrantBinding) []module.GrantBinding {
	out := make([]module.GrantBinding, len(bindings))
	for i, binding := range bindings {
		out[i] = module.GrantBinding{Name: binding.Name, Constraints: make(map[string][]string, len(binding.Constraints))}
		for key, values := range binding.Constraints {
			out[i].Constraints[key] = append([]string(nil), values...)
		}
	}
	return out
}

type grantedChannelHost struct {
	channel.Host
	moduleID string
	grants   []module.GrantBinding
}

func (host grantedChannelHost) ModuleID() string { return host.moduleID }

func (host grantedChannelHost) Secret(name string) (string, error) {
	binding, ok := host.binding(module.GrantSecretRead)
	if !ok || (len(binding.Constraints["names"]) > 0 && !containsConstraint(binding.Constraints["names"], name)) {
		return "", channel.ErrDenied
	}
	return host.Host.Secret(name)
}

func (host grantedChannelHost) HTTP() *http.Client {
	binding, ok := host.binding(module.GrantNetClient)
	if !ok {
		return &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, channel.ErrDenied
		})}
	}
	base := host.Host.HTTP()
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = constrainedTransport{base: transport, constraints: binding.Constraints}
	return &client
}

func (host grantedChannelHost) DialTLS(ctx context.Context, network, address string) (net.Conn, error) {
	binding, ok := host.binding(module.GrantNetClient)
	if !ok || !containsFold(binding.Constraints["schemes"], "wss") {
		return nil, channel.ErrDenied
	}
	hostname, port, err := net.SplitHostPort(address)
	if err != nil || !containsFold(binding.Constraints["hosts"], strings.Trim(hostname, "[]")) {
		return nil, channel.ErrDenied
	}
	if ports := binding.Constraints["ports"]; len(ports) > 0 && !containsConstraint(ports, port) {
		return nil, channel.ErrDenied
	}
	return host.Host.DialTLS(ctx, network, address)
}

func (host grantedChannelHost) Settings() json.RawMessage { return host.Host.Settings() }

func (host grantedChannelHost) PublishInbound(ctx context.Context, message channel.InboundMessage) error {
	for _, grant := range []module.Grant{module.GrantChannelPoll, module.GrantChannelWebhook, module.GrantChannelListen, module.GrantChannelA2A} {
		if _, ok := host.binding(grant); ok {
			return host.Host.PublishInbound(ctx, message)
		}
	}
	return channel.ErrDenied
}

func (host grantedChannelHost) Media() channel.MediaStore { return host.Host.Media() }
func (host grantedChannelHost) Logger() *slog.Logger      { return host.Host.Logger() }

func (host grantedChannelHost) binding(name module.Grant) (module.GrantBinding, bool) {
	for _, binding := range host.grants {
		if binding.Name == name {
			return binding, true
		}
	}
	return module.GrantBinding{}, false
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type constrainedTransport struct {
	base        http.RoundTripper
	constraints map[string][]string
}

func (transport constrainedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	hostname := strings.ToLower(request.URL.Hostname())
	scheme := strings.ToLower(request.URL.Scheme)
	port := request.URL.Port()
	if port == "" {
		switch scheme {
		case "https", "wss":
			port = "443"
		case "http", "ws":
			port = "80"
		}
	}
	if !containsFold(transport.constraints["hosts"], hostname) || !containsFold(transport.constraints["schemes"], scheme) {
		return nil, channel.ErrDenied
	}
	if ports := transport.constraints["ports"]; len(ports) > 0 && !containsConstraint(ports, port) {
		return nil, channel.ErrDenied
	}
	return transport.base.RoundTrip(request)
}

func containsFold(values []string, value string) bool {
	for _, candidate := range values {
		if strings.EqualFold(candidate, value) {
			return true
		}
	}
	return false
}
