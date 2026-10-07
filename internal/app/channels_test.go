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
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/sdk/generation"
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

// taskStubHost is a Host double implementing the optional task contract;
// the real ChannelHost grows these methods in A2A-02..04.
type taskStubHost struct {
	stubChannelHost
	calls map[string]int
	info  channel.TaskServiceInfo
}

func newTaskStubHost() *taskStubHost {
	return &taskStubHost{calls: map[string]int{}, info: channel.TaskServiceInfo{Name: "Vivy"}}
}
func (h *taskStubHost) SubmitTask(context.Context, channel.TaskRequest) (channel.TaskRef, error) {
	h.calls["submit"]++
	return channel.TaskRef{TaskID: "t1", ContextID: "c1"}, nil
}
func (h *taskStubHost) GetTask(context.Context, channel.TaskQuery) (channel.TaskSnapshot, error) {
	h.calls["get"]++
	return channel.TaskSnapshot{}, nil
}
func (h *taskStubHost) ListTasks(context.Context, channel.TaskListQuery) (channel.TaskPage, error) {
	h.calls["list"]++
	return channel.TaskPage{}, nil
}
func (h *taskStubHost) CancelTask(context.Context, channel.TaskQuery) (channel.TaskSnapshot, error) {
	h.calls["cancel"]++
	return channel.TaskSnapshot{Status: channel.TaskStatus{State: channel.TaskStateCanceled}}, nil
}
func (h *taskStubHost) SubscribeTask(context.Context, channel.TaskSubscription) (channel.TaskStream, error) {
	h.calls["subscribe"]++
	return nil, nil
}
func (h *taskStubHost) TaskServiceInfo(context.Context) (channel.TaskServiceInfo, error) {
	h.calls["info"]++
	return h.info, nil
}

// capturingProvider records the Host its Construct received so the test can
// assert exactly which optional contracts the wrapper advertises.
type capturingProvider struct {
	id   string
	host channel.Host
}

func (p *capturingProvider) Definition() channel.Definition { return channel.Definition{ID: p.id} }
func (p *capturingProvider) Construct(_ context.Context, host channel.Host) (channel.Instance, error) {
	p.host = host
	return stubChannelInstance{}, nil
}

// TestChannelTaskHostGrantWrapper: the task wrapper is constructed only when
// the underlying Host really implements TaskHost AND the Module carries an
// effective channel.a2a grant; granted calls forward and re-check the grant.
func TestChannelTaskHostGrantWrapper(t *testing.T) {
	grants := map[string][]module.GrantBinding{
		"vivy.a2a": {{Name: module.GrantChannelA2A}},
	}
	moduleIDs := map[string]string{"vivy.a2a": "projectvivy/a2a-server"}

	// No channel.a2a grant -> the bound Host must not assert TaskHost.
	ungranted := &capturingProvider{id: "vivy.a2a"}
	bound, err := BindChannelsWithModuleIDs([]channel.ChannelProvider{ungranted}, nil, nil, moduleIDs)
	if err != nil || len(bound) != 1 {
		t.Fatalf("bind=%v err=%v", bound, err)
	}
	if err := bound[0].Start(context.Background(), newTaskStubHost()); err != nil {
		t.Fatal(err)
	}
	if _, ok := ungranted.host.(channel.TaskHost); ok {
		t.Fatal("ungranted wrapper must not assert TaskHost")
	}
	if _, ok := ungranted.host.(channel.TaskServiceInfoHost); ok {
		t.Fatal("ungranted wrapper must not assert TaskServiceInfoHost")
	}

	// channel.a2a grant but a Host without TaskHost -> still no assertion.
	noTask := &capturingProvider{id: "vivy.a2a"}
	bound, err = BindChannelsWithModuleIDs([]channel.ChannelProvider{noTask}, grants, nil, moduleIDs)
	if err != nil || len(bound) != 1 {
		t.Fatalf("bind=%v err=%v", bound, err)
	}
	if err := bound[0].Start(context.Background(), &stubChannelHost{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := noTask.host.(channel.TaskHost); ok {
		t.Fatal("wrapper around a taskless Host must not assert TaskHost")
	}

	// Granted + real task Host -> all five calls and the discovery reader
	// reach the underlying Host, and the Module identity is the sealed
	// Module ID, not the provider display ID.
	authorized := &capturingProvider{id: "vivy.a2a"}
	bound, err = BindChannelsWithModuleIDs([]channel.ChannelProvider{authorized}, grants, nil, moduleIDs)
	if err != nil || len(bound) != 1 {
		t.Fatalf("bind=%v err=%v", bound, err)
	}
	underlying := newTaskStubHost()
	if err := bound[0].Start(context.Background(), underlying); err != nil {
		t.Fatal(err)
	}
	if got := authorized.host.ModuleID(); got != "projectvivy/a2a-server" {
		t.Fatalf("ModuleID = %q, want the sealed Module ID", got)
	}
	tasks, ok := authorized.host.(channel.TaskHost)
	if !ok {
		t.Fatal("granted wrapper must assert TaskHost")
	}
	if _, err := tasks.SubmitTask(context.Background(), channel.TaskRequest{MessageID: "m1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.GetTask(context.Background(), channel.TaskQuery{TaskID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.ListTasks(context.Background(), channel.TaskListQuery{}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.CancelTask(context.Background(), channel.TaskQuery{TaskID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.SubscribeTask(context.Background(), channel.TaskSubscription{TaskID: "t1"}); err != nil {
		t.Fatal(err)
	}
	info, ok := authorized.host.(channel.TaskServiceInfoHost)
	if !ok {
		t.Fatal("granted wrapper must assert TaskServiceInfoHost")
	}
	if _, err := info.TaskServiceInfo(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, call := range []string{"submit", "get", "list", "cancel", "subscribe", "info"} {
		if underlying.calls[call] != 1 {
			t.Fatalf("underlying calls[%q] = %d, want 1", call, underlying.calls[call])
		}
	}

	// A missing provider -> module identity map entry is a bind error.
	if _, err := BindChannelsWithModuleIDs([]channel.ChannelProvider{&capturingProvider{id: "vivy.a2a"}}, grants, nil, map[string]string{}); err == nil || !strings.Contains(err.Error(), "module identity") {
		t.Fatalf("missing module identity err=%v", err)
	}
	// An entry that names no sealed Module is equally unusable.
	if _, err := BindChannelsWithModuleIDs([]channel.ChannelProvider{&capturingProvider{id: "vivy.a2a"}}, grants, nil, map[string]string{"vivy.a2a": ""}); err == nil || !strings.Contains(err.Error(), "module identity") {
		t.Fatalf("empty module identity err=%v", err)
	}
}

// TestChannelTaskHostTypedNil: a typed-nil task Host must never be invoked —
// assertions succeed without touching the nil receiver.
func TestChannelTaskHostTypedNil(t *testing.T) {
	grants := map[string][]module.GrantBinding{"vivy.a2a": {{Name: module.GrantChannelA2A}}}
	moduleIDs := map[string]string{"vivy.a2a": "projectvivy/a2a-server"}
	provider := &capturingProvider{id: "vivy.a2a"}
	bound, err := BindChannelsWithModuleIDs([]channel.ChannelProvider{provider}, grants, nil, moduleIDs)
	if err != nil || len(bound) != 1 {
		t.Fatalf("bind=%v err=%v", bound, err)
	}
	var typedNil *taskStubHost // nil pointer: asserts TaskHost but would panic on calls
	if err := bound[0].Start(context.Background(), typedNil); err != nil {
		t.Fatal(err)
	}
	if _, ok := provider.host.(channel.TaskHost); ok {
		t.Fatal("typed-nil Host must not assert TaskHost")
	}
	if _, ok := provider.host.(channel.TaskServiceInfoHost); ok {
		t.Fatal("typed-nil Host must not assert TaskServiceInfoHost")
	}
}

// TestValidateRuntimeAssemblyRejectsChannelModuleIdentityGaps: every
// compiled Channel needs exactly one sealed Module identity.
func TestValidateRuntimeAssemblyRejectsChannelModuleIdentityGaps(t *testing.T) {
	base := genassembly.RuntimeAssembly{
		Channels:        []channel.ChannelProvider{stubChannelProvider{id: "vivy.fake"}},
		ChannelGrants:   map[string][]module.GrantBinding{"vivy.fake": {{Name: module.GrantChannelPoll}}},
		ToolWorldGrants: map[string][]module.GrantBinding{},
		Manifest: generation.Manifest{
			Modules: []string{"fixture/fake"}, Channels: []string{"fake"},
			Face: "kernel-headless", NetworkStates: map[string]generation.CapabilityState{},
		},
	}

	// Missing entry.
	missing := base
	missing.ChannelModuleIDs = map[string]string{}
	if err := validateRuntimeAssembly(missing); err == nil || !strings.Contains(err.Error(), "module identity") {
		t.Fatalf("missing ChannelModuleIDs err=%v", err)
	}

	// Identity that names an unsealed Module.
	unsealed := base
	unsealed.ChannelModuleIDs = map[string]string{"vivy.fake": "fixture/elsewhere"}
	if err := validateRuntimeAssembly(unsealed); err == nil || !strings.Contains(err.Error(), "module identity") {
		t.Fatalf("unsealed module identity err=%v", err)
	}

	// Extra entry beyond the compiled providers.
	extra := base
	extra.ChannelModuleIDs = map[string]string{"vivy.fake": "fixture/fake", "vivy.ghost": "fixture/fake"}
	if err := validateRuntimeAssembly(extra); err == nil || !strings.Contains(err.Error(), "module identity") {
		t.Fatalf("extra module identity err=%v", err)
	}

	// Exact mapping passes.
	ok := base
	ok.ChannelModuleIDs = map[string]string{"vivy.fake": "fixture/fake"}
	if err := validateRuntimeAssembly(ok); err != nil {
		t.Fatalf("exact module identity rejected: %v", err)
	}
}
