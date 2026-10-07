package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
)

// bindChannelsWithModuleIDs resolves each compiled channel provider's sealed
// Module identity from the generated ChannelModuleIDs map. When moduleIDs is
// nil the provider's own display ID stands in (SDK conformance callers);
// once the map is supplied every provider needs exactly one entry.
func bindChannelsWithModuleIDs(providers []channel.ChannelProvider, grants map[string][]module.GrantBinding, configured config.Channels, moduleIDs map[string]string) ([]channel.Channel, error) {
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
		moduleID := definition.ID
		if moduleIDs != nil {
			sealed, ok := moduleIDs[definition.ID]
			if !ok || sealed == "" {
				return nil, fmt.Errorf("app: channel provider %q has no sealed module identity", definition.ID)
			}
			moduleID = sealed
		}
		var capabilityTarget any
		if source, ok := provider.(channel.CapabilitySource); ok {
			capabilityTarget = source.CapabilityTarget()
		}
		out = append(out, &providerChannel{name: name, provider: provider, moduleID: moduleID, grants: cloneGrantBindings(grants[definition.ID]), maxRunes: definition.MaxMessageRunes, capabilityTarget: capabilityTarget})
	}
	for name := range configured {
		if !byName[name] {
			return nil, fmt.Errorf("app: channels.%s is not compiled into this generation", name)
		}
	}
	return out, nil
}

func bindChannels(providers []channel.ChannelProvider, grants map[string][]module.GrantBinding, configured config.Channels) ([]channel.Channel, error) {
	return bindChannelsWithModuleIDs(providers, grants, configured, nil)
}

// BindChannels exposes the internal ChannelHost binding boundary to the SDK
// conformance suite. Product composition uses the same implementation.
func BindChannels(providers []channel.ChannelProvider, grants map[string][]module.GrantBinding, configured config.Channels) ([]channel.Channel, error) {
	return bindChannels(providers, grants, configured)
}

// BindChannelsWithModuleIDs is the product binding path: the generated
// Assembly's ChannelModuleIDs map hands each provider its sealed Module
// identity instead of letting the wrapper borrow the provider display ID.
func BindChannelsWithModuleIDs(providers []channel.ChannelProvider, grants map[string][]module.GrantBinding, configured config.Channels, moduleIDs map[string]string) ([]channel.Channel, error) {
	return bindChannelsWithModuleIDs(providers, grants, configured, moduleIDs)
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
	name             string
	provider         channel.ChannelProvider
	moduleID         string
	grants           []module.GrantBinding
	instance         channel.Instance
	maxRunes         int
	capabilityTarget any
	stopListener     func(context.Context) error
}

func (c *providerChannel) Name() string { return c.name }

// ModuleID returns the sealed Module identity from the generated
// ChannelModuleIDs map (vivy/telegram, projectvivy/a2a-server, …); the
// channel host resolves credentials against it instead of the legacy
// "vivy/"+name derivation.
func (c *providerChannel) ModuleID() string { return c.moduleID }
func (c *providerChannel) Grants() []module.Grant {
	out := make([]module.Grant, 0, len(c.grants))
	for _, grant := range c.grants {
		out = append(out, grant.Name)
	}
	return out
}
func (c *providerChannel) Start(ctx context.Context, host channel.Host) error {
	base := grantedChannelHost{Host: host, moduleID: c.moduleID, grants: c.grants}
	wrapped := channel.Host(base)
	// The optional task contract is advertised only when the underlying Host
	// really implements it and the Module carries an effective channel.a2a
	// grant — the wrapper itself re-checks the grant on every call.
	if tasks, ok := host.(channel.TaskHost); ok && !nilish(host) {
		if _, granted := base.binding(module.GrantChannelA2A); granted {
			wrapped = channel.Host(taskGrantedChannelHost{grantedChannelHost: base, tasks: tasks})
			if info, ok := host.(channel.TaskServiceInfoHost); ok {
				wrapped = channel.Host(taskInfoGrantedChannelHost{taskGrantedChannelHost: wrapped.(taskGrantedChannelHost), info: info})
			}
		}
	}
	instance, err := c.provider.Construct(ctx, wrapped)
	if err != nil {
		return err
	}
	if err := instance.Start(ctx); err != nil {
		_ = instance.Stop(ctx)
		return err
	}
	// A live ListenHandler mounts the dedicated task listener only when the
	// env carries the task surface AND the channel envelope configures an
	// http block — otherwise the capability stays inactive (§10.1). The
	// instance local is used here: c.instance is assigned below.
	if lh, ok := instance.(channel.ListenHandler); ok {
		type taskServerEnv interface {
			ServeTaskHTTP(context.Context, http.Handler) (func(context.Context) error, error)
		}
		if env, ok := host.(taskServerEnv); ok {
			stop, err := env.ServeTaskHTTP(ctx, lh.ListenHandler())
			if err != nil {
				// Recorded as a failed listener in Inspect; the channel
				// itself still started — no partial route exists.
				slog.Warn("app: task listener failed", "channel", c.name, "error", err)
			} else {
				c.stopListener = stop
			}
		}
	}
	c.instance = instance
	return nil
}
func (c *providerChannel) Stop(ctx context.Context) error {
	if c.stopListener != nil {
		// The listener stops before the instance (reverse ownership order).
		if err := c.stopListener(ctx); err != nil {
			slog.Warn("app: task listener drain failed", "channel", c.name, "error", err)
		}
		c.stopListener = nil
	}
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

// CapabilityTarget implements plugin.CapabilitySource: discovery and host
// call paths follow it to the adapter behind this wrapper. Before Start has
// constructed the instance, the bind-time typed-nil probe from the provider
// is returned — valid for method-set assertions only. After Start, the
// instance's own disclosure (the live adapter) wins, so calls such as
// typing and health probes reach the real method set. A provider without
// the bind-time seam leaves that target nil and the chain falls back to
// this wrapper, which honestly reports (and can serve) no optional
// capability.
func (c *providerChannel) CapabilityTarget() any {
	if cs, ok := c.instance.(channel.CapabilitySource); ok {
		if t := cs.CapabilityTarget(); t != nil {
			return t
		}
	}
	return c.capabilityTarget
}

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

// taskGrantedChannelHost forwards the optional task contract only behind an
// effective channel.a2a Module grant. Each call re-checks the grant through
// the same binding path the base wrapper uses for secrets and network, so a
// provider never holds a live capability the sealed grant set did not allow.
type taskGrantedChannelHost struct {
	grantedChannelHost
	tasks channel.TaskHost
}

func (host taskGrantedChannelHost) taskBound() error {
	if _, ok := host.binding(module.GrantChannelA2A); !ok {
		return channel.ErrDenied
	}
	return nil
}

func (host taskGrantedChannelHost) SubmitTask(ctx context.Context, request channel.TaskRequest) (channel.TaskRef, error) {
	if err := host.taskBound(); err != nil {
		return channel.TaskRef{}, err
	}
	return host.tasks.SubmitTask(ctx, request)
}

func (host taskGrantedChannelHost) GetTask(ctx context.Context, query channel.TaskQuery) (channel.TaskSnapshot, error) {
	if err := host.taskBound(); err != nil {
		return channel.TaskSnapshot{}, err
	}
	return host.tasks.GetTask(ctx, query)
}

func (host taskGrantedChannelHost) ListTasks(ctx context.Context, query channel.TaskListQuery) (channel.TaskPage, error) {
	if err := host.taskBound(); err != nil {
		return channel.TaskPage{}, err
	}
	return host.tasks.ListTasks(ctx, query)
}

func (host taskGrantedChannelHost) CancelTask(ctx context.Context, query channel.TaskQuery) (channel.TaskSnapshot, error) {
	if err := host.taskBound(); err != nil {
		return channel.TaskSnapshot{}, err
	}
	return host.tasks.CancelTask(ctx, query)
}

func (host taskGrantedChannelHost) SubscribeTask(ctx context.Context, subscription channel.TaskSubscription) (channel.TaskStream, error) {
	if err := host.taskBound(); err != nil {
		return nil, err
	}
	return host.tasks.SubscribeTask(ctx, subscription)
}

// taskInfoGrantedChannelHost adds the discovery reader only when the
// underlying Host serves it: the bound Module grant gates the call, and no
// remote principal is involved in answering.
type taskInfoGrantedChannelHost struct {
	taskGrantedChannelHost
	info channel.TaskServiceInfoHost
}

func (host taskInfoGrantedChannelHost) TaskServiceInfo(ctx context.Context) (channel.TaskServiceInfo, error) {
	if err := host.taskBound(); err != nil {
		return channel.TaskServiceInfo{}, err
	}
	return host.info.TaskServiceInfo(ctx)
}

// nilish reports whether an interface value holds a typed nil (a nil-able
// kind whose value is nil), so capability assertions on it never reach a
// dead receiver.
func nilish(v any) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
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
