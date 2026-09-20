package channel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/rpccontract"
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

func TestRPCBindingsExposeExactManagementSurface(t *testing.T) {
	changed := 0
	owner := managementOwner(t, &settingsProbe{writable: true}, &changed, true)
	bindings := owner.RPCBindings()
	want := map[string]string{
		"channel/inspect": "channel.inspect",
		"channel/get":     "channel.get",
		"channel/update":  "channel.update",
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
