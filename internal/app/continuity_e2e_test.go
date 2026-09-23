package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

const loopSentinel = "SENTINEL-C-9f3c1a-unrelated"

// loopRecordingModel wraps a scripted model and keeps every request input so
// the test can prove cross-session content never reached the model.
type loopRecordingModel struct {
	inner  model.ToolCallingChatModel
	mu     sync.Mutex
	inputs [][]*schema.Message
}

func (m *loopRecordingModel) record(input []*schema.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*schema.Message, len(input))
	copy(cp, input)
	m.inputs = append(m.inputs, cp)
}

func (m *loopRecordingModel) snapshot() [][]*schema.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]*schema.Message, len(m.inputs))
	for i, input := range m.inputs {
		cp := make([]*schema.Message, len(input))
		copy(cp, input)
		out[i] = cp
	}
	return out
}

func (m *loopRecordingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.record(input)
	return m.inner.Generate(ctx, input, opts...)
}

func (m *loopRecordingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.record(input)
	return m.inner.Stream(ctx, input, opts...)
}

func (m *loopRecordingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m.inner.WithTools(tools)
}

func loopToolCall(id, name, args string) *schema.Message {
	return schema.AssistantMessage("", []schema.ToolCall{{
		ID:       id,
		Function: schema.FunctionCall{Name: name, Arguments: args},
	}})
}

// continuityLoopFixture wires the same stack app.go composes for the
// continuity feature set: sqlite journal, workspace/sandbox managers, file
// and command backends, history/reference/deliverable services and the full
// builtin registry with the five continuity tools enabled.
type continuityLoopFixture struct {
	backend       *sqlite.Backend
	continuity    storage.ContinuityStore
	history       *runtime.HistoryService
	references    *runtime.ReferenceService
	deliverables  *runtime.DeliverableService
	callables     []tools.Tool
	checkpoints   *runtime.VersionedCheckpointStore
	workspaceBDir string
}

func newContinuityLoopFixture(t *testing.T) *continuityLoopFixture {
	t.Helper()
	ctx := context.Background()
	dataRoot := t.TempDir()

	backend, err := sqlite.Open(ctx, filepath.Join(dataRoot, "loop.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "continuity-loop")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}

	wsRoot := filepath.Join(dataRoot, "workspaces")
	manager, err := runtime.NewSessionWorkspaceManager(wsRoot, backend, backend)
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	sandbox, err := runtime.NewSandboxManager(domain.SandboxModeWorkspaceWrite, wsRoot, nil, nil)
	if err != nil {
		t.Fatalf("sandbox manager: %v", err)
	}
	fileBackend := runtime.NewEinoFilesystemBackend(manager, sandbox)
	commandBackend := runtime.NewCommandBackend(manager, sandbox, []string{"rg", "sh", "bash", "tar", "printf"}, 0)

	historyService := runtime.NewHistoryService(backend, backend)
	var continuityStore storage.ContinuityStore = backend
	referenceService := runtime.NewReferenceService(historyService, backend, backend, backend, continuityStore)
	referenceService.SetViewStores(backend, backend)
	historyService.SetReferenceLookup(referenceService.Lookup)

	scratch := filepath.Join(dataRoot, "transfers")
	if err := os.MkdirAll(scratch, 0o700); err != nil {
		t.Fatalf("transfer scratch: %v", err)
	}
	deliverableService, err := runtime.NewDeliverableService(manager, backend, backend, continuityStore, scratch)
	if err != nil {
		t.Fatalf("deliverable service: %v", err)
	}
	deliverableService.SetViewStores(backend, backend)

	registry := tools.BuiltinWithCommands(backend, fileBackend, nil, nil, nil, nil, nil, nil, commandBackend).
		WithHistory(historyService).WithReferences(referenceService).WithDeliverables(deliverableService)
	names := []string{
		tools.EchoInfoName, tools.ListDirName, tools.ReadFileName, tools.SearchFilesName,
		tools.WriteFileName, tools.PatchName, tools.ExecuteName, tools.BashName,
		tools.HistorySearchName, tools.HistoryReadName, tools.HistoryTraceName,
		tools.ReferenceContextName, tools.PresentFilesName,
	}
	callables, err := registry.Resolve(names)
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}

	workspaceBDir := filepath.Join(dataRoot, "workspace-b")
	if err := os.MkdirAll(workspaceBDir, 0o700); err != nil {
		t.Fatalf("workspace b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceBDir, "source.txt"), []byte("package alpha\n\n// TODO fill\n"), 0o600); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	return &continuityLoopFixture{
		backend:       backend,
		continuity:    continuityStore,
		history:       historyService,
		references:    referenceService,
		deliverables:  deliverableService,
		callables:     callables,
		checkpoints:   checkpoints,
		workspaceBDir: workspaceBDir,
	}
}

func (f *continuityLoopFixture) newService(t *testing.T, rec *loopRecordingModel) *runtime.Service {
	t.Helper()
	eng, err := runtime.NewEngine(context.Background(), rec, f.callables, runtime.EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		Checkpoints:          f.checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return runtime.NewService(eng, "scripted", "loop-v1", runtime.ServiceDeps{
		Journal:            f.backend,
		Runs:               f.backend,
		Messages:           f.backend,
		Notes:              f.backend,
		Approvals:          f.backend,
		Questions:          f.backend,
		ApprovalExpiration: 5 * time.Minute,
		Sessions:           f.backend,
		Sink:               &appEventSink{},
		Truncations:        f.backend,
		Compactions:        f.backend,
		Continuity:         f.continuity,
		References:         f.references,
		Deliverables:       f.deliverables,
	})
}

func (f *continuityLoopFixture) createSession(t *testing.T, id, title, workspacePath string) {
	t.Helper()
	if err := f.backend.CreateSession(context.Background(), domain.Session{
		ID:            domain.SessionID(id),
		Title:         title,
		CreatedAt:     time.Now().UnixMilli(),
		WorkspacePath: workspacePath,
	}); err != nil {
		t.Fatalf("create session %s: %v", id, err)
	}
}

func (f *continuityLoopFixture) runToCompletion(t *testing.T, svc *runtime.Service, sessionID, text string, options runtime.RunOptions) domain.RunID {
	t.Helper()
	runID, err := svc.RunWithOptions(context.Background(), domain.SessionID(sessionID), text, options)
	if err != nil {
		t.Fatalf("start run %s: %v", sessionID, err)
	}
	waitFor(t, 30*time.Second, func() bool {
		run, err := f.backend.GetRun(context.Background(), runID)
		if err != nil {
			return false
		}
		return run.Status == domain.RunCompleted || run.Status == domain.RunFailed || run.Status == domain.RunCancelled
	})
	run, err := f.backend.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("get run %s: %v", runID, err)
	}
	if run.Status != domain.RunCompleted {
		events := replayEvents(t, f.backend, runID)
		t.Fatalf("run %s ended %s; last events: %v", runID, run.Status, events[len(events)-min(3, len(events)):])
	}
	return runID
}

// TestContinuityCodingLoop drives the complete SC-D4 delivery loop on the
// real stack: session A produces rationale/message/tool-result history,
// unrelated session C commits a sentinel, session B selects A for further
// read, the scripted model searches/reads/traces, attaches a sanitized
// reference, edits a workspace source file, runs a test command, produces a
// report plus archive through the shell, and presents both as a delivery
// set. Actual model inputs are captured and asserted free of C's sentinel.
func TestContinuityCodingLoop(t *testing.T) {
	ctx := context.Background()
	f := newContinuityLoopFixture(t)

	// Source session A: assistant rationale + one committed tool result.
	f.createSession(t, "sess-a", "source", "")
	svcA := f.newService(t, &loopRecordingModel{inner: runtime.NewScriptedModel(
		loopToolCall("call-a-1", tools.EchoInfoName, `{"text":"probe alpha"}`),
		schema.AssistantMessage("PLAN-ALPHA rationale: refactor the parser then verify.", nil),
	)})
	runA := f.runToCompletion(t, svcA, "sess-a", "explain the plan", runtime.RunOptions{})
	var aToolResults int
	for _, ev := range replayEvents(t, f.backend, runA) {
		if ev.Type == domain.EventToolFinished {
			aToolResults++
		}
	}
	if aToolResults < 1 {
		t.Fatalf("source session A committed %d tool results, want >= 1", aToolResults)
	}
	msgsA, err := f.backend.ListMessages(ctx, "sess-a")
	if err != nil || len(msgsA) == 0 {
		t.Fatalf("list session A messages: %v (%d)", err, len(msgsA))
	}
	msgIDA := msgsA[len(msgsA)-1].ID

	// Unrelated session C: only the sentinel, never shared.
	f.createSession(t, "sess-c", "unrelated", "")
	svcC := f.newService(t, &loopRecordingModel{inner: runtime.NewScriptedModel(
		schema.AssistantMessage("private note "+loopSentinel, nil),
	)})
	f.runToCompletion(t, svcC, "sess-c", "keep this private", runtime.RunOptions{})

	// Session B selects A for further read and runs the full coding loop.
	f.createSession(t, "sess-b", "delivery", f.workspaceBDir)
	selection := domain.HistorySelection{
		SourceSessionID: "sess-a",
		Refs: []domain.SourceRef{{
			SessionID: "sess-a",
			RunID:     runA,
			Kind:      "message",
			MessageID: msgIDA,
		}},
	}
	previewCtx := runtime.WithHistoryOperator(tools.WithSessionID(ctx, "sess-b"))
	preview, err := f.references.Preview(previewCtx, selection)
	if err != nil {
		t.Fatalf("preview reference selection: %v", err)
	}
	if preview.Digest == "" {
		t.Fatal("preview returned empty selection digest")
	}

	recB := &loopRecordingModel{inner: runtime.NewScriptedModel(
		loopToolCall("call-b-1", tools.HistorySearchName, `{"query":"PLAN-ALPHA","session_ids":["sess-a"]}`),
		loopToolCall("call-b-2", tools.HistoryReadName, `{"selection":{"source_session_id":"sess-a","refs":[{"session_id":"sess-a","run_id":"`+string(runA)+`","kind":"message","message_id":"`+msgIDA+`"}]}}`),
		loopToolCall("call-b-3", tools.HistoryTraceName, `{"source_ref":{"session_id":"sess-a","run_id":"`+string(runA)+`","kind":"message","message_id":"`+msgIDA+`"}}`),
		loopToolCall("call-b-4", tools.ReferenceContextName, `{"selection":{"source_session_id":"sess-a","refs":[{"session_id":"sess-a","run_id":"`+string(runA)+`","kind":"message","message_id":"`+msgIDA+`"}]},"expected_digest":"`+preview.Digest+`","description":"source plan"}`),
		loopToolCall("call-b-5", tools.PatchName, `{"path":"source.txt","old_string":"// TODO fill","new_string":"// PLAN-ALPHA implemented\nresult := 42"}`),
		loopToolCall("call-b-6", tools.ExecuteName, `{"command":"rg","args":["-n","PLAN-ALPHA","source.txt"]}`),
		loopToolCall("call-b-7", tools.BashName, `{"command":"printf 'report for PLAN-ALPHA\\n' > report.txt && tar -cf report.tar report.txt"}`),
		loopToolCall("call-b-8", tools.PresentFilesName, `{"files":[{"path":"report.txt","description":"final report"},{"path":"report.tar","description":"report archive"}],"title":"Report bundle"}`),
		schema.AssistantMessage("loop complete", nil),
	)}
	svcB := f.newService(t, recB)
	runB := f.runToCompletion(t, svcB, "sess-b", "fix the source and deliver the report", runtime.RunOptions{
		Profile: domain.PolicyProfileFullAuto,
		Continuity: &domain.ContinuityInput{
			RequestID:    "req-loop-1",
			HistoryScope: domain.HistoryScope{SessionIDs: []domain.SessionID{"sess-a"}},
			References: []domain.ReferenceSelection{{
				Selection:      selection,
				ExpectedDigest: preview.Digest,
			}},
		},
	})

	// The sentinel from unrelated session C never reached the model.
	for i, input := range recB.snapshot() {
		if strings.Contains(joinSchemaContent(input), loopSentinel) {
			t.Fatalf("model request %d contains unrelated session C sentinel", i)
		}
	}

	// Committed journal evidence: accepted scope, one admission reference,
	// one model-side attach, workspace mutations and the presented set.
	events := replayEvents(t, f.backend, runB)
	var attached, presented, toolFinished int
	var startedPayload json.RawMessage
	for _, ev := range events {
		switch ev.Type {
		case domain.EventRunStarted:
			startedPayload = ev.Payload
		case domain.EventContextReferenceAttached:
			attached++
		case domain.EventDeliverablesPresented:
			presented++
		case domain.EventToolFinished:
			toolFinished++
		}
	}
	if attached != 2 {
		t.Fatalf("context.reference_attached events = %d, want 2 (admission + model tool)", attached)
	}
	if presented != 1 {
		t.Fatalf("deliverables.presented events = %d, want 1", presented)
	}
	if toolFinished < 8 {
		t.Fatalf("tool.finished events = %d, want >= 8", toolFinished)
	}
	var started struct {
		HistoryScope *domain.AcceptedHistoryScope `json:"history_scope"`
	}
	if err := json.Unmarshal(startedPayload, &started); err != nil || started.HistoryScope == nil {
		t.Fatalf("run.started lacks accepted history scope: %s", startedPayload)
	}
	var sawA, sawC bool
	for _, src := range started.HistoryScope.SourceSessionIDs {
		if src == "sess-a" {
			sawA = true
		}
		if src == "sess-c" {
			sawC = true
		}
	}
	if !sawA || sawC {
		t.Fatalf("accepted scope sources = %v, want sess-a present and sess-c absent", started.HistoryScope.SourceSessionIDs)
	}

	// Physical workspace mutations through the shell and file backends.
	source, err := os.ReadFile(filepath.Join(f.workspaceBDir, "source.txt"))
	if err != nil || !strings.Contains(string(source), "PLAN-ALPHA implemented") {
		t.Fatalf("source.txt not patched: %v %q", err, source)
	}
	for _, name := range []string{"report.txt", "report.tar"} {
		if _, err := os.Stat(filepath.Join(f.workspaceBDir, name)); err != nil {
			t.Fatalf("shell artifact %s missing: %v", name, err)
		}
	}

	// Delivery discovery: the committed set lists both artifacts and the
	// read path returns byte-identical content bound to its digest.
	setCtx := tools.WithSessionID(ctx, "sess-b")
	page, err := f.deliverables.List(setCtx, "sess-b", "", 10)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("deliverables list: %v (%d sets)", err, len(page.Items))
	}
	set := page.Items[0]
	if len(set.Items) != 2 || len(set.Failures) != 0 {
		t.Fatalf("set items/failures = %d/%d, want 2/0", len(set.Items), len(set.Failures))
	}
	var reportItem *domain.Deliverable
	for i := range set.Items {
		if set.Items[i].Path == "report.txt" {
			reportItem = &set.Items[i]
		}
	}
	if reportItem == nil {
		t.Fatalf("report.txt missing from delivered items: %+v", set.Items)
	}
	diskSHA := sha256.Sum256(mustReadFile(t, filepath.Join(f.workspaceBDir, "report.txt")))
	if reportItem.SHA256 != hex.EncodeToString(diskSHA[:]) {
		t.Fatalf("item sha256 %s != disk %s", reportItem.SHA256, hex.EncodeToString(diskSHA[:]))
	}
	chunk, err := f.deliverables.Read(setCtx, domain.DeliveryReadRequest{
		ItemID:         reportItem.ID,
		ExpectedDigest: reportItem.SHA256,
		Offset:         0,
		Length:         256 << 10,
	})
	if err != nil {
		t.Fatalf("deliverables read: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(chunk.DataBase64)
	if err != nil || string(raw) != "report for PLAN-ALPHA\n" {
		t.Fatalf("delivered bytes mismatch: %v %q", err, raw)
	}
	if !chunk.EOF {
		t.Fatal("single read should reach EOF for a small file")
	}
	if err := f.deliverables.CloseTransfer(setCtx, chunk.TransferID); err != nil {
		t.Fatalf("close transfer: %v", err)
	}

	// An identical retry under the same caller-stable request_id replays the
	// committed admission instead of writing a second run — the receipt
	// binds the caller identity, so no duplicate reference or turn lands.
	recB2 := &loopRecordingModel{inner: runtime.NewScriptedModel(
		schema.AssistantMessage("ack", nil),
	)}
	svcB2 := f.newService(t, recB2)
	replayID, err := svcB2.RunWithOptions(ctx, "sess-b", "fix the source and deliver the report", runtime.RunOptions{
		Profile: domain.PolicyProfileFullAuto,
		Continuity: &domain.ContinuityInput{
			RequestID:    "req-loop-1",
			HistoryScope: domain.HistoryScope{SessionIDs: []domain.SessionID{"sess-a"}},
			References: []domain.ReferenceSelection{{
				Selection:      selection,
				ExpectedDigest: preview.Digest,
			}},
		},
	})
	if err != nil {
		t.Fatalf("identical retry under committed request_id: %v", err)
	}
	if replayID != runB {
		t.Fatalf("retry returned %s, want the committed run %s", replayID, runB)
	}
	if got := len(recB2.snapshot()); got != 0 {
		t.Fatalf("replayed admission reached the model %d times, want 0", got)
	}
}

func joinSchemaContent(msgs []*schema.Message) string {
	var sb strings.Builder
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		sb.WriteString(msg.Content)
	}
	return sb.String()
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
