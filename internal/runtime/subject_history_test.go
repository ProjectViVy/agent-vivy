package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

// Every body here uses the actual admission, Journal, message projector, tool
// operation receipt and backend BlobStore. No alternate transcript is seeded.
func TestIssue58EvidenceCompleteAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "evidence.db")
	svc, b := openD15CrashService(t, path)
	svc.deps.Sink = newTestSink()
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("restart-source", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "restart-run")
	a.Message.Attachments = []domain.Attachment{{Name: "evidence.bin", MimeType: "application/octet-stream", Data: []byte{0, 1, 2, 255}}}
	a.Message.FileContexts = []domain.FileContext{{Path: "/owned/file.txt", Name: "file.txt", Size: 8, Content: []byte("snapshot")}}
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper(a.Run.ID, 64<<10)
	persist := func(e domain.RunEvent) {
		t.Helper()
		if !svc.persistAndPublish(ctx, source.SessionID, e) {
			t.Fatal("Journal append failed")
		}
	}
	persist(m.build(domain.EventModelDelta, payloadModelDelta{Delta: "assistant intermediate"}))
	args := []byte("{ \"number\" : 42 }")
	persist(m.build(domain.EventToolRequested, payloadToolRequested{ToolCallID: "restart-call", ToolName: "issue58_exact", Args: map[string]any{"number": 42}, OriginalArguments: string(args)}))
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	op, err := c.Admit(ctx, "restart-call", "issue58_exact", args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Execute(ctx, op, func(context.Context) (string, error) { return "exact tool result", nil })
	if err != nil {
		t.Fatal(err)
	}
	persist(m.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: "restart-call", ToolName: "issue58_exact", Result: result}))
	correction := domain.Message{ID: "steer-correction", RunID: a.Run.ID, SessionID: source.SessionID, Role: domain.RoleUser, CreatedAt: 2, Content: "user correction"}
	if _, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{m.build(domain.EventTurnSteered, payloadTurnSteered{QueueID: "correction", Text: correction.Content})}, Messages: []domain.Message{correction}}); err != nil {
		t.Fatal(err)
	}
	persist(m.build(domain.EventModelDelta, payloadModelDelta{Delta: "assistant final exact"}))
	completion := m.build(domain.EventModelCompleted, v2Completion("assistant final exact"))
	completion.PayloadVersion = 2
	persist(completion)
	svc.emitTerminal(ctx, m, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
	rows, interaction, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, a.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 || rows[0].Content != a.Message.Content || string(rows[0].Attachments[0].Data) != string(a.Message.Attachments[0].Data) || string(rows[0].FileContexts[0].Content) != "snapshot" {
		t.Fatalf("incomplete originals: %+v", rows)
	}
	var call, tool, mid, final, steer bool
	for _, row := range rows {
		switch {
		case row.ToolCallID == "restart-call" && row.Role == domain.RoleAssistant:
			call = string(row.ToolArgs) == string(args)
		case row.Role == domain.RoleTool:
			tool = row.Content == "exact tool result"
		case row.Content == "assistant intermediate":
			mid = true
		case row.Content == "assistant final exact":
			final = true
		case row.Content == "user correction":
			steer = true
		}
	}
	if !call || !tool || !mid || !final || !steer {
		t.Fatalf("original class missing call=%t tool=%t intermediate=%t final=%t steer=%t", call, tool, mid, final, steer)
	}
	t.Logf("manifest before restart %s %s", interaction.ID, interaction.SourceDigest)
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
		t.Fatalf("restart changed source: %s %v", digest, err)
	}
	t.Logf("manifest after restart %s %s", interaction.ID, digest)
	// A sealed exact result cannot silently become a shortened original on loss.
	op, err = reopened.GetToolOperation(ctx, a.Run.ID, "restart-call")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := storage.DecodeToolEvidenceReference(op)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Blobs().Delete(ctx, ref.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.ReadSubjectInteraction(ctx, source.SubjectHistoryBinding, interaction.ID); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
		t.Fatalf("missing artifact yielded successful original: %v", err)
	}
}

func TestIssue58TerminalPartialEvidenceLabeled(t *testing.T) {
	for _, status := range []domain.RunStatus{domain.RunFailed, domain.RunCancelled} {
		t.Run(string(status), func(t *testing.T) {
			ctx := context.Background()
			svc, b, _ := newTestService(t, testsupport.NewEchoModel())
			conformance.Issue58Identity(t, b)
			source := conformance.Issue58Source("partial-source", false)
			mustCreateSession(t, b, source.SessionID)
			svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
			a := conformance.Issue58Admission(source, "partial-run")
			if _, err := b.CommitRunAdmission(ctx, a); err != nil {
				t.Fatal(err)
			}
			m := newEventMapper(a.Run.ID, 64<<10)
			trajCommit(t, ctx, b, a.Run.ID, m.build(domain.EventModelDelta, payloadModelDelta{Delta: "known incomplete assistant"}))
			typ := domain.EventRunFailed
			if status == domain.RunCancelled {
				typ = domain.EventRunCancelled
			}
			if _, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{m.build(typ, map[string]string{"outcome": string(status)})}}); err != nil {
				t.Fatal(err)
			}
			if err := b.SetRunStatus(ctx, a.Run.ID, status); err != nil {
				t.Fatal(err)
			}
			if err := svc.RepairSubjectHistory(ctx, source.SessionID); err != nil {
				t.Fatal(err)
			}
			rows, in, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, a.Run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 || rows[1].Content != "known incomplete assistant" || rows[1].Source != "journal_partial" {
				t.Fatalf("partial evidence not labeled: %+v", rows)
			}
			if in.Sequence != 0 {
				t.Fatal("reader invented sequence")
			}
		})
	}
}

func TestIssue58ArchiveRetainsSources(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("archive-source", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	rid, err := svc.Run(ctx, source.SessionID, "original archived user")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not finish")
	}
	rows, in, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, rid)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSession(ctx, source.SessionID); !errors.Is(err, storage.ErrRetainedSession) {
		t.Fatalf("UI removal deleted retained original: %v", err)
	}
	if err := b.ArchiveRetainedSession(ctx, source.SessionID); err != nil {
		t.Fatal(err)
	}
	listed, err := b.ListSessions(ctx)
	if err != nil || len(listed) != 0 {
		t.Fatalf("archive still listed: %+v %v", listed, err)
	}
	if _, err := b.GetSession(ctx, source.SessionID); err != nil {
		t.Fatalf("archive no longer addressable: %v", err)
	}
	got, err := b.ReadSubjectInteraction(ctx, source.SubjectHistoryBinding, in.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(rows)
	after, _ := json.Marshal(got)
	if string(before) != string(after) {
		t.Fatal("archive changed source bytes")
	}
	t.Logf("manifest before/after archive %s %s", in.ID, in.SourceDigest)
	if err := b.DeleteSession(ctx, source.SessionID); !errors.Is(err, storage.ErrRetainedSession) {
		t.Fatalf("direct deletion removed archive: %v", err)
	}
}

func TestIssue58CompletedReceiptRepairsMissingToolFinished(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("receipt-repair", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "receipt-repair-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper(a.Run.ID, 64<<10)
	trajCommit(t, ctx, b, a.Run.ID, m.build(domain.EventToolRequested, payloadToolRequested{ToolCallID: "receipt-call", ToolName: "issue58_error", Args: map[string]any{}}))
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	args := []byte(`{}`)
	op, err := c.Admit(ctx, "receipt-call", "issue58_error", args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Execute(ctx, op, func(context.Context) (string, error) {
		return "known partial tool bytes", errors.New("exact original tool error")
	})
	if err == nil {
		t.Fatal("expected tool error")
	}
	// Crash point: completion is durable, Eino's tool.finished/checkpoint is not.
	svc.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{Message: "interrupted"}))
	if err := svc.RepairSubjectHistory(ctx, source.SessionID); err != nil {
		t.Fatal(err)
	}
	rows, in, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, a.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[2].Content != "known partial tool bytes" || rows[2].ToolError != "exact original tool error" || rows[2].Source != "journal_partial" {
		t.Fatalf("known terminal receipt evidence missing: %+v", rows)
	}
	high, err := b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || high != 1 {
		t.Fatalf("complete known partial evidence unsealed: %d %v", high, err)
	}
	t.Logf("manifest partial receipt %s %s", in.ID, in.SourceDigest)
}

func TestIssue58ForkReplayRewindIdentity(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("fork-source", false)
	mustCreateSession(t, b, source.SessionID)
	sources := map[domain.SessionID]storage.SubjectSourceBinding{source.SessionID: source}
	svc.deps.SubjectSources = func(_ context.Context, sid domain.SessionID) (*storage.SubjectSourceBinding, error) {
		s, ok := sources[sid]
		if !ok {
			return nil, nil
		}
		return &s, nil
	}
	rid, err := svc.Run(ctx, source.SessionID, "canonical original user")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not settle")
	}
	original, in, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, rid)
	if err != nil {
		t.Fatal(err)
	}
	fork, err := svc.ForkSession(ctx, source.SessionID, original[len(original)-1].ID, "evidence fork")
	if err != nil {
		t.Fatal(err)
	}
	child := domain.SessionID(fork.SessionID)
	childSource := conformance.Issue58Source(child, false)
	sources[child] = childSource
	if err := svc.RepairSubjectHistory(ctx, child); err != nil {
		t.Fatal(err)
	}
	high, err := b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || high != 1 {
		t.Fatalf("fork copy resealed parent evidence: %d %v", high, err)
	}
	hs := NewHistoryService(b, b)
	scope, err := domain.NewAcceptedHistoryScope(child, []domain.SessionID{source.SessionID, child})
	if err != nil {
		t.Fatal(err)
	}
	childRows, err := b.ListMessages(ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	page, err := hs.Read(WithHistoryScope(tools.WithSessionID(ctx, child), scope), domain.HistoryReadRequest{Selection: &domain.HistorySelection{SourceSessionID: child, Refs: []domain.SourceRef{{SessionID: child, RunID: childRows[0].RunID, MessageID: childRows[0].ID, Kind: string(domain.SourceKindMessage), CreatedAt: childRows[0].CreatedAt}}}})
	if err != nil {
		t.Fatal(err)
	}
	canonical := false
	for _, item := range page.Items {
		for _, ref := range item.SourceRefs {
			if ref.SessionID == source.SessionID && ref.MessageID == original[0].ID {
				canonical = true
			}
		}
	}
	if !canonical {
		t.Fatalf("fork history lost canonical original identity: %+v", page)
	}
	branchRun, err := svc.Run(ctx, child, "new branch interaction")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("branch did not settle")
	}
	if branchRun == rid {
		t.Fatal("branch reused parent run identity")
	}
	high, err = b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || high != 2 {
		t.Fatalf("new branch evidence did not seal once: %d %v", high, err)
	}
	if _, err := svc.RewindSession(ctx, source.SessionID, original[0].ID); err != nil {
		t.Fatal(err)
	}
	got, err := b.ReadSubjectInteraction(ctx, source.SubjectHistoryBinding, in.ID)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := storage.SubjectEvidenceDigest(got)
	if digest != in.SourceDigest {
		t.Fatal("rewind mutated canonical original digest")
	}
	highAfter, err := b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || highAfter != high {
		t.Fatalf("rewind regressed cursor: %d %v", highAfter, err)
	}
	t.Logf("manifest before/after fork and rewind %s %s", in.ID, digest)
}

type issue58MainResolver struct {
	binding cognitivecontract.MainConversationBinding
}

func (m issue58MainResolver) ResolveMainConversation(context.Context) (cognitivecontract.MainConversationBinding, error) {
	return m.binding, nil
}
func issue58AuthorizeReader(svc *Service, s storage.SubjectSourceBinding) {
	main := conformance.Issue58Source("main-reader", true)
	svc.deps.MainConversation = issue58MainResolver{binding: cognitivecontract.MainConversationBinding{SubjectID: main.SubjectID, DomainID: main.DomainID, ConversationID: main.ConversationID, ActiveSessionID: main.SessionID, PolicyID: main.PolicyID, ToolPolicyRef: main.ToolPolicyRef, SchemaVersion: main.SchemaVersion, Grant: main.Grant}}
}
func TestIssue58RewindDispositionDoesNotMutateOriginal(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("disposition-source", false)
	mustCreateSession(t, b, source.SessionID)
	allowed := true
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) {
		if !allowed {
			return nil, nil
		}
		return &source, nil
	}
	issue58AuthorizeReader(svc, source)
	rid, err := svc.Run(ctx, source.SessionID, "superseded user fact")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not settle")
	}
	rows, in, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, rid)
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.ReadSubjectOriginal(ctx, "main-reader", in.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Disposition[rows[0].ID] != "included" {
		t.Fatalf("unexpected initial disposition: %+v", before)
	}
	if _, err := svc.RewindSession(ctx, source.SessionID, rows[0].ID); err != nil {
		t.Fatal(err)
	}
	after, err := svc.ReadSubjectOriginal(ctx, "main-reader", in.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SourceDigest != before.SourceDigest || after.Disposition[rows[0].ID] != "superseded" || len(after.Supersession) != 1 || after.Outcome != domain.RunCompleted {
		t.Fatalf("rewind lost immutable source or supersession: %+v", after)
	}
	if err := svc.ArchiveSubjectSource(ctx, source.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReadSubjectOriginal(ctx, "main-reader", in.ID); err != nil {
		t.Fatalf("archive lost source navigation: %v", err)
	}
	allowed = false
	if _, err := svc.ReadSubjectOriginal(ctx, "main-reader", in.ID); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("membership replaced current read grant: %v", err)
	}
	if _, err := svc.ReadSubjectOriginal(ctx, source.SessionID, in.ID); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("ordinary contribution acquired whole-domain reads: %v", err)
	}
}

func TestIssue58ExactCopyProvenanceAcrossRestartAndArchive(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "copy-origin.db")
	svc, b := openD15CrashService(t, path)
	svc.deps.Sink = newTestSink()
	svc.deps.Truncations = b
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("identical-parent", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	admission := conformance.Issue58Admission(source, "identical-run")
	admission.Message.Content = "indistinguishable original"
	if _, err := b.CommitRunAdmission(ctx, admission); err != nil {
		t.Fatal(err)
	}
	twin := admission.Message
	twin.ID = "steer-identical" // Every other persisted field is identical.
	m := newEventMapper(admission.Run.ID, 64<<10)
	if _, err := b.Append(ctx, storage.Commit{RunID: admission.Run.ID, Events: []domain.RunEvent{m.build(domain.EventTurnSteered, payloadTurnSteered{QueueID: "identical", Text: twin.Content})}, Messages: []domain.Message{twin}}); err != nil {
		t.Fatal(err)
	}
	svc.emitTerminal(ctx, m, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
	_, interaction, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, admission.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	parents, err := b.ListMessages(ctx, source.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	fork, err := svc.ForkSession(ctx, source.SessionID, parents[len(parents)-1].ID, "identical fork")
	if err != nil {
		t.Fatal(err)
	}
	child := domain.SessionID(fork.SessionID)
	copies, err := b.ListMessages(ctx, child)
	if err != nil || len(copies) != 2 {
		t.Fatalf("copies: %d %v", len(copies), err)
	}
	parentIDs := map[string]bool{}
	for _, copy := range copies {
		link, err := b.GetMessageCopySource(ctx, child, copy.ID)
		if err != nil || link.SessionID != source.SessionID {
			t.Fatalf("equal content lost exact parent identity: %+v %v", link, err)
		}
		parentIDs[link.MessageID] = true
	}
	if len(parentIDs) != 2 || !parentIDs[admission.Message.ID] || !parentIDs[twin.ID] {
		t.Fatalf("indistinguishable originals collapsed: %+v", parentIDs)
	}
	clone, err := svc.CloneSession(ctx, child, "nested clone")
	if err != nil {
		t.Fatal(err)
	}
	nested := domain.SessionID(clone.SessionID)
	nestedRows, err := b.ListMessages(ctx, nested)
	if err != nil {
		t.Fatal(err)
	}
	expectedDirect := map[string]string{}
	expectedCanonical := map[string]string{}
	for _, row := range nestedRows {
		direct, err := b.GetMessageCopySource(ctx, nested, row.ID)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := b.GetMessageCopySource(ctx, child, direct.MessageID)
		if err != nil {
			t.Fatal(err)
		}
		expectedDirect[row.ID] = direct.MessageID
		expectedCanonical[row.ID] = canonical.MessageID
	}
	if err := b.ArchiveRetainedSession(ctx, source.SessionID); err != nil {
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
	hs := NewHistoryService(reopened, reopened)
	scope, err := domain.NewAcceptedHistoryScope(nested, []domain.SessionID{source.SessionID, child, nested})
	if err != nil {
		t.Fatal(err)
	}
	read := func(scope domain.AcceptedHistoryScope) domain.HistoryPage {
		t.Helper()
		refs := make([]domain.SourceRef, 0, 2)
		for _, row := range nestedRows {
			refs = append(refs, domain.SourceRef{SessionID: nested, RunID: row.RunID, MessageID: row.ID, Kind: string(domain.SourceKindMessage), CreatedAt: row.CreatedAt})
		}
		page, err := hs.Read(WithHistoryScope(tools.WithSessionID(ctx, nested), scope), domain.HistoryReadRequest{Selection: &domain.HistorySelection{SourceSessionID: nested, Refs: refs}})
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	page := read(scope)
	if len(page.Items) != 2 {
		t.Fatalf("nested history unavailable: %+v", page)
	}
	for _, item := range page.Items {
		if item.OriginStatus != "ok" || len(item.SourceRefs) != 2 || item.SourceRefs[0].MessageID != expectedDirect[item.Ref.MessageID] || item.SourceRefs[1].MessageID != expectedCanonical[item.Ref.MessageID] {
			t.Fatalf("nested origin not exact after restart/archive: %+v", item)
		}
	}
	deniedScope, _ := domain.NewAcceptedHistoryScope(nested, []domain.SessionID{source.SessionID, nested})
	denied := read(deniedScope)
	if len(denied.Items) != 2 || denied.Items[0].OriginStatus != "forbidden" || len(denied.Items[0].SourceRefs) != 0 {
		t.Fatalf("origin link bypassed direct parent grant: %+v", denied)
	}
	original, err := reopened.ReadSubjectInteraction(ctx, source.SubjectHistoryBinding, interaction.ID)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := storage.SubjectEvidenceDigest(original)
	high, err := reopened.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || high != 1 || digest != interaction.SourceDigest {
		t.Fatalf("copy/archive resealed or changed canonical original: %d %s %v", high, digest, err)
	}
	t.Logf("manifest exact copy before/after nested fork/restart/archive %s %s", interaction.ID, digest)
}

func TestIssue58HistoricalCopyOriginUnavailableAndOrdinaryDeleteUnchanged(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	mustCreateSession(t, b, "ordinary-parent")
	if err := b.CreateRun(ctx, domain.Run{ID: "ordinary-parent-run", SessionID: "ordinary-parent", Status: domain.RunCompleted}); err != nil {
		t.Fatal(err)
	}
	original := domain.Message{ID: "ordinary-original", SessionID: "ordinary-parent", RunID: "ordinary-parent-run", Role: domain.RoleUser, Content: "ordinary exact", CreatedAt: 1}
	if err := b.AppendMessage(ctx, original); err != nil {
		t.Fatal(err)
	}
	fork, err := svc.ForkSession(ctx, original.SessionID, original.ID, "ordinary fork")
	if err != nil {
		t.Fatal(err)
	}
	child := domain.SessionID(fork.SessionID)
	rows, err := b.ListMessages(ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.DeleteSession(ctx, original.SessionID); err != nil {
		t.Fatalf("copy provenance changed ordinary parent deletion: %v", err)
	}
	if link, err := b.GetMessageCopySource(ctx, child, rows[0].ID); err != nil || link.MessageID != original.ID {
		t.Fatalf("parent deletion rewrote origin identity: %+v %v", link, err)
	}
	// A historical copy admitted without the new optional provenance metadata
	// stays readable, but no body comparison invents a canonical original ID.
	historical := domain.Session{ID: "historical-copy"}
	copy := original
	copy.ID = "historical-copy-message"
	copy.SessionID = historical.ID
	if _, err := b.CommitSessionFork(ctx, historical, []domain.Message{copy}, nil, nil); err != nil {
		t.Fatal(err)
	}
	hs := NewHistoryService(b, b)
	ref := domain.SourceRef{SessionID: copy.SessionID, RunID: copy.RunID, MessageID: copy.ID, Kind: string(domain.SourceKindMessage), CreatedAt: copy.CreatedAt}
	page, err := hs.Read(tools.WithSessionID(ctx, copy.SessionID), domain.HistoryReadRequest{Selection: &domain.HistorySelection{SourceSessionID: copy.SessionID, Refs: []domain.SourceRef{ref}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].OriginStatus != "unavailable" || len(page.Items[0].SourceRefs) != 0 {
		t.Fatalf("historical origin guessed from equal content: %+v", page)
	}
}
