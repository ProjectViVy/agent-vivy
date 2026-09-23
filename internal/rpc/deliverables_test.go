package rpc

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

type fakeDeliverableOps struct {
	gotSession domain.SessionID
	page       domain.DeliverySetPage
	set        domain.DeliverySet
	chunk      domain.DeliveryChunk
	closedID   string
	err        error
}

func (f *fakeDeliverableOps) Present(context.Context, domain.PresentRequest) (domain.DeliverySet, error) {
	return f.set, f.err
}

func (f *fakeDeliverableOps) List(ctx context.Context, session domain.SessionID, _ string, _ int) (domain.DeliverySetPage, error) {
	f.gotSession = tools.SessionIDFromContext(ctx)
	if f.gotSession != session {
		return domain.DeliverySetPage{}, runtime.DeliverableError{Status: string(domain.HistoryStatusForbidden), Reason: "list requires the owning session"}
	}
	return f.page, f.err
}

func (f *fakeDeliverableOps) Get(ctx context.Context, session domain.SessionID, _ string) (domain.DeliverySet, error) {
	f.gotSession = tools.SessionIDFromContext(ctx)
	return f.set, f.err
}

func (f *fakeDeliverableOps) Read(ctx context.Context, _ domain.DeliveryReadRequest) (domain.DeliveryChunk, error) {
	f.gotSession = tools.SessionIDFromContext(ctx)
	return f.chunk, f.err
}

func (f *fakeDeliverableOps) CloseTransfer(ctx context.Context, id string) error {
	f.gotSession = tools.SessionIDFromContext(ctx)
	f.closedID = id
	return f.err
}

func newDeliverableTestEnv(t *testing.T, ops *fakeDeliverableOps) *controlTestEnv {
	t.Helper()
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.History = fakeHistoryOps{}
		deps.Deliverables = ops
	})
	if err := env.backend.CreateSession(context.Background(), domain.Session{ID: "B", Title: "owner", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return env
}

func TestDeliverablesAdvertisedAndHiddenWithoutService(t *testing.T) {
	env := newDeliverableTestEnv(t, &fakeDeliverableOps{})
	listed, rpcErr := callControl(t, env.handler, "capabilities", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, _ := json.Marshal(listed)
	var caps struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(listedJSON, &caps); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"deliverables/list", "deliverables/get", "deliverables/read", "deliverables/close"} {
		found := false
		for _, capability := range caps.Capabilities {
			if capability == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("capabilities missing %s: %v", want, caps.Capabilities)
		}
	}

	bare := newControlTestEnv(t, func(deps *ControlDeps) { deps.History = fakeHistoryOps{} })
	listed, rpcErr = callControl(t, bare.handler, "capabilities", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, _ = json.Marshal(listed)
	if err := json.Unmarshal(listedJSON, &caps); err != nil {
		t.Fatal(err)
	}
	for _, capability := range caps.Capabilities {
		if strings.HasPrefix(capability, "deliverables/") {
			t.Fatalf("unwired deliverables advertised %s", capability)
		}
	}
	if _, rpcErr := callControl(t, bare.handler, "deliverables/list", map[string]any{"session_id": "B"}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unwired list error = %v, want MethodNotFound", rpcErr)
	}
}

func TestDeliverablesListGetCloseRoundtrip(t *testing.T) {
	ops := &fakeDeliverableOps{
		page: domain.DeliverySetPage{Items: []domain.DeliverySet{{ID: "dvs_1", Status: domain.DeliveryStatusOK}}, NextCursor: "c1"},
		set:  domain.DeliverySet{ID: "dvs_1", Status: domain.DeliveryStatusOK},
	}
	env := newDeliverableTestEnv(t, ops)

	result, rpcErr := callControl(t, env.handler, "deliverables/list", map[string]any{"session_id": "B", "limit": 5})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	resultJSON, _ := json.Marshal(result)
	var page domain.DeliverySetPage
	if err := json.Unmarshal(resultJSON, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "dvs_1" || page.NextCursor != "c1" {
		t.Fatalf("page = %#v", page)
	}
	if ops.gotSession != "B" {
		t.Fatalf("trusted session = %q, want B", ops.gotSession)
	}

	result, rpcErr = callControl(t, env.handler, "deliverables/get", map[string]any{"session_id": "B", "set_id": "dvs_1"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	resultJSON, _ = json.Marshal(result)
	var set domain.DeliverySet
	if err := json.Unmarshal(resultJSON, &set); err != nil {
		t.Fatal(err)
	}
	if set.ID != "dvs_1" {
		t.Fatalf("set = %#v", set)
	}

	result, rpcErr = callControl(t, env.handler, "deliverables/close", map[string]any{"session_id": "B", "transfer_id": "xfr_1"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if ops.closedID != "xfr_1" {
		t.Fatalf("closed = %q", ops.closedID)
	}
}

// The read path survives a real JSONL transport hop: request bytes over a
// net.Pipe, peer dispatch, verified chunk back.
func TestDeliverablesReadOverTransport(t *testing.T) {
	ops := &fakeDeliverableOps{chunk: domain.DeliveryChunk{
		TransferID: "xfr_1", ItemID: "dvl_1", Digest: "aa", Offset: 0,
		DataBase64: "YWxwaGE=", EOF: true, ExpiresAt: 42,
	}}
	env := newDeliverableTestEnv(t, ops)

	left, right := net.Pipe()
	defer right.Close()
	server := NewPeer(NewJSONLTransport(left, left, left.Close), env.handler, Options{})
	defer server.Close()
	go func() { _ = server.Serve(context.Background()) }()

	client := NewJSONLTransport(right, right, right.Close)
	request, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "r1", "method": "deliverables/read",
		"params": map[string]any{
			"session_id": "B", "item_id": "dvl_1", "expected_digest": "aa", "length": 8,
		},
	})
	if err := client.WriteFrame(request); err != nil {
		t.Fatalf("write: %v", err)
	}
	frame, err := client.ReadFrame()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var response struct {
		Result json.RawMessage `json:"result"`
		Error  *Error          `json:"error"`
	}
	if err := json.Unmarshal(frame, &response); err != nil {
		t.Fatalf("response: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("rpc error: %v", response.Error)
	}
	var chunk domain.DeliveryChunk
	if err := json.Unmarshal(response.Result, &chunk); err != nil {
		t.Fatalf("chunk: %v", err)
	}
	if chunk.DataBase64 != "YWxwaGE=" || !chunk.EOF || chunk.Digest != "aa" {
		t.Fatalf("chunk = %#v", chunk)
	}
	if ops.gotSession != "B" {
		t.Fatalf("trusted session = %q, want B", ops.gotSession)
	}
}

func TestDeliverablesReadErrorKeepsStableReason(t *testing.T) {
	ops := &fakeDeliverableOps{err: runtime.DeliverableError{Status: string(domain.HistoryStatusConflict), Reason: "file changed since presentation"}}
	env := newDeliverableTestEnv(t, ops)
	_, rpcErr := callControl(t, env.handler, "deliverables/read", map[string]any{
		"session_id": "B", "item_id": "dvl_1", "expected_digest": "aa", "length": 4,
	})
	if rpcErr == nil || rpcErr.Code != InvalidParams || !strings.Contains(rpcErr.Message, "file changed since presentation") {
		t.Fatalf("error = %v, want stable changed reason", rpcErr)
	}
}

func TestDeliverablesRejectSpoofedUnknownAndForeign(t *testing.T) {
	ops := &fakeDeliverableOps{}
	env := newDeliverableTestEnv(t, ops)
	for _, tc := range []struct {
		method string
		params map[string]any
	}{
		{"deliverables/list", map[string]any{"session_id": "B", "actor": "browser"}},
		{"deliverables/list", map[string]any{"session_id": "B", "bogus": 1}},
		{"deliverables/get", map[string]any{"session_id": "B", "set_id": "dvs_1", "identity": "root"}},
		{"deliverables/read", map[string]any{"session_id": "B", "item_id": "dvl_1", "expected_digest": "aa", "length": 4, "workspace_id": "/etc"}},
		{"deliverables/close", map[string]any{"session_id": "B", "transfer_id": "xfr_1", "caller": "root"}},
		{"deliverables/get", map[string]any{"session_id": "ghost", "set_id": "dvs_1"}},
	} {
		if _, rpcErr := callControl(t, env.handler, tc.method, tc.params); rpcErr == nil {
			t.Fatalf("%s params %v succeeded", tc.method, tc.params)
		}
	}
	if ops.gotSession == "ghost" {
		t.Fatal("ghost session reached the service")
	}
}

// A set owned by another session is invisible: the authority context binds
// the caller's session and the service enforces the ownership check itself.
func TestDeliverablesForeignSetReturnsForbidden(t *testing.T) {
	ops := &fakeDeliverableOps{err: runtime.DeliverableError{Status: string(domain.HistoryStatusForbidden), Reason: "set requires the owning session"}}
	env := newDeliverableTestEnv(t, ops)
	_, rpcErr := callControl(t, env.handler, "deliverables/get", map[string]any{"session_id": "B", "set_id": "dvs_other"})
	if rpcErr == nil || rpcErr.Code != InvalidParams || !strings.Contains(rpcErr.Message, string(domain.HistoryStatusForbidden)) {
		t.Fatalf("error = %v, want forbidden envelope", rpcErr)
	}
}

var _ = storage.ErrNotFound // keep storage import used when only fake ops are exercised
