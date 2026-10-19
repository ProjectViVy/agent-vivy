package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

func TestIssue58NotebookProvenanceRetainsOriginals(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "provenance.db")
	svc, b := openD15CrashService(t, path)
	svc.deps.Sink = newTestSink()
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("notebook-provenance", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "notebook-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper(a.Run.ID, 64<<10)
	persist := func(e domain.RunEvent) {
		t.Helper()
		if !svc.persistAndPublish(ctx, source.SessionID, e) {
			t.Fatal("persist")
		}
	}
	request := func(origin string, excluded bool) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"source": "main", "input_provenance": map[string]any{"content_origin": origin, "exclude_automatic_ingest": excluded}})
		persist(domain.RunEvent{RunID: a.Run.ID, Type: domain.EventModelRequest, PayloadVersion: 4, Payload: raw})
	}
	request("original", false)
	args := []byte(`{"id":"note-1"}`)
	persist(m.build(domain.EventToolRequested, payloadToolRequested{ToolCallID: "read-1", ToolName: tools.ReadNoteName, Args: map[string]any{"id": "note-1"}, OriginalArguments: string(args)}))
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	op, err := c.Admit(ctx, "read-1", tools.ReadNoteName, args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Execute(ctx, op, func(context.Context) (string, error) { return "private notebook exact bytes", nil })
	if err != nil {
		t.Fatal(err)
	}
	persist(m.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: "read-1", ToolName: tools.ReadNoteName, Result: result}))
	request("notebook", true)
	persist(m.build(domain.EventModelDelta, payloadModelDelta{Delta: "inseparable derived answer"}))
	completion := m.build(domain.EventModelCompleted, v2Completion("inseparable derived answer"))
	completion.PayloadVersion = 2
	persist(completion)
	svc.emitTerminal(ctx, m, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
	rows, interaction, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, a.Run.ID)
	if err != nil {
		t.Fatalf("Notebook originals must remain sealable: %v", err)
	}
	for _, row := range rows {
		if row.Role == domain.RoleTool || row.Content == "inseparable derived answer" {
			if row.ContentOrigin != "notebook" || !row.ExcludeAutomaticIngest {
				t.Fatalf("lost derivative exclusion %+v", row)
			}
		}
	}
	if rows[len(rows)-2].Content != "private notebook exact bytes" {
		t.Fatalf("original tool bytes lost: %+v", rows)
	}
	if _, err := b.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, interaction); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.ReadSubjectInteraction(ctx, source.SubjectHistoryBinding, interaction.ID)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := storage.SubjectEvidenceDigest(got)
	if err != nil || digest != interaction.SourceDigest {
		t.Fatalf("restart changed exact evidence %s %s %v", digest, interaction.SourceDigest, err)
	}
}

func TestIssue58GroupedAssistantUnknownDominates(t *testing.T) {
	rows := []domain.Message{{Role: domain.RoleAssistant, ToolCallID: "a", ToolName: "first", ToolArgs: []byte(`{}`), ContentOrigin: "notebook", ExcludeAutomaticIngest: true}, {Role: domain.RoleAssistant, ToolCallID: "b", ToolName: "second", ToolArgs: []byte(`{}`)}}
	grouped := projectFeed(rows)
	if len(grouped) != 1 {
		t.Fatal("assistant calls must remain grouped")
	}
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	p, err := svc.modelInputProvenance(context.Background(), "unused", grouped)
	if err != nil || p.ContentOrigin != "unknown" || p.ExcludeAutomaticIngest {
		t.Fatalf("unknown hidden by excluded sibling %+v %v", p, err)
	}
	_ = schema.User
}

func TestIssue58InvalidUTF8ArgumentsRejectedBeforeSerialization(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("utf8-args", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "utf8-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	raw := append([]byte(`{"value":"`), 0xff)
	raw = append(raw, []byte(`"}`)...)
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	if _, err := c.Admit(ctx, "invalid", "issue58_exact", raw, raw, raw); err == nil {
		t.Fatal("admitted invalid UTF8 arguments before serialization")
	}
	if _, err := b.GetToolOperation(ctx, a.Run.ID, "invalid"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rejection left operation: %v", err)
	}
}

func TestIssue58GroupedAssistantKnownExclusion(t *testing.T) {
	rows := []domain.Message{{Role: domain.RoleAssistant, ToolCallID: "a", ToolName: "first", ToolArgs: []byte(`{}`), ContentOrigin: "notebook", ExcludeAutomaticIngest: true}, {Role: domain.RoleAssistant, ToolCallID: "b", ToolName: "second", ToolArgs: []byte(`{}`), ContentOrigin: "original"}}
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	for _, withContext := range []bool{false, true} {
		grouped := projectFeed(rows)
		if withContext {
			var err error
			grouped, err = projectFeedWithContext(context.Background(), nil, rows)
			if err != nil {
				t.Fatal(err)
			}
		}
		p, err := svc.modelInputProvenance(context.Background(), "unused", grouped)
		if err != nil || p.ContentOrigin != "notebook" || !p.ExcludeAutomaticIngest {
			t.Fatalf("known grouped derivative lost: %+v %v", p, err)
		}
	}
}

func TestIssue58ModelRequestPersistsInputProvenance(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	mustCreateSession(t, b, "observed")
	if err := b.CreateRun(ctx, domain.Run{ID: "observed-run", SessionID: "observed", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper("observed-run", 64<<10)
	m.runProvider = "fixture"
	m.runModel = "fixture"
	obs := modelCallObserverFactoryFrom(svc.withLiveModelStreamObserver(ctx, m, "observed", nil, nil, false, nil))(modelCallRoute{})
	input := projectFeed([]domain.Message{{Role: domain.RoleTool, ToolCallID: "note", Content: "note body", ContentOrigin: "notebook", ExcludeAutomaticIngest: true}})
	meta, err := obs.Begin(ctx, modelCallInput{Mode: "stream", Messages: input})
	if err != nil {
		t.Fatal(err)
	}
	chunks := []*schema.Message{{Role: schema.Assistant, Content: "derived "}, {Role: schema.Assistant, Content: "answer"}}
	for _, chunk := range chunks {
		if err := obs.Chunk(ctx, meta, chunk); err != nil {
			t.Fatal(err)
		}
	}
	joined, err := schema.ConcatMessages(chunks)
	if err != nil {
		t.Fatalf("native Eino stream concat: %v", err)
	}
	p, err := svc.modelInputProvenance(ctx, "observed-run", []*schema.Message{joined})
	if err != nil || p.ContentOrigin != "notebook" || !p.ExcludeAutomaticIngest {
		t.Fatalf("stream lost provenance %+v %v", p, err)
	}
	found := false
	for _, e := range replayAll(t, b, "observed-run") {
		if e.Type == domain.EventModelRequest {
			found = true
			if e.PayloadVersion != 4 {
				t.Fatalf("request version %d", e.PayloadVersion)
			}
			p, err := storage.DecodeModelInputProvenance(e)
			if err != nil || p.ContentOrigin != "notebook" || !p.ExcludeAutomaticIngest {
				t.Fatalf("persisted request %+v %v", p, err)
			}
		}
	}
	if !found {
		t.Fatal("missing authoritative request")
	}
}

func TestIssue58InvalidUTF8ToolResultRecordsExecutionUncertainty(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("utf8-result", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "utf8-result-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	op, err := c.Admit(ctx, "executed", "issue58_exact", []byte(`{}`), []byte(`{}`), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err = c.Execute(ctx, op, func(context.Context) (string, error) { calls++; return string([]byte{0xff}), nil })
	if !errors.Is(err, ErrToolOperationUnknown) || calls != 1 {
		t.Fatalf("execution outcome misreported calls=%d err=%v", calls, err)
	}
	stored, readErr := b.GetToolOperation(ctx, a.Run.ID, op.OperationID)
	if readErr != nil || stored.State != domain.ToolOperationClaimed {
		t.Fatalf("must retain claimed fence %+v %v", stored, readErr)
	}
	_, err = c.Execute(ctx, op, func(context.Context) (string, error) { calls++; return "replayed", nil })
	if !errors.Is(err, ErrToolOperationUnknown) || calls != 1 {
		t.Fatalf("uncertain executed tool replayed calls=%d %v", calls, err)
	}
}

func TestIssue58ProvenanceExtraJSONRoundTrip(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	for _, origin := range []string{"notebook", "unknown", "future"} {
		msg := schema.UserMessage("derived summary")
		stampInputProvenance(msg, storage.InputProvenance{ContentOrigin: origin, ExcludeAutomaticIngest: origin == "notebook"})
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		var restored schema.Message
		if err := json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		p, err := svc.modelInputProvenance(context.Background(), "restored", []*schema.Message{&restored})
		if err != nil {
			t.Fatal(err)
		}
		expected := origin
		if origin == "future" {
			expected = "unknown"
		}
		if p.ContentOrigin != expected {
			t.Fatalf("JSON restored %s recertified as %+v", origin, p)
		}
	}
}

func TestIssue58LegacySummaryCannotRecertifyUnknown(t *testing.T) {
	for _, tc := range []struct {
		name, origin string
		notebook     bool
		wantOrigin   string
	}{{"legacy", "", false, "unknown"}, {"fresh", "original", false, "original"}, {"legacy-mixed-notebook", "", true, "unknown"}, {"known-notebook", "original", true, "notebook"}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, b, _ := newTestService(t, testsupport.NewEchoModel())
			mustCreateSession(t, b, "fold")
			svc.deps.Compactions = b
			rows := []domain.Message{{ID: "u", SessionID: "fold", Role: domain.RoleUser, CreatedAt: 1, Content: "source"}, {ID: "a", SessionID: "fold", Role: domain.RoleAssistant, CreatedAt: 2, Content: "answer", ContentOrigin: tc.origin}, {ID: "tail", SessionID: "fold", Role: domain.RoleUser, CreatedAt: 3, Content: "tail"}}
			if tc.notebook {
				rows[0].ContentOrigin = "notebook"
				rows[0].ExcludeAutomaticIngest = true
			}
			for _, row := range rows {
				if err := b.AppendMessage(ctx, row); err != nil {
					t.Fatal(err)
				}
			}
			if err := b.SaveSessionCompaction(ctx, storage.SessionCompaction{SessionID: "fold", RunID: "fold-run", Summary: "existing stored summary bytes", TailFrom: 2, CreatedAt: 4}); err != nil {
				t.Fatal(err)
			}
			stored, err := b.ListMessages(ctx, "fold")
			if err != nil {
				t.Fatal(err)
			}
			folded, ok := svc.foldSessionHistory(ctx, "fold", stored)
			if !ok || len(folded) != 2 {
				t.Fatalf("fold failed %+v", folded)
			}
			if folded[0].ContentOrigin != tc.wantOrigin || folded[0].Content != compactionSummaryPrefix+"existing stored summary bytes" {
				t.Fatalf("summary provenance/content %+v", folded[0])
			}
			wire, err := json.Marshal(projectFeed(folded))
			if err != nil {
				t.Fatal(err)
			}
			var restored []*schema.Message
			if err := json.Unmarshal(wire, &restored); err != nil {
				t.Fatal(err)
			}
			p, err := svc.modelInputProvenance(ctx, "fold-run", restored)
			if err != nil || p.ContentOrigin != tc.wantOrigin {
				t.Fatalf("summary JSON recertified: %+v %v", p, err)
			}
			after, err := b.ListMessages(ctx, "fold")
			if err != nil {
				t.Fatal(err)
			}
			beforeDigest, _ := storage.SubjectEvidenceDigest(stored)
			afterDigest, _ := storage.SubjectEvidenceDigest(after)
			if beforeDigest != afterDigest {
				t.Fatal("legacy originals mutated")
			}
		})
	}
}

func TestIssue58RawInvalidUTF8CannotBeRecertifiedByJournal(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "mapper", true: "native-generate"}[native], func(t *testing.T) {
			ctx := context.Background()
			svc, b, _ := newTestService(t, testsupport.NewEchoModel())
			conformance.Issue58Identity(t, b)
			source := conformance.Issue58Source("raw-invalid", false)
			mustCreateSession(t, b, source.SessionID)
			svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
			a := conformance.Issue58Admission(source, "raw-invalid-run")
			if _, err := b.CommitRunAdmission(ctx, a); err != nil {
				t.Fatal(err)
			}
			raw := append([]byte(`{"value":"`), 0xff)
			raw = append(raw, []byte(`"}`)...)
			msg := schema.AssistantMessage("", []schema.ToolCall{{ID: "invalid-raw", Function: schema.FunctionCall{Name: "issue58_exact", Arguments: string(raw)}}})
			m := newEventMapper(a.Run.ID, 64<<10)
			m.runProvider = "fixture"
			m.runModel = "fixture"
			if native {
				observed := svc.withLiveModelStreamObserver(ctx, m, source.SessionID, nil, nil, false, nil)
				response, err := observeGenerate(observed, &observedModelCallCore{}, NewScriptedModel(msg), []*schema.Message{schema.UserMessage("test")})
				if !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) || response != nil {
					t.Fatalf("invalid native output forwarded %v %v", response, err)
				}
			} else {
				for _, event := range m.toolCallEvents(msg) {
					if !svc.persistAndPublish(ctx, source.SessionID, event) {
						t.Fatal("invalidity marker must persist")
					}
				}
			}
			found := false
			for _, event := range replayAll(t, b, a.Run.ID) {
				if event.Type == domain.EventToolRequested {
					var marker struct {
						Invalid  bool   `json:"original_arguments_invalid_utf8"`
						Original string `json:"original_arguments"`
						Raw      []byte `json:"original_arguments_bytes"`
					}
					if err := json.Unmarshal(event.Payload, &marker); err != nil {
						t.Fatal(err)
					}
					if !marker.Invalid || marker.Original != "" || string(marker.Raw) != string(raw) {
						t.Fatalf("serialization recertified invalid bytes: %s", event.Payload)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("missing durable evidence-unavailable marker")
			}
			svc.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{}))
			if _, _, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, a.Run.ID); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
				t.Fatalf("invalid raw arguments sealed as original: %v", err)
			}
			if _, err := svc.projectedMessages(ctx, source.SessionID, a.Run.ID); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
				t.Fatalf("projection recertified invalidity: %v", err)
			}
			if _, err := b.GetToolOperation(ctx, a.Run.ID, "invalid-raw"); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("invalid raw model tool executed/admitted: %v", err)
			}
		})
	}
}

func TestIssue58NativeFinalizeRetainsProvenance(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	for _, origin := range []string{"notebook", "unknown"} {
		rawSummary := schema.AssistantMessage("generated summary", nil)
		stampInputProvenance(rawSummary, storage.InputProvenance{ContentOrigin: origin, ExcludeAutomaticIngest: origin == "notebook"})
		finalized, err := preserveProjectFileContextsFinalize(context.Background(), []*schema.Message{schema.SystemMessage("trusted instructions"), schema.UserMessage("original input")}, rawSummary)
		if err != nil {
			t.Fatal(err)
		}
		p, err := svc.modelInputProvenance(context.Background(), "native-finalize", finalized)
		if err != nil || p.ContentOrigin != origin {
			t.Fatalf("native DefaultFinalize recertified %s: %+v %v", origin, p, err)
		}
	}
}

func TestIssue58InvalidUTF8StreamRetainsArgumentPrefixAndIdentity(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "stream.db")
	svc, b := openD15CrashService(t, path)
	svc.deps.Sink = newTestSink()
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("invalid-stream", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "invalid-stream-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper(a.Run.ID, 64<<10)
	m.runProvider = "fixture"
	m.runModel = "fixture"
	index := 0
	prefix := `{"value":"`
	suffix := string([]byte{0xff}) + `"}`
	chunks := []*schema.Message{{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, ID: "stream-operation", Function: schema.FunctionCall{Name: "issue58_exact", Arguments: prefix}}}}, {Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, Function: schema.FunctionCall{Arguments: suffix}}}}}
	observed := svc.withLiveModelStreamObserver(ctx, m, source.SessionID, nil, nil, false, nil)
	out, err := observeStream(observed, &observedModelCallCore{}, &fakeChatModel{streamReader: schema.StreamReaderFromArray(chunks)}, []*schema.Message{schema.UserMessage("test")})
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := out.Recv(); err != nil {
		t.Fatal(err)
	}
	if _, err := out.Recv(); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
		t.Fatalf("invalid fragment forwarded: %v", err)
	}
	found := false
	for _, event := range replayAll(t, b, a.Run.ID) {
		if event.Type == domain.EventToolRequested {
			var p payloadToolRequested
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				t.Fatal(err)
			}
			if p.ToolCallID != "stream-operation" || p.ToolName != "issue58_exact" || string(p.OriginalArgumentsBytes) != prefix+suffix || !p.OriginalArgumentsInvalidUTF8 {
				t.Fatalf("lost exact stream arguments/identity: %+v", p)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing stream unavailable evidence")
	}
	if _, err := b.GetToolOperation(ctx, a.Run.ID, "stream-operation"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("invalid stream executed/admitted: %v", err)
	}
	svc.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{Message: "invalid streamed tool arguments"}))
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, _, err := reopened.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, a.Run.ID); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
		t.Fatalf("invalid streamed source became sealable after reopen: %v", err)
	}
	// The unavailable bytes never become an original; the run is excluded.
	issue58AssertExcludedOnly(t, reopened, source.SubjectHistoryBinding)
	persisted := false
	for _, event := range replayAll(t, reopened, a.Run.ID) {
		if event.Type == domain.EventToolRequested {
			var p payloadToolRequested
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				t.Fatal(err)
			}
			if p.ToolCallID != "stream-operation" || p.ToolName != "issue58_exact" || string(p.OriginalArgumentsBytes) != prefix+suffix {
				t.Fatalf("raw stream evidence changed after reopen: %+v", p)
			}
			persisted = true
		}
	}
	if !persisted {
		t.Fatal("raw stream evidence missing after restart")
	}
}
