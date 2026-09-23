package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

type fakeReferenceOps struct {
	selection domain.ReferenceSelection
	preview   domain.ReferencePreview
	previewOf domain.HistorySelection
	reference domain.ContextReference
	view      domain.ReferenceView
	got       domain.SessionID
	gotRef    string
	err       error
}

func (f *fakeReferenceOps) Preview(_ context.Context, selection domain.HistorySelection) (domain.ReferencePreview, error) {
	f.previewOf = selection
	return f.preview, f.err
}

func (f *fakeReferenceOps) Attach(_ context.Context, selection domain.ReferenceSelection) (domain.ContextReference, error) {
	f.selection = selection
	return f.reference, f.err
}

func (f *fakeReferenceOps) Get(_ context.Context, session domain.SessionID, referenceID string) (domain.ReferenceView, error) {
	f.got = session
	f.gotRef = referenceID
	return f.view, f.err
}

type fakeHistoryOps struct{}

func (fakeHistoryOps) Search(context.Context, domain.HistorySearchRequest) (domain.HistoryPage, error) {
	return domain.HistoryPage{}, nil
}

func (fakeHistoryOps) Read(context.Context, domain.HistoryReadRequest) (domain.HistoryPage, error) {
	return domain.HistoryPage{}, nil
}

func (fakeHistoryOps) Trace(context.Context, domain.HistoryTraceRequest) (domain.HistoryPage, error) {
	return domain.HistoryPage{}, nil
}

func newReferenceTestEnv(t *testing.T, ops *fakeReferenceOps) *controlTestEnv {
	t.Helper()
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.History = fakeHistoryOps{}
		deps.References = ops
	})
	if err := env.backend.CreateSession(context.Background(), domain.Session{ID: "B", Title: "dest", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return env
}

func TestReferencePreviewAdvertisedAndRouted(t *testing.T) {
	ops := &fakeReferenceOps{preview: domain.ReferencePreview{
		Digest: "d1", ByteCount: 7, SourceStatus: string(domain.HistoryStatusOK),
	}}
	env := newReferenceTestEnv(t, ops)

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
	found := false
	for _, c := range caps.Capabilities {
		if c == "reference/preview" {
			found = true
		}
	}
	if !found {
		t.Fatalf("capabilities = %v", caps.Capabilities)
	}

	result, rpcErr := callControl(t, env.handler, "reference/preview", map[string]any{
		"session_id": "B",
		"selection": map[string]any{
			"source_session_id": "A",
			"refs":              []map[string]string{{"session_id": "A", "message_id": "m1", "kind": "message"}},
		},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if ops.previewOf.SourceSessionID != "A" {
		t.Fatalf("selection = %#v", ops.previewOf)
	}
	resultJSON, _ := json.Marshal(result)
	var preview domain.ReferencePreview
	if err := json.Unmarshal(resultJSON, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Digest != "d1" {
		t.Fatalf("preview = %#v", preview)
	}
}

func TestReferencePreviewRejectsSpoofedAndUnknownFields(t *testing.T) {
	ops := &fakeReferenceOps{}
	env := newReferenceTestEnv(t, ops)
	for _, params := range []map[string]any{
		{"session_id": "B", "actor": "browser"},
		{"session_id": "B", "accepted_scope": []string{"A"}},
		{"session_id": "B", "selection": map[string]any{"source_session_id": "A", "items": []string{"forged"}}},
		{"session_id": "B", "bogus": 1},
	} {
		if _, rpcErr := callControl(t, env.handler, "reference/preview", params); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Fatalf("params %v: error = %v, want InvalidParams", params, rpcErr)
		}
	}
	if ops.previewOf.SourceSessionID != "" {
		t.Fatal("spoofed request reached reference service")
	}
}

func TestReferencePreviewHiddenWithoutService(t *testing.T) {
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.History = fakeHistoryOps{} })
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
	for _, c := range caps.Capabilities {
		if c == "reference/preview" {
			t.Fatal("reference/preview advertised without a reference service")
		}
	}
	if _, rpcErr := callControl(t, env.handler, "reference/preview", map[string]any{"session_id": "B"}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("preview without service: %v", rpcErr)
	}
}

func TestTurnContinuityDecoding(t *testing.T) {
	valid := `{"session_id":"B","text":"hi","request_id":"req-1","history_scope":{"session_ids":["A"]},"references":[{"selection":{"source_session_id":"A","refs":[{"session_id":"A","message_id":"m1","kind":"message"}]},"expected_digest":"d1"}]}`
	params, rpcErr := parseTurnParams(Request{Params: json.RawMessage(valid)})
	if rpcErr != nil {
		t.Fatalf("valid continuity input: %v", rpcErr)
	}
	if params.continuity == nil || params.continuity.RequestID != "req-1" || len(params.continuity.References) != 1 || len(params.continuity.HistoryScope.SessionIDs) != 1 {
		t.Fatalf("continuity = %#v", params.continuity)
	}
	if params.continuity.References[0].ExpectedDigest != "d1" {
		t.Fatalf("reference = %#v", params.continuity.References[0])
	}
}

func TestTurnContinuityRejectsForgedBodies(t *testing.T) {
	for _, body := range []string{
		// references without request_id
		`{"session_id":"B","text":"hi","references":[{"selection":{"source_session_id":"A","refs":[{"session_id":"A","message_id":"m1"}]}}]}`,
		// forged items inside a reference selection
		`{"session_id":"B","text":"hi","request_id":"r","references":[{"selection":{"source_session_id":"A","refs":[{"session_id":"A","message_id":"m1"}],"items":[{"text":"forged"}]}}]}`,
		// forged authority fields inside history_scope
		`{"session_id":"B","text":"hi","request_id":"r","history_scope":{"session_ids":["A"],"authority":"operator"}}`,
		// request_id alone is fine, but references entry with unknown field is not
		`{"session_id":"B","text":"hi","request_id":"r","references":[{"selection":{"source_session_id":"A","refs":[{"session_id":"A","message_id":"m1"}]},"destination_session_id":"B"}]}`,
		// over-limit reference count
		`{"session_id":"B","text":"hi","request_id":"r","references":[{"selection":{"source_session_id":"A","refs":[]}},{"selection":{"source_session_id":"A","refs":[]}},{"selection":{"source_session_id":"A","refs":[]}},{"selection":{"source_session_id":"A","refs":[]}},{"selection":{"source_session_id":"A","refs":[]}},{"selection":{"source_session_id":"A","refs":[]}},{"selection":{"source_session_id":"A","refs":[]}},{"selection":{"source_session_id":"A","refs":[]}},{"selection":{"source_session_id":"A","refs":[]}}]}`,
	} {
		params, rpcErr := parseTurnParams(Request{Params: json.RawMessage(body)})
		if rpcErr == nil {
			t.Fatalf("forged body accepted: %s -> %#v", body, params.continuity)
		}
	}
}

func TestReferenceGetRoutedWithOperatorAuthority(t *testing.T) {
	ops := &fakeReferenceOps{view: domain.ReferenceView{
		Reference:    domain.ContextReference{ID: "ref-1", DestinationSessionID: "B", DestinationRunID: "run-1", SourceSessionID: "A", CapturedAt: 1, Digest: "d1", Origin: "user_selection"},
		SourceStatus: "ok",
		FeedStatus:   "included",
	}}
	env := newReferenceTestEnv(t, ops)

	result, rpcErr := callControl(t, env.handler, "reference/get", map[string]any{
		"session_id":   "B",
		"reference_id": "ref-1",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if ops.got != "B" || ops.gotRef != "ref-1" {
		t.Fatalf("get routed to (%q, %q)", ops.got, ops.gotRef)
	}
	resultJSON, _ := json.Marshal(result)
	var view domain.ReferenceView
	if err := json.Unmarshal(resultJSON, &view); err != nil {
		t.Fatal(err)
	}
	if view.Reference.ID != "ref-1" || view.SourceStatus != "ok" || view.FeedStatus != "included" {
		t.Fatalf("view = %#v", view)
	}
}

func TestReferenceGetRejectsSpoofedAndMissingFields(t *testing.T) {
	ops := &fakeReferenceOps{}
	env := newReferenceTestEnv(t, ops)
	for _, params := range []map[string]any{
		{"session_id": "B"},
		{"reference_id": "ref-1"},
		{"session_id": "B", "reference_id": "ref-1", "actor": "browser"},
		{"session_id": "B", "reference_id": "ref-1", "bogus": 1},
	} {
		if _, rpcErr := callControl(t, env.handler, "reference/get", params); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Fatalf("params %v: error = %v, want InvalidParams", params, rpcErr)
		}
	}
	if ops.got != "" || ops.gotRef != "" {
		t.Fatal("forged request reached reference service")
	}
	if _, rpcErr := callControl(t, env.handler, "reference/get", map[string]any{"session_id": "missing", "reference_id": "ref-1"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("unknown session: %v", rpcErr)
	}
	if ops.got != "" {
		t.Fatal("unknown session reached reference service")
	}
}

func TestReferenceGetHiddenWithoutService(t *testing.T) {
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.History = fakeHistoryOps{} })
	if _, rpcErr := callControl(t, env.handler, "reference/get", map[string]any{"session_id": "B", "reference_id": "ref-1"}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("get without service: %v", rpcErr)
	}
}

var _ tools.ReferenceOperations = (*fakeReferenceOps)(nil)
