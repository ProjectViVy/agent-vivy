package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func RunIssue58SubjectRetention(t *testing.T, b storage.Engine) {
	t.Helper()
	ctx := context.Background()
	Issue58Identity(t, b)
	source := Issue58Source("retention-source", false)
	if err := b.CreateSession(ctx, domain.Session{ID: source.SessionID}); err != nil {
		t.Fatal(err)
	}
	a := Issue58Admission(source, "retention-run")
	a.Message.Attachments = []domain.Attachment{{Name: "exact", MimeType: "application/octet-stream", Data: []byte{0, 255, 1}}}
	a.Message.FileContexts = []domain.FileContext{{Path: "/original", Name: "original", Size: 8, Content: []byte("snapshot")}}
	if _, err := b.(storage.RunAdmissionStore).CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	args := []byte("{ \"authorized\" : \"original input\" }")
	requested, _ := json.Marshal(map[string]any{"tool_call_id": "retention-call", "tool_name": "retention-tool", "args": map[string]any{"authorized": "original input"}, "original_arguments": string(args)})
	seq, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{{RunID: a.Run.ID, Type: domain.EventToolRequested, CreatedAt: 2, PayloadVersion: 1, Payload: requested}}})
	if err != nil {
		t.Fatal(err)
	}
	requestMessage := domain.Message{ID: fmt.Sprintf("msgp_%s_%020d_1", a.Run.ID, seq), RunID: a.Run.ID, SessionID: source.SessionID, Role: domain.RoleAssistant, CreatedAt: 2, ToolCallID: "retention-call", ToolName: "retention-tool", ToolArgs: []byte(`{"authorized":"original input"}`)}
	if _, err := b.AppendMessageIfAbsent(ctx, requestMessage); err != nil {
		t.Fatal(err)
	}
	ops := b.(storage.ToolOperationStore)
	private := []byte(`{"authorized":"original input","resolved_private":"synthetic-private-value"}`)
	// The model request and private invocation bytes intentionally differ.
	op := domain.ToolOperation{RunID: a.Run.ID, OperationID: "retention-call", ToolName: "retention-tool", RequestDigest: evidenceHash(args), ArgumentsDigest: evidenceHash(private), MiddlewareInputArguments: private, EffectiveArguments: private, CreatedAt: 3}
	op, _, _, err = ops.AdmitToolOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Blobs().Put(ctx, storage.ToolEvidenceArgumentsID(op), args); err != nil {
		t.Fatal(err)
	}
	op, acquired, _, err := ops.ClaimToolOperation(ctx, a.Run.ID, op.OperationID, "owner")
	if err != nil || !acquired {
		t.Fatalf("claim: %v %v", acquired, err)
	}
	evidence := storage.ToolEvidence{Result: strings.Repeat("exact tool output\n", 10000), Failure: "exact partial failure", ModelProjection: "bounded public diagnostic", ModelFailure: "exact partial failure"}
	raw, _ := json.Marshal(evidence)
	ref := storage.NewToolEvidenceReference(op, raw, evidence.ModelProjection)
	if err := b.Blobs().Put(ctx, ref.ID, raw); err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(ref)
	if _, _, err := ops.CompleteToolOperation(ctx, a.Run.ID, op.OperationID, "owner", string(receipt), evidence.ModelFailure); err != nil {
		t.Fatal(err)
	}
	finished, _ := json.Marshal(map[string]any{"tool_call_id": op.OperationID, "tool_name": op.ToolName, "result": "", "error": evidence.ModelFailure})
	seq, err = b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{{RunID: a.Run.ID, Type: domain.EventToolFinished, CreatedAt: 4, PayloadVersion: 1, Payload: finished}}})
	if err != nil {
		t.Fatal(err)
	}
	projected := domain.Message{ID: fmt.Sprintf("msgp_%s_%020d_0", a.Run.ID, seq), SessionID: source.SessionID, RunID: a.Run.ID, Role: domain.RoleTool, CreatedAt: 4, ToolCallID: op.OperationID, ToolName: op.ToolName, Content: evidence.ModelFailure}
	if _, err := b.AppendMessageIfAbsent(ctx, projected); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{{RunID: a.Run.ID, Type: domain.EventRunFailed, CreatedAt: 5, PayloadVersion: 1, Payload: []byte(`{"outcome":"failed"}`)}}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetRunStatus(ctx, a.Run.ID, domain.RunFailed); err != nil {
		t.Fatal(err)
	}
	sources := b.(storage.SubjectSourceStore)
	rows, in, err := sources.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, a.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || string(rows[1].ToolArgs) != string(args) || rows[2].Content != evidence.Result || rows[2].ToolError != evidence.Failure {
		t.Fatalf("exact evidence expansion lost originals: %+v", rows)
	}
	encoded, _ := json.Marshal(rows)
	if strings.Contains(string(encoded), "synthetic-private-value") {
		t.Fatal("private effective arguments escaped original boundary")
	}
	sealed, err := sources.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, in)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := sources.ArchiveRetainedSession(ctx, source.SessionID); err != nil {
			t.Fatal(err)
		}
	}
	got, err := sources.ReadSubjectInteraction(ctx, source.SubjectHistoryBinding, in.ID)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := storage.SubjectEvidenceDigest(got)
	if err != nil || digest != in.SourceDigest {
		t.Fatalf("archive changed source hash %s %v", digest, err)
	}
	if _, err := sources.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, sealed); err != nil {
		t.Fatal(err)
	}
	if err := b.DeleteSession(ctx, source.SessionID); !errors.Is(err, storage.ErrRetainedSession) {
		t.Fatalf("protected originals deleted: %v", err)
	}
	if _, found, err := b.Blobs().Get(ctx, ref.ID); err != nil || !found {
		t.Fatalf("archive broke artifact ownership: %v", err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "unprotected-retention-control"}); err != nil {
		t.Fatal(err)
	}
	if err := b.DeleteSession(ctx, "unprotected-retention-control"); err != nil {
		t.Fatalf("ordinary deletion changed: %v", err)
	}
	t.Logf("manifest before/after archive %s %s artifact %s %s %d", in.ID, in.SourceDigest, ref.ID, ref.SHA256, ref.ByteCount)
}

func evidenceHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func RunIssue58MissingEvidenceBlocksNewSeal(t *testing.T, b storage.Engine) {
	t.Helper()
	ctx := context.Background()
	Issue58Identity(t, b)
	source := Issue58Source("missing-evidence-source", false)
	if err := b.CreateSession(ctx, domain.Session{ID: source.SessionID}); err != nil {
		t.Fatal(err)
	}
	first := Issue58Admission(source, "missing-evidence-run")
	if _, err := b.(storage.RunAdmissionStore).CommitRunAdmission(ctx, first); err != nil {
		t.Fatal(err)
	}
	// Crash after exact Journal completion, before the deterministic assistant
	// projection was repaired. The active source below must not block sealing.
	content := "recoverable exact original"
	complete, _ := json.Marshal(map[string]any{"byte_len": len(content), "content_sha256": evidenceHash([]byte(content))})
	if _, err := b.Append(ctx, storage.Commit{RunID: first.Run.ID, Events: []domain.RunEvent{
		{RunID: first.Run.ID, Type: domain.EventModelDelta, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{"delta":"recoverable exact original"}`)},
		{RunID: first.Run.ID, Type: domain.EventModelCompleted, CreatedAt: 3, PayloadVersion: 2, Payload: complete},
		{RunID: first.Run.ID, Type: domain.EventRunCompleted, CreatedAt: 4, PayloadVersion: 1, Payload: []byte(`{"outcome":"completed"}`)},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetRunStatus(ctx, first.Run.ID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	active := Issue58Admission(source, "current-unfinished-run")
	if _, err := b.(storage.RunAdmissionStore).CommitRunAdmission(ctx, active); err != nil {
		t.Fatal(err)
	}

	next := Issue58Completed(t, b, source, "later-complete-run")
	sources := b.(storage.SubjectSourceStore)
	if _, err := sources.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, next); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
		t.Fatalf("index advanced past missing original: %v", err)
	}
	high, err := sources.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || high != 0 {
		t.Fatalf("missing evidence consumed prefix sequence: %d %v", high, err)
	}
	repaired := domain.Message{ID: fmt.Sprintf("msgp_%s_%020d_0", first.Run.ID, 3), SessionID: source.SessionID, RunID: first.Run.ID, Role: domain.RoleAssistant, CreatedAt: 3, Content: content}
	if _, err := b.AppendMessageIfAbsent(ctx, repaired); err != nil {
		t.Fatal(err)
	}
	_, firstInteraction, err := sources.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, first.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sources.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, firstInteraction)
	if err != nil || sealed.Sequence != 1 {
		t.Fatalf("repair did not unblock first source: %+v %v", sealed, err)
	}
	later, err := sources.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, next)
	if err != nil || later.Sequence != 2 {
		t.Fatalf("repair lost later source: %+v %v", later, err)
	}
	if again, err := sources.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, sealed); err != nil || again.Sequence != 1 {
		t.Fatalf("repair retry duplicated source: %+v %v", again, err)
	}
	high, err = sources.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || high != 2 {
		t.Fatalf("repair left prefix hole or duplicate: %d %v", high, err)
	}

}

func RunIssue58CopySources(t *testing.T, b storage.Engine) {
	t.Helper()
	ctx := context.Background()
	parent := domain.Session{ID: "copy-source-parent"}
	if err := b.CreateSession(ctx, parent); err != nil {
		t.Fatal(err)
	}
	originals := []domain.Message{{ID: "copy-original-a", SessionID: parent.ID, Role: domain.RoleUser, CreatedAt: 1, Content: "indistinguishable"}, {ID: "copy-original-b", SessionID: parent.ID, Role: domain.RoleUser, CreatedAt: 1, Content: "indistinguishable"}}
	for _, m := range originals {
		if err := b.AppendMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	child := domain.Session{ID: "copy-source-child"}
	copies := make([]domain.Message, 0, 2)
	for i, m := range originals {
		m.CopySource = &domain.MessageCopySource{SessionID: parent.ID, MessageID: m.ID}
		m.ID = fmt.Sprintf("copy-new-%d", i)
		m.SessionID = child.ID
		copies = append(copies, m)
	}
	markers := []storage.SessionTruncation{{SessionID: parent.ID, Reason: storage.TruncationFork, ForkSessionID: string(child.ID), CutoffMessageID: originals[1].ID, TailMessageID: originals[1].ID}, {SessionID: child.ID, Reason: storage.TruncationForkedFrom, ForkSessionID: string(parent.ID), CutoffMessageID: copies[1].ID, TailMessageID: copies[1].ID}}
	failedChild := domain.Session{ID: "copy-source-failed"}
	failedCopies := append([]domain.Message(nil), copies...)
	for i := range failedCopies {
		failedCopies[i].ID = fmt.Sprintf("failed-copy-%d", i)
		failedCopies[i].SessionID = failedChild.ID
	}
	failedCopies[1].CopySource = &domain.MessageCopySource{SessionID: parent.ID, MessageID: "missing-original"}
	failedMarkers := []storage.SessionTruncation{{SessionID: parent.ID, Reason: storage.TruncationFork, ForkSessionID: string(failedChild.ID), CutoffMessageID: originals[1].ID, TailMessageID: originals[1].ID}}
	if _, err := b.(storage.HistoryMutationStore).CommitSessionFork(ctx, failedChild, failedCopies, failedMarkers, nil); err == nil {
		t.Fatal("failed copy provenance admission accepted")
	}
	if _, err := b.GetSession(ctx, failedChild.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("failed provenance left child session: %v", err)
	}
	if _, err := b.(storage.MessageCopySourceStore).GetMessageCopySource(ctx, failedChild.ID, failedCopies[0].ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("failed provenance left first source link: %v", err)
	}
	if _, err := b.(storage.HistoryMutationStore).CommitSessionFork(ctx, child, copies, markers, nil); err != nil {
		t.Fatal(err)
	}
	origins := b.(storage.MessageCopySourceStore)
	for i, copy := range copies {
		link, err := origins.GetMessageCopySource(ctx, child.ID, copy.ID)
		if err != nil || link.SessionID != parent.ID || link.MessageID != originals[i].ID {
			t.Fatalf("exact copy source %d: %+v %v", i, link, err)
		}
	}
	if err := b.DeleteSession(ctx, parent.ID); err != nil {
		t.Fatalf("provenance restricted ordinary parent deletion: %v", err)
	}
	if link, err := origins.GetMessageCopySource(ctx, child.ID, copies[0].ID); err != nil || link.MessageID != originals[0].ID {
		t.Fatalf("deletion changed origin metadata: %+v %v", link, err)
	}
	if err := b.DeleteSession(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := origins.GetMessageCopySource(ctx, child.ID, copies[0].ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted copy kept an addressable source link: %v", err)
	}
}
