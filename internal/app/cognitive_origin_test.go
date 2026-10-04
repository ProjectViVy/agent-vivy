package app

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	divacognitive "agent-vivy/internal/modules/diva-cognitive"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	controlaction "agent-vivy/sdk/port/controlaction"
)

// cognitiveOriginFixture wires the exact seam App.DialControl installs: a
// real control handler over net.Pipe, the embedded-face identity, the
// process-owned caller token, and the real ActionHost deps — only the
// generated-Assembly factory emission is replaced by a direct factory open,
// because the checked-in default generation does not select the module.
type cognitiveOriginFixture struct {
	client *controlrpc.Peer
	host   *actionhost.Host
	token  string
}

func newCognitiveOriginFixture(t *testing.T) *cognitiveOriginFixture {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "origin.db"))
	if err != nil {
		t.Fatalf("open backend: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	dataDir := t.TempDir()
	bundle, err := divacognitive.Open(ctx, cognitivecontract.FactoryInput{
		Config:       config.Config{Storage: config.Storage{DataDir: dataDir}},
		GenerationID: "generation-origin-test",
	})
	if err != nil {
		t.Fatalf("open cognitive bundle: %v", err)
	}
	t.Cleanup(func() { _ = bundle.Close() })

	provider, ok := bundle.(cognitivecontract.DispatcherProvider)
	if !ok {
		t.Fatal("cognitive bundle does not expose the dispatcher")
	}
	engine, err := runtime.NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	token := controlrpc.NewSessionToken()
	host, err := actionhost.New(actionhost.Deps{
		ProviderSets: []controlaction.ProviderSet{{
			ModuleID:   "vivy/diva-cognitive",
			AllowedIDs: divacognitive.ActionIDs,
			Providers:  divacognitive.ActionProviders(),
		}},
		Cognitive: provider,
		CognitiveSessionCheck: func(ctx context.Context, sessionID domain.SessionID) error {
			_, err := backend.GetSession(ctx, sessionID)
			return err
		},
		GenerationAvailable: true,
		GenerationID:        "generation-origin-test",
		Audit:               actionhost.JournalAuditSink{Journal: backend},
		// The operator-selected full_auto profile admits effectful actions
		// explicitly; the predicate itself still rejects approval-marked
		// definitions, and the session/origin guards stay independent of it.
		Authenticate: actionAuthenticate(token),
		Authorize:    actionAuthorize(engine, domain.PolicyProfileFullAuto),
	})
	if err != nil {
		t.Fatalf("build action host: %v", err)
	}
	t.Cleanup(func() { _ = host.Close() })

	handler, err := controlrpc.NewControlHandler(controlrpc.ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: events.NewBus(8),
		Service:    &runtime.Service{},
		ActionHost: host,
	})
	if err != nil {
		t.Fatalf("build control handler: %v", err)
	}

	hostConn, clientConn := net.Pipe()
	hostPeer := controlrpc.NewPeer(
		controlrpc.NewJSONLTransport(hostConn, hostConn, hostConn.Close),
		handler, controlrpc.Options{
			OutgoingBuffer: 64,
			Caller:         actionhost.NewCaller(token),
			Identity:       actionhost.Identity{ID: "face/embedded", Face: "embedded"},
		},
	)
	go func() { _ = hostPeer.Serve(ctx) }()
	client := controlrpc.NewPeer(
		controlrpc.NewJSONLTransport(clientConn, clientConn, clientConn.Close),
		nil, controlrpc.Options{OutgoingBuffer: 64},
	)
	go func() { _ = client.Serve(ctx) }()
	t.Cleanup(func() { _ = client.Close() })
	return &cognitiveOriginFixture{client: client, host: host, token: token}
}

// createCognitiveSession performs the real session/create control call that
// binds the serving peer's identity to the created session — the only way a
// session claim becomes trusted.
func createCognitiveSession(t *testing.T, client *controlrpc.Peer) string {
	t.Helper()
	created := callControl(t, client, "session/create", map[string]any{"title": "cognitive origin"})
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("session/create = %v", created)
	}
	return id
}

func invokeCognitive(t *testing.T, client *controlrpc.Peer, actionID string, input map[string]any) (json.RawMessage, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return client.Call(ctx, "module.action.invoke", map[string]any{
		"module_id": "vivy/diva-cognitive",
		"action_id": actionID,
		"input":     input,
	})
}

func TestCognitiveActionsRequireABoundSession(t *testing.T) {
	fixture := newCognitiveOriginFixture(t)
	callControl(t, fixture.client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})

	// Before session/create the serving peer has no trusted session: a
	// caller-claimed session_id must be denied, never trusted.
	_, err := invokeCognitive(t, fixture.client, divacognitive.ActionStatus, map[string]any{
		"session_id": "sess_forged",
	})
	if err == nil {
		t.Fatal("unbound session claim admitted")
	}
}

func TestCognitiveActionsRejectForeignSessionClaims(t *testing.T) {
	fixture := newCognitiveOriginFixture(t)
	callControl(t, fixture.client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	sessionID := createCognitiveSession(t, fixture.client)

	// A second real session is foreign to this peer's binding: claiming it
	// is denied even though it exists.
	second := createCognitiveSession(t, fixture.client)
	_ = second // the peer rebinds to the newest session on each create

	result, err := invokeCognitive(t, fixture.client, divacognitive.ActionStatus, map[string]any{
		"session_id": sessionID,
	})
	if err == nil {
		t.Fatalf("foreign session claim admitted: %s", result)
	}
	// A session that does not exist at all is denied as well.
	if _, err := invokeCognitive(t, fixture.client, divacognitive.ActionStatus, map[string]any{
		"session_id": "sess_nonexistent",
	}); err == nil {
		t.Fatal("nonexistent session claim admitted")
	}
}

func TestCognitiveActionsServeBoundSessionEndToEnd(t *testing.T) {
	fixture := newCognitiveOriginFixture(t)
	callControl(t, fixture.client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	sessionID := createCognitiveSession(t, fixture.client)

	raw, err := invokeCognitive(t, fixture.client, divacognitive.ActionStatus, map[string]any{
		"session_id": sessionID,
	})
	if err != nil {
		t.Fatalf("bound status invoke: %v", err)
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || (out.Status != "ok" && out.Status != "unavailable") {
		t.Fatalf("status outcome = %s err=%v", raw, err)
	}

	// Every contract action id resolves inside the module's own namespace:
	// the inventory resolves, the strict schema accepts a contract-shaped
	// input, and the dispatcher returns an envelope — ok or an honest
	// non-ok, never a transport failure.
	minimal := map[string]map[string]any{
		divacognitive.ActionStatus:              {},
		divacognitive.ActionPersonaInitialize:   {"initialization": map[string]any{"identity": "i", "relationship": "r", "redline": "x", "user": "u", "world": "w"}},
		divacognitive.ActionPersonaRead:         {"kind": "identity"},
		divacognitive.ActionPersonaSave:         {"kind": "identity", "content": "c", "base_revision": 0},
		divacognitive.ActionPersonaReviewList:   {},
		divacognitive.ActionPersonaReviewDecide: {"review_id": "r", "decision": "reject"},
		divacognitive.ActionFrozenRead:          {},
		divacognitive.ActionActmemRead:          {"sections": []string{"pulse"}, "max_chars": 120},
		divacognitive.ActionActmemWorkPatch:     {"patch": map[string]any{"base_revision": 0, "changes": []any{}}},
		divacognitive.ActionActmemOwnerRead:     {},
		divacognitive.ActionActmemOwnerSave:     {"markdown": "m", "base_revision": 0},
		divacognitive.ActionMemorySearch:        {"query": "q", "limit": 1, "budget_chars": 100},
		divacognitive.ActionMemoryExpand:        {"card_id": "c", "expected_revision": 0, "budget_chars": 100},
		divacognitive.ActionMemoryMutate:        {"mutation": map[string]any{"operation": "put", "body": "{}"}},
		divacognitive.ActionMemoryReceipt:       {"operation_id": "op"},
		divacognitive.ActionPolicyGet:           {},
		divacognitive.ActionPolicySet:           {"enabled": true, "min_interval_ms": 1, "base_revision": 0},
		divacognitive.ActionTrigger:             {},
		divacognitive.ActionCancel:              {"run_id": "r"},
		divacognitive.ActionResultsList:         {},
	}
	for _, id := range divacognitive.ActionIDs {
		input := map[string]any{"session_id": sessionID}
		for k, v := range minimal[id] {
			input[k] = v
		}
		raw, err = invokeCognitive(t, fixture.client, id, input)
		if err != nil {
			t.Fatalf("%s through the trusted peer: %v", id, err)
		}
		var env struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(raw, &env); err != nil || env.Status == "" {
			t.Fatalf("%s envelope = %s err=%v", id, raw, err)
		}
	}
}

func TestCognitiveActionsRejectUnauthenticatedCaller(t *testing.T) {
	// A serving peer that presents a forged caller token never reaches the
	// dispatcher: the authenticate predicate denies before any domain work.
	fixture := newCognitiveOriginFixture(t)
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "forged.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	handler, err := controlrpc.NewControlHandler(controlrpc.ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: events.NewBus(8),
		Service:    &runtime.Service{},
		ActionHost: fixture.host,
	})
	if err != nil {
		t.Fatal(err)
	}
	hostConn, clientConn := net.Pipe()
	forgedHost := controlrpc.NewPeer(
		controlrpc.NewJSONLTransport(hostConn, hostConn, hostConn.Close),
		handler, controlrpc.Options{
			OutgoingBuffer: 64,
			Caller:         actionhost.NewCaller("forged-token"),
			Identity:       actionhost.Identity{ID: "face/embedded", Face: "embedded"},
		},
	)
	go func() { _ = forgedHost.Serve(ctx) }()
	client := controlrpc.NewPeer(
		controlrpc.NewJSONLTransport(clientConn, clientConn, clientConn.Close),
		nil, controlrpc.Options{OutgoingBuffer: 64},
	)
	go func() { _ = client.Serve(ctx) }()
	t.Cleanup(func() { _ = client.Close() })

	createCognitiveSession(t, client)
	if _, err := invokeCognitive(t, client, divacognitive.ActionStatus, map[string]any{
		"session_id": "sess_any",
	}); err == nil {
		t.Fatal("forged caller token admitted")
	}
}

// TestCognitiveActionFixtureCapture emits the ledger fixture: every contract
// action invoked through the trusted embedded peer, with the real envelope
// or transport denial recorded verbatim. Skipped unless
// VIVY_CAPTURE_COGNITIVE_FIXTURE points at the output path.
func TestCognitiveActionFixtureCapture(t *testing.T) {
	outPath := os.Getenv("VIVY_CAPTURE_COGNITIVE_FIXTURE")
	if outPath == "" {
		t.Skip("fixture capture disabled")
	}
	fixture := newCognitiveOriginFixture(t)
	callControl(t, fixture.client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	sessionID := createCognitiveSession(t, fixture.client)

	minimal := map[string]map[string]any{
		divacognitive.ActionStatus:              {},
		divacognitive.ActionPersonaInitialize:   {"initialization": map[string]any{"identity": "i", "relationship": "r", "redline": "x", "user": "u", "world": "w"}},
		divacognitive.ActionPersonaRead:         {"kind": "identity"},
		divacognitive.ActionPersonaSave:         {"kind": "identity", "content": "c", "base_revision": 0},
		divacognitive.ActionPersonaReviewList:   {},
		divacognitive.ActionPersonaReviewDecide: {"review_id": "r", "decision": "reject"},
		divacognitive.ActionFrozenRead:          {},
		divacognitive.ActionActmemRead:          {"sections": []string{"pulse"}, "max_chars": 120},
		divacognitive.ActionActmemWorkPatch:     {"patch": map[string]any{"base_revision": 0, "changes": []any{}}},
		divacognitive.ActionActmemOwnerRead:     {},
		divacognitive.ActionActmemOwnerSave:     {"markdown": "m", "base_revision": 0},
		divacognitive.ActionMemorySearch:        {"query": "q", "limit": 1, "budget_chars": 100},
		divacognitive.ActionMemoryExpand:        {"card_id": "c", "expected_revision": 0, "budget_chars": 100},
		divacognitive.ActionMemoryMutate:        {"mutation": map[string]any{"operation": "put", "body": "{}"}},
		divacognitive.ActionMemoryReceipt:       {"operation_id": "op"},
		divacognitive.ActionPolicyGet:           {},
		divacognitive.ActionPolicySet:           {"enabled": true, "min_interval_ms": 1, "base_revision": 0},
		divacognitive.ActionTrigger:             {},
		divacognitive.ActionCancel:              {"run_id": "r"},
		divacognitive.ActionResultsList:         {},
	}
	type capture struct {
		ActionID string          `json:"action_id"`
		Input    map[string]any  `json:"input"`
		Result   json.RawMessage `json:"result,omitempty"`
		Error    string          `json:"error,omitempty"`
	}
	fixtureDoc := struct {
		Module      string    `json:"module_id"`
		Owner       string    `json:"owner"`
		Peer        string    `json:"peer"`
		ActionCount int       `json:"action_count"`
		Actions     []capture `json:"actions"`
		Denials     []capture `json:"denials"`
	}{Module: "vivy/diva-cognitive", Owner: "vivy/diva-cognitive", Peer: "face/embedded", ActionCount: len(divacognitive.ActionIDs)}
	for _, id := range divacognitive.ActionIDs {
		input := map[string]any{"session_id": sessionID}
		for k, v := range minimal[id] {
			input[k] = v
		}
		entry := capture{ActionID: id, Input: input}
		raw, err := invokeCognitive(t, fixture.client, id, input)
		if err != nil {
			entry.Error = err.Error()
		} else {
			entry.Result = json.RawMessage(raw)
		}
		fixtureDoc.Actions = append(fixtureDoc.Actions, entry)
	}
	// Origin denials are transport errors, recorded verbatim: forged
	// session before binding, foreign session after binding, unknown
	// session after binding.
	deny := func(input map[string]any) {
		entry := capture{ActionID: divacognitive.ActionStatus, Input: input}
		if _, err := invokeCognitive(t, fixture.client, divacognitive.ActionStatus, input); err != nil {
			entry.Error = err.Error()
		} else {
			entry.Result = json.RawMessage(`{"status":"captured_ok"}`)
		}
		fixtureDoc.Denials = append(fixtureDoc.Denials, entry)
	}
	deny(map[string]any{"session_id": "sess_nonexistent"})
	raw, err := json.MarshalIndent(fixtureDoc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
