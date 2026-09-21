package channel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/rpccontract"
	"agent-vivy/internal/storage"
	channelport "agent-vivy/sdk/port/channel"
)

type managementPeer struct{}

func (managementPeer) AuthenticatedCaller() (actionhost.Caller, bool) {
	return actionhost.Caller{}, false
}
func (managementPeer) AfterResponse(json.RawMessage, func()) {}
func (managementPeer) Call(context.Context, string, any) (json.RawMessage, error) {
	return nil, nil
}
func (managementPeer) Notify(string, any) error                         { return nil }
func (managementPeer) NotifyContext(context.Context, string, any) error { return nil }

type settingsProbe struct {
	doc         channelcontract.Settings
	writable    bool
	frozen      bool
	updateErr   error
	updateCalls int
}

type managementInstance struct {
	mu          sync.Mutex
	sent        []channelport.OutboundMessage
	sendStarted chan struct{}
	releaseSend chan struct{}
	startOnce   sync.Once
}

func (*managementInstance) Start(context.Context) error { return nil }
func (*managementInstance) Stop(context.Context) error  { return nil }

func (instance *managementInstance) Send(ctx context.Context, message channelport.OutboundMessage) ([]string, error) {
	instance.mu.Lock()
	instance.sent = append(instance.sent, message)
	instance.mu.Unlock()
	if instance.sendStarted != nil {
		instance.startOnce.Do(func() { close(instance.sendStarted) })
	}
	if instance.releaseSend != nil {
		select {
		case <-instance.releaseSend:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []string{"management-message-1"}, nil
}

func (instance *managementInstance) snapshot() []channelport.OutboundMessage {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	return append([]channelport.OutboundMessage(nil), instance.sent...)
}

type managementProvider struct {
	instance *managementInstance
}

func (*managementProvider) Definition() channelport.Definition {
	return channelport.Definition{ID: "vivy.fake"}
}

func (provider *managementProvider) Construct(context.Context, channelport.Host) (channelport.Instance, error) {
	return provider.instance, nil
}

type healthManagementInstance struct {
	*managementInstance
	err error
}

func (instance *healthManagementInstance) Health(context.Context) error { return instance.err }

type healthManagementProvider struct {
	instance *healthManagementInstance
}

func (*healthManagementProvider) Definition() channelport.Definition {
	return channelport.Definition{ID: "vivy.fake"}
}

func (provider *healthManagementProvider) Construct(context.Context, channelport.Host) (channelport.Instance, error) {
	return provider.instance, nil
}

func (provider *healthManagementProvider) CapabilityTarget() any { return provider.instance }

type failingDeliveryStore struct {
	storage.ChannelDeliveryStore
	err error
}

func (store failingDeliveryStore) ListFailedChannelDeliveries(context.Context) ([]storage.ChannelDelivery, error) {
	return nil, store.err
}

func (p *settingsProbe) Read(context.Context) (channelcontract.Settings, error) {
	return cloneSettings(p.doc), nil
}

func (p *settingsProbe) Update(_ context.Context, overlay channelcontract.ChannelOverlay) (channelcontract.Settings, error) {
	p.updateCalls++
	if p.updateErr != nil {
		return channelcontract.Settings{}, p.updateErr
	}
	found := false
	for i := range p.doc.Channels {
		if p.doc.Channels[i].Name == overlay.Name {
			p.doc.Channels[i] = overlay
			found = true
			break
		}
	}
	if !found {
		p.doc.Channels = append(p.doc.Channels, overlay)
	}
	return cloneSettings(p.doc), nil
}

func (p *settingsProbe) Writable() bool { return p.writable }
func (p *settingsProbe) Frozen() bool   { return p.frozen }

func cloneSettings(input channelcontract.Settings) channelcontract.Settings {
	return channelcontract.Settings{Channels: append([]channelcontract.ChannelOverlay(nil), input.Channels...)}
}

func managementOwner(t *testing.T, settings *settingsProbe, changed *int, configured bool) channelcontract.Owned {
	t.Helper()
	deps := validDeps(t)
	deps.Settings = settings
	deps.OnSettingsChanged = func() { *changed++ }
	selection := channelcontract.Selection{
		Providers:        []channelport.ChannelProvider{&providerProbe{}},
		ProcessAvailable: false,
	}
	if configured {
		selection.Config = channelcontract.Config{"fake": {
			Enabled: false, AllowFrom: []string{"alice"}, TokenEnv: "VIVY_TEST_FAKE_CHANNEL_TOKEN",
		}}
	}
	owner, err := NewFactory().Construct(context.Background(), deps, selection)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	return owner
}

func managementRuntimeOwner(
	t *testing.T,
	deps channelcontract.Dependencies,
	provider channelport.ChannelProvider,
	processAvailable bool,
) channelcontract.Owned {
	t.Helper()
	owner, err := NewFactory().Construct(context.Background(), deps, channelcontract.Selection{
		Providers: []channelport.ChannelProvider{provider},
		Config: channelcontract.Config{"fake": {
			Enabled: true, AllowFrom: []string{"alice"},
		}},
		ProcessAvailable: processAvailable,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	return owner
}

func managementBinding(t *testing.T, owner channelcontract.Owned, method string) rpccontract.MethodBinding {
	t.Helper()
	for _, item := range owner.RPCBindings() {
		if item.Method == method {
			return item
		}
	}
	t.Fatalf("binding %q missing", method)
	return rpccontract.MethodBinding{}
}

func callManagement(t *testing.T, owner channelcontract.Owned, method string, params any) (any, *rpccontract.Error) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	binding := managementBinding(t, owner, method)
	return binding.Handler(context.Background(), managementPeer{}, rpccontract.Request{Method: method, Params: raw})
}

func inspectManagementRow(t *testing.T, owner channelcontract.Owned) map[string]any {
	t.Helper()
	got, rpcErr := callManagement(t, owner, "channel/inspect", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("inspect rows=%s, want one", raw)
	}
	return rows[0]
}

type managementDeliveryResult struct {
	RunID       string `json:"run_id"`
	SessionID   string `json:"session_id"`
	Channel     string `json:"channel"`
	ChatID      string `json:"chat_id"`
	TopicID     string `json:"topic_id"`
	State       string `json:"state"`
	Attempts    int    `json:"attempts"`
	CreatedAtMs int64  `json:"created_at_ms"`
	UpdatedAtMs int64  `json:"updated_at_ms"`
}

func listManagementDeliveries(t *testing.T, owner channelcontract.Owned) ([]managementDeliveryResult, string) {
	t.Helper()
	got, rpcErr := callManagement(t, owner, "channel/deliveries/list", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Deliveries []managementDeliveryResult `json:"deliveries"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Deliveries, string(raw)
}

func seedManagementDelivery(
	t *testing.T,
	deps channelcontract.Dependencies,
	runID domain.RunID,
	state string,
	attempts int,
	content string,
) {
	t.Helper()
	now := time.Now().UnixMilli()
	if content != "" {
		if err := deps.Messages.AppendMessage(context.Background(), domain.Message{
			ID: "message-" + string(runID), SessionID: "session-management", RunID: runID,
			Role: domain.RoleAssistant, CreatedAt: now, Content: content,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := deps.Deliveries.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: runID, SessionID: "session-management", Channel: "fake", ChatID: "chat-management",
		TopicID: "topic-management", State: state, Attempts: attempts,
		CreatedAtMs: now, UpdatedAtMs: now,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRPCBindingsExposeExactManagementSurface(t *testing.T) {
	changed := 0
	owner := managementOwner(t, &settingsProbe{writable: true}, &changed, true)
	bindings := owner.RPCBindings()
	want := map[string]string{
		"channel/inspect":              "channel.inspect",
		"channel/get":                  "channel.get",
		"channel/update":               "channel.update",
		"channel/deliveries/list":      "channel.deliveries.list",
		"channel/deliveries/redeliver": "channel.deliveries.redeliver",
	}
	if len(bindings) != len(want) {
		t.Fatalf("bindings=%#v", bindings)
	}
	for _, binding := range bindings {
		if want[binding.Method] != binding.Capability || binding.Handler == nil {
			t.Fatalf("binding=%#v", binding)
		}
	}
	bindings[0].Method = "mutated"
	if owner.RPCBindings()[0].Method == "mutated" {
		t.Fatal("RPCBindings returned mutable owner state")
	}
}

func TestManagementHealthProjection(t *testing.T) {
	t.Run("not started", func(t *testing.T) {
		deps := validDeps(t)
		provider := &healthManagementProvider{instance: &healthManagementInstance{managementInstance: &managementInstance{}}}
		owner := managementRuntimeOwner(t, deps, provider, true)
		row := inspectManagementRow(t, owner)
		if health, ok := row["health"]; !ok || health != nil {
			t.Fatalf("health=%#v present=%v, want explicit null", health, ok)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		deps := validDeps(t)
		provider := &managementProvider{instance: &managementInstance{}}
		owner := managementRuntimeOwner(t, deps, provider, true)
		if err := owner.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		row := inspectManagementRow(t, owner)
		if health, ok := row["health"]; !ok || health != nil {
			t.Fatalf("health=%#v present=%v, want explicit null", health, ok)
		}
	})

	for _, test := range []struct {
		class  channelport.ErrorClass
		detail string
	}{
		{class: channelport.ClassRateLimit, detail: "retry window"},
		{class: channelport.ClassTemporary, detail: "socket reset"},
		{class: channelport.ClassDead, detail: "token revoked"},
	} {
		t.Run(string(test.class), func(t *testing.T) {
			deps := validDeps(t)
			provider := &healthManagementProvider{instance: &healthManagementInstance{
				managementInstance: &managementInstance{},
				err:                &channelport.HealthError{Class: test.class, Err: errors.New(test.detail)},
			}}
			owner := managementRuntimeOwner(t, deps, provider, true)
			if err := owner.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			row := inspectManagementRow(t, owner)
			health, ok := row["health"].(map[string]any)
			if !ok || health["ok"] != false || health["class"] != string(test.class) || health["detail"] != test.detail {
				t.Fatalf("health=%#v, want class %q detail %q", row["health"], test.class, test.detail)
			}
		})
	}
}

func TestManagementDeliveriesListFailedIdentifiersOnly(t *testing.T) {
	deps := validDeps(t)
	provider := &managementProvider{instance: &managementInstance{}}
	owner := managementRuntimeOwner(t, deps, provider, true)
	failedRun := domain.RunID("run-management-failed")
	seedManagementDelivery(t, deps, failedRun, storage.ChannelDeliveryFailed, 3, "reply content must stay in the message log")
	seedManagementDelivery(t, deps, "run-management-armed", storage.ChannelDeliveryArmed, 0, "")

	deliveries, raw := listManagementDeliveries(t, owner)
	if len(deliveries) != 1 {
		t.Fatalf("deliveries=%s, want one failed row", raw)
	}
	delivery := deliveries[0]
	if delivery.RunID != string(failedRun) || delivery.SessionID != "session-management" ||
		delivery.Channel != "fake" || delivery.ChatID != "chat-management" || delivery.TopicID != "topic-management" ||
		delivery.State != storage.ChannelDeliveryFailed || delivery.Attempts != 3 ||
		delivery.CreatedAtMs == 0 || delivery.UpdatedAtMs == 0 {
		t.Fatalf("delivery=%+v, want the seeded failed identifiers and state", delivery)
	}
	if strings.Contains(raw, "reply content") || strings.Contains(raw, `"content"`) {
		t.Fatalf("failed-delivery projection exposed reply content: %s", raw)
	}
}

func TestManagementDeliveryRedeliverValidationAndConflicts(t *testing.T) {
	t.Run("missing and unknown run", func(t *testing.T) {
		deps := validDeps(t)
		owner := managementRuntimeOwner(t, deps, &managementProvider{instance: &managementInstance{}}, true)
		if _, rpcErr := callManagement(t, owner, "channel/deliveries/redeliver", map[string]any{}); rpcErr == nil || rpcErr.Code != rpccontract.InvalidParams {
			t.Fatalf("missing run_id error=%v, want InvalidParams", rpcErr)
		}
		if _, rpcErr := callManagement(t, owner, "channel/deliveries/redeliver", map[string]any{"run_id": "run-unknown"}); rpcErr == nil || rpcErr.Code != rpccontract.CodeNotFound || rpcErr.Message != "failed delivery not found" {
			t.Fatalf("unknown run error=%v, want CodeNotFound", rpcErr)
		}
	})

	t.Run("inactive provider", func(t *testing.T) {
		deps := validDeps(t)
		owner := managementRuntimeOwner(t, deps, &managementProvider{instance: &managementInstance{}}, true)
		seedManagementDelivery(t, deps, "run-inactive", storage.ChannelDeliveryFailed, 3, "inactive reply")
		if _, rpcErr := callManagement(t, owner, "channel/deliveries/redeliver", map[string]any{"run_id": "run-inactive"}); rpcErr == nil || rpcErr.Code != rpccontract.CodeConflict || rpcErr.Message != "channel delivery is unavailable" {
			t.Fatalf("inactive provider error=%v, want CodeConflict", rpcErr)
		}
	})

	t.Run("draining provider", func(t *testing.T) {
		deps := validDeps(t)
		owner := managementRuntimeOwner(t, deps, &managementProvider{instance: &managementInstance{}}, true)
		seedManagementDelivery(t, deps, "run-draining", storage.ChannelDeliveryFailed, 3, "draining reply")
		if err := owner.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := owner.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, rpcErr := callManagement(t, owner, "channel/deliveries/redeliver", map[string]any{"run_id": "run-draining"}); rpcErr == nil || rpcErr.Code != rpccontract.CodeConflict || rpcErr.Message != "channel delivery is unavailable" {
			t.Fatalf("draining provider error=%v, want CodeConflict", rpcErr)
		}
	})
}

func TestManagementDeliveriesRedactStoreFailures(t *testing.T) {
	deps := validDeps(t)
	deps.Deliveries = failingDeliveryStore{ChannelDeliveryStore: deps.Deliveries, err: errors.New("database is not running at private path")}
	owner := managementRuntimeOwner(t, deps, &managementProvider{instance: &managementInstance{}}, true)
	if _, rpcErr := callManagement(t, owner, "channel/deliveries/list", nil); rpcErr == nil || rpcErr.Code != rpccontract.InternalError || rpcErr.Message != "internal error" {
		t.Fatalf("list store error=%v, want redacted InternalError", rpcErr)
	}
	if _, rpcErr := callManagement(t, owner, "channel/deliveries/redeliver", map[string]any{"run_id": "run-private"}); rpcErr == nil || rpcErr.Code != rpccontract.InternalError || rpcErr.Message != "internal error" {
		t.Fatalf("redeliver store error=%v, want redacted InternalError", rpcErr)
	}
}

func TestManagementDeliveryRedeliverRearmsSendsOnceAndClearsFailure(t *testing.T) {
	deps := validDeps(t)
	instance := &managementInstance{sendStarted: make(chan struct{}), releaseSend: make(chan struct{})}
	owner := managementRuntimeOwner(t, deps, &managementProvider{instance: instance}, true)
	if err := owner.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	runID := domain.RunID("run-management-redeliver")
	seedManagementDelivery(t, deps, runID, storage.ChannelDeliveryFailed, 3, "redelivered reply")

	got, rpcErr := callManagement(t, owner, "channel/deliveries/redeliver", map[string]any{"run_id": string(runID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, ok := got.(map[string]any)
	if !ok || result["run_id"] != string(runID) || result["redelivered"] != true {
		t.Fatalf("redeliver result=%#v", got)
	}
	select {
	case <-instance.sendStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("redelivered intent never reached the Provider")
	}
	if sent := instance.snapshot(); len(sent) != 1 {
		t.Fatalf("sent=%+v, want exactly one redelivered message", sent)
	}
	open, err := deps.Deliveries.ListOpenChannelDeliveries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].RunID != runID || open[0].State != storage.ChannelDeliveryPending {
		t.Fatalf("open deliveries=%+v, want re-armed pending intent", open)
	}
	close(instance.releaseSend)

	deadline := time.Now().Add(2 * time.Second)
	for {
		open, err = deps.Deliveries.ListOpenChannelDeliveries(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		failed, err := deps.Deliveries.ListFailedChannelDeliveries(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(open) == 0 && len(failed) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("delivery row remained after successful send: open=%+v failed=%+v", open, failed)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if sent := instance.snapshot(); len(sent) != 1 {
		t.Fatalf("sent after completed redelivery=%+v, want exactly one redelivered message", sent)
	}
	if deliveries, raw := listManagementDeliveries(t, owner); len(deliveries) != 0 {
		t.Fatalf("failed listing after successful redelivery=%s", raw)
	}
}

func TestManagementInspectAndGetPreserveWireShape(t *testing.T) {
	changed := 0
	owner := managementOwner(t, &settingsProbe{writable: true}, &changed, true)
	got, rpcErr := callManagement(t, owner, "channel/inspect", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(encoded, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["name"] != "fake" || rows[0]["allow_from"] == nil || rows[0]["capabilities"] == nil {
		t.Fatalf("inspect wire=%s", encoded)
	}

	got, rpcErr = callManagement(t, owner, "channel/get", map[string]any{"name": "fake"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	envelope := got.(channelEnvelopeResult)
	if envelope.Name != "fake" || envelope.Enabled || !envelope.Configured || len(envelope.AllowFrom) != 1 || envelope.AllowFrom[0] != "alice" {
		t.Fatalf("get envelope=%+v", envelope)
	}
	if _, rpcErr = callManagement(t, owner, "channel/get", map[string]any{"name": "ghost"}); rpcErr == nil || rpcErr.Code != rpccontract.CodeNotFound {
		t.Fatalf("unknown get error=%v", rpcErr)
	}
}

func TestManagementUpdateFoldsOverlayAndNotifiesOnce(t *testing.T) {
	changed := 0
	settings := &settingsProbe{writable: true}
	owner := managementOwner(t, settings, &changed, true)
	got, rpcErr := callManagement(t, owner, "channel/update", map[string]any{
		"name": "fake", "enabled": true, "allow_from": []string{"bob", "carol"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	envelope := got.(channelEnvelopeResult)
	if !envelope.Enabled || len(envelope.AllowFrom) != 2 || envelope.TokenEnv != "VIVY_TEST_FAKE_CHANNEL_TOKEN" {
		t.Fatalf("update envelope=%+v", envelope)
	}
	if settings.updateCalls != 1 || changed != 1 {
		t.Fatalf("updates=%d changed=%d, want 1/1", settings.updateCalls, changed)
	}

	addChanged := 0
	addOwner := managementOwner(t, &settingsProbe{writable: true}, &addChanged, false)
	got, rpcErr = callManagement(t, addOwner, "channel/update", map[string]any{
		"name": "fake", "enabled": true, "allow_from": []string{"carol"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if envelope = got.(channelEnvelopeResult); !envelope.Configured || !envelope.Enabled || len(envelope.AllowFrom) != 1 {
		t.Fatalf("overlay-only envelope=%+v", envelope)
	}
}

func TestManagementUpdateGuardsAndFailureNotification(t *testing.T) {
	tests := []struct {
		name     string
		settings *settingsProbe
		params   any
		code     int
	}{
		{name: "read only", settings: &settingsProbe{}, params: map[string]any{"name": "fake", "enabled": true}, code: rpccontract.CodeConflict},
		{name: "frozen", settings: &settingsProbe{writable: true, frozen: true}, params: map[string]any{"name": "fake", "enabled": true}, code: rpccontract.CodeConflict},
		{name: "unknown", settings: &settingsProbe{writable: true}, params: map[string]any{"name": "ghost", "enabled": true}, code: rpccontract.InvalidParams},
		{name: "wildcard validation", settings: &settingsProbe{writable: true, updateErr: channelcontract.MarkInvalidSettings(errors.New("allow_from wildcard is forbidden"))}, params: map[string]any{"name": "fake", "allow_from": []string{"*"}}, code: rpccontract.InvalidParams},
		{name: "token validation", settings: &settingsProbe{writable: true, updateErr: channelcontract.MarkInvalidSettings(errors.New("invalid token_env"))}, params: map[string]any{"name": "fake", "token_env": "not-an-env"}, code: rpccontract.InvalidParams},
		{name: "backend", settings: &settingsProbe{writable: true, updateErr: errors.New("disk path secret")}, params: map[string]any{"name": "fake", "enabled": true}, code: rpccontract.InternalError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := 0
			owner := managementOwner(t, test.settings, &changed, true)
			if _, rpcErr := callManagement(t, owner, "channel/update", test.params); rpcErr == nil || rpcErr.Code != test.code {
				t.Fatalf("rpc error=%v, want code %d", rpcErr, test.code)
			} else if test.name == "backend" && rpcErr.Message != "internal error" {
				t.Fatalf("backend message=%q, want redacted internal error", rpcErr.Message)
			}
			if changed != 0 {
				t.Fatalf("notification count=%d after failure", changed)
			}
		})
	}
}

func TestManagementRejectsMalformedParams(t *testing.T) {
	changed := 0
	owner := managementOwner(t, &settingsProbe{writable: true}, &changed, true)
	binding := managementBinding(t, owner, "channel/update")
	_, rpcErr := binding.Handler(context.Background(), managementPeer{}, rpccontract.Request{
		Method: "channel/update", Params: json.RawMessage(`{"name":`),
	})
	if rpcErr == nil || rpcErr.Code != rpccontract.InvalidParams || rpcErr.Message != "params must be a valid JSON object" {
		t.Fatalf("malformed params error=%v", rpcErr)
	}
	if changed != 0 {
		t.Fatalf("notification count=%d after malformed params", changed)
	}
}
