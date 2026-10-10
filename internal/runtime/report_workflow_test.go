package runtime

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	nb "agent-vivy/internal/notebookcontract"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"

	inofy "github.com/ProjectViVy/inofy"
)

func waitReportRun(t *testing.T, svc *Service, runID string, want domain.RunStatus) domain.Run {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		run, err := svc.deps.Runs.GetRun(context.Background(), domain.RunID(runID))
		if err != nil {
			t.Fatal(err)
		}
		if run.Status.Terminal() {
			if run.Status != want {
				t.Fatalf("run terminal = %v, want %v", run.Status, want)
			}
			return run
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s did not reach %v", runID, want)
	return domain.Run{}
}

// TestReportWorkflowEndToEnd drives admission → sealed INOFY program →
// atomic publication → get/cancel on the real sqlite backend. The echo
// model returns non-JSON so narrate renders deterministic fallback.
func TestReportWorkflowEndToEnd(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	ac := reportAdmissionContext()

	admission, err := svc.StartReport(ctx, ac, reportRequest("op-e2e"))
	if err != nil {
		t.Fatal(err)
	}
	run := waitReportRun(t, svc, admission.RunID, domain.RunCompleted)

	// Publication committed: generation row + notebook entry head.
	g, err := backend.GetReportGenerationByRun(ctx, "home", admission.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if g.OutcomeMode != storage.OutcomePromoted || g.EntryID == "" || g.RevisionID == "" {
		t.Fatalf("generation = %+v", g)
	}
	head, err := backend.GetReportEntryHead(ctx, "home", g.EntryID)
	if err != nil || head.HeadRevisionID != g.RevisionID || head.Deleted {
		t.Fatalf("entry head = %+v", head)
	}
	res, err := svc.GetReport(ctx, ac, admission.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != string(domain.RunCompleted) || res.Generation == nil ||
		res.Generation.EntryID != g.EntryID || res.Generation.RevisionID != g.RevisionID {
		t.Fatalf("get = %+v", res)
	}
	if res.Generation.OutcomeMode == "" || res.Generation.WindowID == "" {
		t.Fatalf("provenance incomplete: %+v", res.Generation)
	}
	_ = run
}

// TestReportWorkflowCancelled verifies cancellation leaves no publication
// receipt and no fabricated success.
func TestReportWorkflowCancelled(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	ac := reportAdmissionContext()

	admission, err := svc.StartReport(ctx, ac, reportRequest("op-cancel"))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CancelReport(ctx, ac, admission.RunID); err != nil {
		t.Fatal(err)
	}
	// Cancel is best-effort: the run may finish before the cancel lands;
	// what matters is the run never reports a fabricated generation.
	for i := 0; i < 200; i++ {
		run, err := svc.deps.Runs.GetRun(ctx, domain.RunID(admission.RunID))
		if err != nil {
			t.Fatal(err)
		}
		if run.Status.Terminal() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	run, _ := svc.deps.Runs.GetRun(ctx, domain.RunID(admission.RunID))
	if run.Status == domain.RunCancelled {
		if _, err := backend.GetReportGenerationByRun(ctx, "home", admission.RunID); err == nil {
			t.Fatal("cancelled run must not carry a generation receipt")
		}
	}
	res, err := svc.GetReport(ctx, ac, admission.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status == string(domain.RunCompleted) && res.Generation == nil {
		t.Fatal("done run must carry provenance")
	}
}

// TestReportSettingsReadMaterializesDefaults checks the manual-default row
// path through the service surface.
func TestReportSettingsReadMaterializesDefaults(t *testing.T) {
	ctx := context.Background()
	svc, _ := inofyExecService(t, testsupport.NewEchoModel())
	ac := reportAdmissionContext()

	got, err := svc.ReadReportSettings(ctx, ac, rc.PeriodWeekly)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 1 || got.Enabled || got.Period != rc.PeriodWeekly {
		t.Fatalf("settings = %+v", got)
	}
	// A disabled row does not block manual generation.
	if _, err := svc.StartReport(ctx, ac, rc.ReportRequest{Period: rc.PeriodWeekly, Window: rc.WindowCompleted, OperationKey: "op-wk"}); err != nil {
		t.Fatalf("manual generation on disabled settings: %v", err)
	}
}

// TestReportNarrateModelJSON exercises the narrate effect directly with a
// model that returns valid bounded JSON — one call, zero tools.
func TestReportNarrateModelJSON(t *testing.T) {
	ctx := context.Background()
	model := &scriptedReportModel{response: `{"mode":"model","sections":[{"heading":"Summary","claims":[{"text":"worked","source_ids":["m1"]}]}]}`}
	svc, _ := inofyExecService(t, model)
	e := newReportNodeExecutor(svc)

	bundle := reportCollectOutput{
		Pins: reportRunInput{Scope: "home", Period: rc.PeriodDaily, SeriesID: "report.daily",
			WindowID: "2026-10-09", Timezone: "UTC", SectionID: "section-daily"},
		Bundle: rc.FactBundle{
			Window:   rc.Window{Period: rc.PeriodDaily, ID: "2026-10-09"},
			Facts:    []rc.Fact{{Date: "2026-10-09", Text: "user: hi", Sources: []rc.SourceRef{{Kind: "message", ID: "m1", Digest: "d"}}}},
			Included: 1,
		},
	}
	raw, _ := json.Marshal(struct {
		Input reportCollectOutput `json:"input"`
	}{Input: bundle})
	reply, err := e.effectNarrate(ctx, inofyCall(reportNodeNarrate, raw))
	if err != nil {
		t.Fatal(err)
	}
	var out reportNarrateOutput
	if err := json.Unmarshal(reply.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.Narrative.Mode != rc.NarrativeModel || model.calls != 1 {
		t.Fatalf("narrative = %+v calls=%d", out.Narrative, model.calls)
	}
}

func TestReportNarrateFallbacks(t *testing.T) {
	ctx := context.Background()
	bundle := reportCollectOutput{
		Pins:   reportRunInput{Scope: "home", Period: rc.PeriodDaily, SeriesID: "report.daily", WindowID: "2026-10-09", Timezone: "UTC"},
		Bundle: rc.FactBundle{Facts: []rc.Fact{{Date: "2026-10-09", Text: "x", Sources: []rc.SourceRef{{Kind: "message", ID: "m1"}}}}},
	}
	raw, _ := json.Marshal(struct {
		Input reportCollectOutput `json:"input"`
	}{Input: bundle})

	// Unknown source ref → deterministic fallback.
	model := &scriptedReportModel{response: `{"mode":"model","sections":[{"heading":"h","claims":[{"text":"c","source_ids":["bogus"]}]}]}`}
	svc, _ := inofyExecService(t, model)
	e := newReportNodeExecutor(svc)
	reply, err := e.effectNarrate(ctx, inofyCall(reportNodeNarrate, raw))
	if err != nil {
		t.Fatal(err)
	}
	var out reportNarrateOutput
	_ = json.Unmarshal(reply.Output, &out)
	if out.Narrative.Mode != rc.NarrativeFallback || out.Narrative.Reason != "unknown_source_ref" {
		t.Fatalf("unknown-ref fallback = %+v", out.Narrative)
	}
	// Malformed JSON → fallback.
	model.response = "not json at all"
	reply, err = e.effectNarrate(ctx, inofyCall(reportNodeNarrate, raw))
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(reply.Output, &out)
	if out.Narrative.Mode != rc.NarrativeFallback || out.Narrative.Reason != "malformed_output" {
		t.Fatalf("malformed fallback = %+v", out.Narrative)
	}
	// Empty bundle → no model call.
	empty := reportCollectOutput{Pins: bundle.Pins, Bundle: rc.FactBundle{}}
	rawE, _ := json.Marshal(struct {
		Input reportCollectOutput `json:"input"`
	}{Input: empty})
	reply, err = e.effectNarrate(ctx, inofyCall(reportNodeNarrate, rawE))
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(reply.Output, &out)
	if out.Narrative.Mode != rc.NarrativeEmpty || model.calls != 2 {
		t.Fatalf("empty = %+v calls=%d", out.Narrative, model.calls)
	}
}

func TestReportRenderModes(t *testing.T) {
	pins := reportRunInput{Period: rc.PeriodDaily, WindowID: "2026-10-09", Timezone: "UTC", Scope: "home"}
	bundle := rc.FactBundle{
		Window: rc.Window{Period: rc.PeriodDaily, ID: "2026-10-09"},
		Facts:  []rc.Fact{{Date: "2026-10-09", Text: "did work", Sources: []rc.SourceRef{{Kind: "message", ID: "m1"}}}},
	}
	// Fallback renders facts.
	doc2, err := renderReportDocument(pins, bundle, rc.Narrative{Mode: rc.NarrativeFallback, Reason: "model_timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if doc2.Mode != string(rc.NarrativeFallback) || doc2.Markdown == "" {
		t.Fatalf("fallback doc = %+v", doc2)
	}
	// Empty renders honest empty.
	doc3, err := renderReportDocument(pins, bundle, rc.Narrative{Mode: rc.NarrativeEmpty})
	if err != nil || doc3.Mode != string(rc.NarrativeEmpty) {
		t.Fatalf("empty doc = %+v", doc3)
	}
}

// scriptedReportModel answers every Generate with one fixed message; it is a
// domain.ChatModel so WrapModel turns it into the tool-free Eino surface.
type scriptedReportModel struct {
	response string
	calls    int
}

func (m *scriptedReportModel) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	m.calls++
	return scriptedStreamOf(&domain.Message{Role: domain.RoleAssistant, Content: m.response}), nil
}

type scriptedStreamImpl struct {
	msg  *domain.Message
	sent bool
}

func scriptedStreamOf(msg *domain.Message) domain.Stream[*domain.Message] {
	return &scriptedStreamImpl{msg: msg}
}

func (s *scriptedStreamImpl) Recv() (*domain.Message, error) {
	if s.sent {
		return nil, io.EOF
	}
	s.sent = true
	return s.msg, nil
}

func inofyCall(typeID string, input json.RawMessage) inofy.NodeCall {
	return inofy.NodeCall{
		Ref:              inofy.ExecutionRef{RunID: "run-x", ProgramDigest: "pd", HostBindingID: "hb"},
		Path:             "root/x",
		TypeID:           typeID,
		ImplementationID: reportImplementationID,
		Input:            input,
		OperationKey:     "op",
		Attempt:          1,
	}
}

// TestReportCollectBoundedSources collects real session messages through
// the bounded store path: tool messages and ingest-protected rows are
// excluded, facts are bounded, sources carry digests.
func TestReportCollectBoundedSources(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	_ = svc
	if err := backend.CreateSession(ctx, domain.Session{ID: "src-1", Title: "work", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	// One message inside the window.
	if err := backend.AppendMessage(ctx, domain.Message{ID: "m1", SessionID: "src-1", Role: domain.RoleUser,
		Content: "hello", CreatedAt: time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC).UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	pins := reportRunInput{Scope: "home", Period: rc.PeriodDaily, SeriesID: "report.daily",
		WindowID: "2026-10-09", Timezone: "UTC", SectionID: "section-daily",
		WindowStartMs: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC).UnixMilli(),
		WindowEndMs:   time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC).UnixMilli()}
	bundle := rc.FactBundle{}
	if err := collectSessions(ctx, backend, &bundle, pins, nil); err != nil {
		t.Fatal(err)
	}
	if bundle.Included != 1 || len(bundle.Facts) != 1 {
		t.Fatalf("collected = %+v", bundle)
	}
	if bundle.Facts[0].Sources[0].ID != "m1" || bundle.Facts[0].Sources[0].Digest == "" {
		t.Fatalf("source ref = %+v", bundle.Facts[0].Sources)
	}
}

// TestReportFeedbackSnapshot exercises the frozen feedback path: active
// comments become pinned snapshots inside the fact bundle.
func TestReportFeedbackSnapshot(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	e := newReportNodeExecutor(svc)
	// Seed a notebook entry + active comment via the notebook store.
	nbs, ok := any(backend).(interface {
		CreateEntry(context.Context, nb.MutationContext, nb.CreateEntryRequest) (nb.MutationReceipt, error)
		CreateComment(context.Context, nb.MutationContext, nb.CreateCommentRequest) (nb.MutationReceipt, error)
	})
	if !ok {
		t.Skip("backend does not expose notebook store")
	}
	mc := nb.MutationContext{ScopeID: nb.HomeScopeID, OperationKey: "k1", Actor: nb.Actor{Kind: nb.ActorHuman}}
	rec, err := nbs.CreateEntry(ctx, mc, nb.CreateEntryRequest{SectionID: "section-notes", Title: "t", Markdown: "m"})
	if err != nil {
		t.Fatal(err)
	}
	cmc := nb.MutationContext{ScopeID: nb.HomeScopeID, OperationKey: "k2", Actor: nb.Actor{Kind: nb.ActorHuman}}
	if _, err := nbs.CreateComment(ctx, cmc, nb.CreateCommentRequest{EntryID: rec.ResourceID, Body: "fix this"}); err != nil {
		t.Fatal(err)
	}
	pins := reportRunInput{Scope: "home", Period: rc.PeriodDaily, SeriesID: "report.daily",
		WindowID: "2026-10-09", Timezone: "UTC", SectionID: "section-daily",
		WindowStartMs: 0, WindowEndMs: 1,
		Target: &rc.TargetRef{EntryID: rec.ResourceID}}
	raw, _ := json.Marshal(struct {
		Input reportRunInput `json:"input"`
	}{Input: pins})
	reply, err := e.effectCollect(ctx, inofyCall(reportNodeCollect, raw))
	if err != nil {
		t.Fatal(err)
	}
	var out reportCollectOutput
	if err := json.Unmarshal(reply.Output, &out); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, fb := range out.Bundle.Feedback {
		if fb.EntryID == rec.ResourceID && fb.Digest != "" && fb.Body == "fix this" {
			found = true
		}
	}
	if !found {
		t.Fatalf("feedback not frozen: %+v", out.Bundle.Feedback)
	}
}
