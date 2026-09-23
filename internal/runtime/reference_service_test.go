package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

// referenceFixture composes the real T3/T4 services and stores the way the
// app does: one HistoryService feeding one ReferenceService, wired into a
// live Service so admission runs the genuine atomic path.
type referenceFixture struct {
	backend *sqlite.Backend
	refs    *ReferenceService
	history *HistoryService
	svc     *Service
	sink    *testSink
}

func newReferenceFixture(t *testing.T) *referenceFixture {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "references.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	for _, session := range []domain.Session{
		{ID: "A", Title: "source", CreatedAt: 1, WorkspacePath: "/srv/source-a"},
		{ID: "B", Title: "destination", CreatedAt: 2},
		{ID: "C", Title: "adjacent", CreatedAt: 3},
	} {
		if err := backend.CreateSession(ctx, session); err != nil {
			t.Fatalf("create session %s: %v", session.ID, err)
		}
	}
	for _, message := range []domain.Message{
		{ID: "a-safe", SessionID: "A", Role: domain.RoleUser, CreatedAt: 10, Content: "release notes draft for alpha"},
		{ID: "a-two", SessionID: "A", Role: domain.RoleUser, CreatedAt: 11, Content: "second selectable record"},
		{ID: "c-marker", SessionID: "C", Role: domain.RoleUser, CreatedAt: 12, Content: "adjacent-source-marker"},
	} {
		if err := backend.AppendMessage(ctx, message); err != nil {
			t.Fatalf("append message %s: %v", message.ID, err)
		}
	}
	history := NewHistoryService(backend, backend)
	refs := NewReferenceService(history, backend, backend, backend, backend)
	history.SetReferenceLookup(refs.Lookup)

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(testsupport.NewEchoModel()), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		Sink: sink, Truncations: backend, Continuity: backend, References: refs,
	})
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })
	return &referenceFixture{backend: backend, refs: refs, history: history, svc: svc, sink: sink}
}

func (f *referenceFixture) PreviewAsOperator(t *testing.T, dest domain.SessionID, selection domain.HistorySelection) domain.ReferencePreview {
	t.Helper()
	ctx := WithHistoryOperator(tools.WithSessionID(context.Background(), dest))
	preview, err := f.refs.Preview(ctx, selection)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	return preview
}

// AdmitAsOperator submits one task through the real atomic admission: the
// operator UI carries preview digests inside each reference selector.
func (f *referenceFixture) AdmitAsOperator(t *testing.T, input *domain.ContinuityInput) (domain.RunID, error) {
	t.Helper()
	return f.svc.RunWithOptions(context.Background(), "B", "summarize the selected context", RunOptions{Continuity: input})
}

// AttachAsModel invokes the same path the reference_context tool takes: the
// caller's run ctx carries the accepted scope and the stable tool_call id.
func (f *referenceFixture) AttachAsModel(t *testing.T, ctx context.Context, selection domain.ReferenceSelection) (domain.ContextReference, error) {
	t.Helper()
	return f.refs.Attach(ctx, selection)
}

// modelCtx builds the tool invocation context for one live run row, with the
// accepted scope and stable tool_call id the adapter would propagate. No
// sources means the run carries no admitted scope at all.
func (f *referenceFixture) modelCtx(t *testing.T, runID domain.RunID, callID string, sources ...domain.SessionID) context.Context {
	t.Helper()
	ctx := context.Background()
	if len(sources) > 0 {
		scope, err := domain.NewAcceptedHistoryScope("B", sources)
		if err != nil {
			t.Fatalf("accepted scope: %v", err)
		}
		ctx = WithHistoryScope(ctx, scope)
	}
	ctx = tools.WithSessionID(ctx, "B")
	ctx = tools.WithRunID(ctx, runID)
	ctx = tools.WithToolCallID(ctx, callID)
	return ctx
}

func (f *referenceFixture) createLiveRun(t *testing.T, id string) {
	t.Helper()
	if err := f.backend.CreateRun(context.Background(), domain.Run{ID: domain.RunID(id), SessionID: "B", Status: domain.RunActive, CreatedAt: 20, Kind: domain.RunKindPrimary, RootID: domain.RunID(id)}); err != nil {
		t.Fatalf("create run %s: %v", id, err)
	}
}

func safeSelectionA() domain.HistorySelection {
	return domain.HistorySelection{
		SourceSessionID: "A",
		Refs: []domain.SourceRef{
			{SessionID: "A", MessageID: "a-safe", Kind: string(domain.SourceKindMessage)},
		},
	}
}

func committedReferenceEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID) []domain.ContextReference {
	t.Helper()
	iter, err := backend.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay %s: %v", runID, err)
	}
	defer func() { _ = iter.Close() }()
	var out []domain.ContextReference
	for iter.Next() {
		entry := iter.Value()
		if entry.Event.Type != domain.EventContextReferenceAttached {
			continue
		}
		var payload payloadContextReferenceAttached
		if err := json.Unmarshal(entry.Event.Payload, &payload); err != nil {
			t.Fatalf("decode reference event: %v", err)
		}
		out = append(out, payload.Reference)
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("replay err: %v", err)
	}
	return out
}

func TestReferencePreviewAsOperator(t *testing.T) {
	f := newReferenceFixture(t)
	preview := f.PreviewAsOperator(t, "B", safeSelectionA())
	if preview.SourceStatus != string(domain.HistoryStatusOK) || len(preview.Items) != 1 {
		t.Fatalf("preview = %#v", preview)
	}
	if preview.Digest == "" || preview.Digest != CanonicalHistorySelectionDigest(preview.Items) {
		t.Fatalf("preview digest = %q", preview.Digest)
	}
	if preview.ByteCount <= 0 || preview.Items[0].Text != "release notes draft for alpha" {
		t.Fatalf("preview items = %#v", preview.Items)
	}
}

func TestReferenceAdmissionAttachesExcerptOnly(t *testing.T) {
	f := newReferenceFixture(t)
	preview := f.PreviewAsOperator(t, "B", safeSelectionA())
	// The excerpt attaches without its source entering the model's
	// HistoryScope: the empty scope resolves to the destination alone.
	runID, err := f.AdmitAsOperator(t, &domain.ContinuityInput{
		RequestID: "req-excerpt",
		References: []domain.ReferenceSelection{
			{Selection: preview.Selection, ExpectedDigest: preview.Digest},
		},
	})
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	events := committedReferenceEvents(t, f.backend, runID)
	if len(events) != 1 {
		t.Fatalf("reference events = %#v", events)
	}
	accepted := events[0]
	if accepted.SourceSessionID != "A" {
		t.Fatal("lost selected source")
	}
	if accepted.DestinationSessionID != "B" || accepted.DestinationRunID != runID || accepted.Origin != "user_selection" {
		t.Fatalf("reference destination/origin = %#v", accepted)
	}
	if accepted.Digest != preview.Digest || len(accepted.Items) != 1 || accepted.Items[0].Text != "release notes draft for alpha" {
		t.Fatalf("snapshot drifted from preview: %#v", accepted)
	}
	if accepted.SourceWorkspace != "/srv/source-a" {
		t.Fatalf("source workspace label = %q", accepted.SourceWorkspace)
	}
	// The excerpt attaches beside a broader model HistoryScope without
	// widening it.
	iter, err := f.backend.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	var started payloadRunStarted
	seen := 0
	for iter.Next() {
		entry := iter.Value()
		if entry.Event.Type != domain.EventRunStarted {
			continue
		}
		seen++
		if err := json.Unmarshal(entry.Event.Payload, &started); err != nil {
			t.Fatalf("decode started: %v", err)
		}
	}
	_ = iter.Close()
	if seen != 1 || started.HistoryScope == nil {
		t.Fatalf("started scope missing: seen=%d scope=%#v", seen, started.HistoryScope)
	}
	if len(started.HistoryScope.SourceSessionIDs) != 1 || started.HistoryScope.SourceSessionIDs[0] != "B" {
		t.Fatal("excerpt broadened scope")
	}
}

func TestReferenceAdmissionDigestConflictFailsRecoverably(t *testing.T) {
	f := newReferenceFixture(t)
	preview := f.PreviewAsOperator(t, "B", safeSelectionA())
	_, err := f.AdmitAsOperator(t, &domain.ContinuityInput{
		RequestID:    "req-stale",
		HistoryScope: domain.HistoryScope{SessionIDs: []domain.SessionID{"A"}},
		References: []domain.ReferenceSelection{
			{Selection: preview.Selection, ExpectedDigest: strings.Repeat("0", 64)},
		},
	})
	if err == nil || !errors.Is(err, storage.ErrSourceChanged) {
		t.Fatalf("stale digest admission = %v", err)
	}
	// The draft stayed recoverable: nothing was committed for destination B.
	runs, listErr := f.backend.ListRunsBySession(context.Background(), "B")
	if listErr != nil {
		t.Fatalf("list runs: %v", listErr)
	}
	if len(runs) != 0 {
		t.Fatalf("stale admission persisted %d runs", len(runs))
	}
}

func TestReferenceAdjacentSourceNeverLeaks(t *testing.T) {
	f := newReferenceFixture(t)
	f.createLiveRun(t, "run-model-denied")
	// A model run admitted to source A cannot attach adjacent session C.
	ctx := f.modelCtx(t, "run-model-denied", "call-denied", "A")
	_, err := f.AttachAsModel(t, ctx, domain.ReferenceSelection{
		Selection: domain.HistorySelection{
			SourceSessionID: "C",
			Refs:            []domain.SourceRef{{SessionID: "C", MessageID: "c-marker", Kind: string(domain.SourceKindMessage)}},
		},
	})
	var refErr ReferenceError
	if err == nil || !errors.As(err, &refErr) || refErr.Status != string(domain.HistoryStatusForbidden) {
		t.Fatalf("adjacent source attach = %v", err)
	}
	if events := committedReferenceEvents(t, f.backend, "run-model-denied"); len(events) != 0 {
		t.Fatal("adjacent source content leaked")
	}
	// Nor can one selection smuggle adjacent refs past the A scope.
	mixed := domain.HistorySelection{
		SourceSessionID: "A",
		Refs: []domain.SourceRef{
			{SessionID: "A", MessageID: "a-safe", Kind: string(domain.SourceKindMessage)},
			{SessionID: "C", MessageID: "c-marker", Kind: string(domain.SourceKindMessage)},
		},
	}
	if _, err := f.AttachAsModel(t, f.modelCtx(t, "run-model-denied", "call-mixed", "A"), domain.ReferenceSelection{Selection: mixed}); err == nil {
		t.Fatal("a mixed selection smuggled adjacent content")
	}
}

func TestReferenceModelAttachCommitsAndDeduplicates(t *testing.T) {
	f := newReferenceFixture(t)
	f.createLiveRun(t, "run-model-1")
	ctx := f.modelCtx(t, "run-model-1", "call-attach-1", "A")
	selection := domain.ReferenceSelection{Selection: safeSelectionA()}
	first, err := f.AttachAsModel(t, ctx, selection)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if first.Origin != "model_tool" || first.DestinationRunID != "run-model-1" || first.Digest == "" {
		t.Fatalf("first reference = %#v", first)
	}
	// A checkpoint resume re-executes the identical call: the receipt replays
	// the committed reference rather than writing a second event.
	second, err := f.AttachAsModel(t, ctx, selection)
	if err != nil {
		t.Fatalf("duplicate attach: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate attach produced a new reference: %s vs %s", second.ID, first.ID)
	}
	if events := committedReferenceEvents(t, f.backend, "run-model-1"); len(events) != 1 {
		t.Fatalf("duplicate attach wrote %d events", len(events))
	}
	// A different tool_call id is a genuinely new call and may commit again.
	next := f.modelCtx(t, "run-model-1", "call-attach-2", "A")
	third, err := f.AttachAsModel(t, next, selection)
	if err != nil {
		t.Fatalf("second call attach: %v", err)
	}
	if third.ID == first.ID {
		t.Fatal("distinct tool_call reused a reference id")
	}
}

func TestReferenceModelAttachHonorsRunScope(t *testing.T) {
	f := newReferenceFixture(t)
	f.createLiveRun(t, "run-model-scope")
	// A run with no admitted scope may only attach its own session.
	ctx := f.modelCtx(t, "run-model-scope", "call-scope")
	_, err := f.AttachAsModel(t, ctx, domain.ReferenceSelection{Selection: safeSelectionA()})
	var refErr ReferenceError
	if err == nil || !errors.As(err, &refErr) || refErr.Status != string(domain.HistoryStatusForbidden) {
		t.Fatalf("out-of-scope attach = %v", err)
	}
	own, err := f.AttachAsModel(t, ctx, domain.ReferenceSelection{
		Selection: domain.HistorySelection{
			SourceSessionID: "B",
			Refs:            []domain.SourceRef{{SessionID: "B", RunID: "run-model-scope", EventSeq: 1, Kind: string(domain.SourceKindEvent)}},
		},
	})
	// The own-session read may be not_found (run carries no events yet); the
	// forbidden check is the contract under test.
	if err != nil {
		var ownErr ReferenceError
		if !errors.As(err, &ownErr) || ownErr.Status == string(domain.HistoryStatusForbidden) {
			t.Fatalf("own session attach = %v, ref=%#v", err, own)
		}
	}
}

func TestReferenceModelAttachRequiresCallIdentity(t *testing.T) {
	f := newReferenceFixture(t)
	ctx := tools.WithSessionID(tools.WithRunID(context.Background(), "run-missing"), "B")
	_, err := f.refs.Attach(ctx, domain.ReferenceSelection{Selection: safeSelectionA()})
	var refErr ReferenceError
	if err == nil || !errors.As(err, &refErr) || refErr.Status != string(domain.HistoryStatusForbidden) {
		t.Fatalf("identity-less attach = %v", err)
	}
}

func TestReferenceModelAttachRejectsInvalidDigestEncoding(t *testing.T) {
	f := newReferenceFixture(t)
	f.createLiveRun(t, "run-model-utf8")
	ctx := f.modelCtx(t, "run-model-utf8", "call-utf8", "A")
	_, err := f.AttachAsModel(t, ctx, domain.ReferenceSelection{
		Selection:      safeSelectionA(),
		ExpectedDigest: "bad\x00digest",
	})
	if err == nil {
		t.Fatal("control bytes in expected_digest were accepted")
	}
}

func TestReferenceModelAttachPerTaskBudget(t *testing.T) {
	f := newReferenceFixture(t)
	limits := domain.DefaultContinuityLimits()
	limits.ReferencesPerTask = 2
	f.refs.SetLimits(limits)
	f.createLiveRun(t, "run-model-budget")
	selection := domain.ReferenceSelection{Selection: safeSelectionA()}
	for i, callID := range []string{"call-b1", "call-b2"} {
		if _, err := f.AttachAsModel(t, f.modelCtx(t, "run-model-budget", callID, "A"), selection); err != nil {
			t.Fatalf("attach %d: %v", i, err)
		}
	}
	_, err := f.AttachAsModel(t, f.modelCtx(t, "run-model-budget", "call-b3", "A"), selection)
	var refErr ReferenceError
	if err == nil || !errors.As(err, &refErr) || !strings.Contains(refErr.Reason, "per task") {
		t.Fatalf("per task budget attach = %v", err)
	}
}

func TestReferenceOverBudgetSelectionIsExplicit(t *testing.T) {
	f := newReferenceFixture(t)
	limits := domain.DefaultContinuityLimits()
	limits.ReferenceBytes = 32
	f.refs.SetLimits(limits)
	preview := f.PreviewAsOperator(t, "B", safeSelectionA())
	if preview.ByteCount <= 32 {
		t.Fatalf("preview byte count = %d", preview.ByteCount)
	}
	// The preview stays honest; the attach refuses rather than truncating.
	f.createLiveRun(t, "run-model-bytes")
	_, err := f.AttachAsModel(t, f.modelCtx(t, "run-model-bytes", "call-bytes", "A"), domain.ReferenceSelection{Selection: safeSelectionA()})
	if err == nil {
		t.Fatal("over-budget snapshot was silently accepted")
	}
}

func TestReferenceReadByIDReturnsDestinationCopy(t *testing.T) {
	f := newReferenceFixture(t)
	f.createLiveRun(t, "run-model-read")
	ctx := f.modelCtx(t, "run-model-read", "call-read", "A")
	reference, err := f.AttachAsModel(t, ctx, domain.ReferenceSelection{Selection: safeSelectionA()})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	page, err := f.history.Read(tools.WithSessionID(context.Background(), "B"), domain.HistoryReadRequest{ReferenceID: reference.ID})
	if err != nil {
		t.Fatalf("reference read: %v", err)
	}
	if page.Status != string(domain.HistoryStatusOK) || len(page.Items) != 1 || page.Items[0].Text != "release notes draft for alpha" {
		t.Fatalf("reference page = %#v", page)
	}
	// The source session never sees the destination-owned copy.
	page, err = f.history.Read(tools.WithSessionID(context.Background(), "A"), domain.HistoryReadRequest{ReferenceID: reference.ID})
	if err != nil {
		t.Fatalf("cross read: %v", err)
	}
	if page.Status == string(domain.HistoryStatusOK) || len(page.Items) != 0 {
		t.Fatalf("reference read broadened source scope: %#v", page)
	}
}

func TestReferenceSourceDeletionFailsAttach(t *testing.T) {
	f := newReferenceFixture(t)
	preview := f.PreviewAsOperator(t, "B", safeSelectionA())
	if err := f.backend.DeleteSession(context.Background(), "A"); err != nil {
		t.Fatalf("delete source: %v", err)
	}
	_, err := f.AdmitAsOperator(t, &domain.ContinuityInput{
		RequestID:    "req-deleted",
		HistoryScope: domain.HistoryScope{SessionIDs: []domain.SessionID{"A"}},
		References:   []domain.ReferenceSelection{{Selection: preview.Selection, ExpectedDigest: preview.Digest}},
	})
	if err == nil {
		t.Fatal("deleted source was still attachable")
	}
}

func jsonString(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
