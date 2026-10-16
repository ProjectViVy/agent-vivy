package conformance

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type subjectContextCapture interface {
	CaptureSubjectContext(context.Context, storage.SubjectHistoryBinding) (storage.SubjectContextSnapshot, error)
}

func contextStore(t *testing.T, b storage.Engine) subjectContextCapture {
	t.Helper()
	q, ok := b.(subjectContextCapture)
	if !ok {
		t.Fatal("backend does not capture a complete subject disposition snapshot")
	}
	return q
}
func contextSource(t *testing.T, b storage.Engine, id string) (storage.SubjectSourceBinding, storage.SubjectInteraction) {
	t.Helper()
	s := Issue58Source(domain.SessionID(id), false)
	if err := b.CreateSession(context.Background(), domain.Session{ID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	in := Issue58Completed(t, b, s, id+"-run")
	in, err := b.(storage.SubjectSourceStore).SealSubjectInteraction(context.Background(), s.SubjectHistoryBinding, in)
	if err != nil {
		t.Fatal(err)
	}
	return s, in
}
func RunIssue58SubjectContextSnapshot(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	Issue58Identity(t, b)
	s, in := contextSource(t, b, "context-archived-A")
	q := contextStore(t, b)
	before, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil {
		t.Fatal(err)
	}
	if before.Through != 1 || len(before.Dispositions) != 0 || before.DispositionDigest == "" {
		t.Fatalf("initial cut %+v", before)
	}
	if err := b.(storage.SubjectSourceStore).ArchiveRetainedSession(ctx, s.SessionID); err != nil {
		t.Fatal(err)
	}
	marker := storage.SessionTruncation{SessionID: s.SessionID, CutoffMessageID: "user_" + string(in.RunID), TailMessageID: "user_" + string(in.RunID), Reason: storage.TruncationRewind, CreatedAt: -10}
	if err := b.RecordSessionTruncation(ctx, marker); err != nil {
		t.Fatal(err)
	}
	after, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil {
		t.Fatal(err)
	}
	if after.Through != before.Through || after.DispositionDigest == before.DispositionDigest || len(after.Dispositions) != 1 {
		t.Fatalf("archived rewind missing at unchanged ceiling: %+v", after)
	}
	d := after.Dispositions[0]
	if d.MarkerID == 0 || d.Marker != marker || len(d.Targets) != 1 {
		t.Fatalf("marker identity lost %+v", d)
	}
	target := d.Targets[0]
	if target.InteractionID != in.ID || target.Sequence != 1 || target.SourceDigest != in.SourceDigest || len(target.MessageIDs) != 1 || target.MessageIDs[0] != marker.CutoffMessageID || target.ViewMessageIDs[0] != target.MessageIDs[0] {
		t.Fatalf("inexact target %+v", target)
	}
	// An edit is visible at admission, before its replacement is terminal.
	a := Issue58Admission(s, "context-replacement")
	edit := marker
	edit.RunID = a.Run.ID
	edit.Reason = storage.TruncationEdit
	edit.CreatedAt = -20
	a.Edit = &edit
	if _, err := b.(storage.RunAdmissionStore).CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	edited, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Through != 1 || len(edited.Dispositions) != 2 || edited.DispositionDigest == after.DispositionDigest {
		t.Fatalf("unsealed admission edit omitted %+v", edited)
	}
	// A private marker never participates, even if its strings name an original.
	if err := b.CreateSession(ctx, domain.Session{ID: "context-private"}); err != nil {
		t.Fatal(err)
	}
	private := marker
	private.SessionID = "context-private"
	if err := b.RecordSessionTruncation(ctx, private); err != nil {
		t.Fatal(err)
	}
	stable, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil || stable.DispositionDigest != edited.DispositionDigest {
		t.Fatalf("private view invalidated parent %+v %v", stable, err)
	}

	// Pre-binding/NoContext rows and active runs are not eligible targets.
	for _, row := range []domain.Message{{ID: "context-unpinned", SessionID: s.SessionID, Role: domain.RoleUser, Content: "private old row", CreatedAt: 5}, {ID: "context-active-only", SessionID: s.SessionID, RunID: a.Run.ID, Role: domain.RoleUser, Content: "active", CreatedAt: -5}} {
		if err := b.AppendMessage(ctx, row); err != nil {
			t.Fatal(err)
		}
		ignored := marker
		ignored.CutoffMessageID = row.ID
		ignored.TailMessageID = row.ID
		if err := b.RecordSessionTruncation(ctx, ignored); err != nil {
			t.Fatal(err)
		}
	}
	excluded, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil || excluded.DispositionDigest != stable.DispositionDigest {
		t.Fatalf("unpinned/active rows invalidated original %+v %v", excluded, err)
	}
	t.Logf("manifest archived/edit through=%d markers=%d original=%s digest=%s disposition=%s", stable.Through, len(stable.Dispositions), in.ID, in.SourceDigest, stable.DispositionDigest)
}
func RunIssue58SubjectContextScope(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	Issue58Identity(t, b)
	s, _ := contextSource(t, b, "context-scope")
	q := contextStore(t, b)
	for _, binding := range []storage.SubjectHistoryBinding{{}, {SubjectID: "foreign", DomainID: s.DomainID}} {
		got, err := q.CaptureSubjectContext(ctx, binding)
		if err == nil || got.Through != 0 || len(got.Dispositions) != 0 {
			t.Fatalf("foreign capture leaked %+v %v", got, err)
		}
	}
	marker := storage.SessionTruncation{SessionID: s.SessionID, CutoffMessageID: "missing", TailMessageID: "missing", Reason: storage.TruncationRewind}
	if err := b.RecordSessionTruncation(ctx, marker); err != nil {
		t.Fatal(err)
	}
	got, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) || got.Through != 0 {
		t.Fatalf("missing anchors fail open %+v %v", got, err)
	}
}

func RunIssue58SubjectContextCopyScope(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	Issue58Identity(t, b)
	s, in := contextSource(t, b, "context-copy-parent")
	q := contextStore(t, b)
	before, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil {
		t.Fatal(err)
	}
	original, err := b.ListMessages(ctx, s.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	parent := s.SessionID
	messages := original
	for _, name := range []string{"context-copy-child", "context-copy-nested"} {
		child := domain.Session{ID: domain.SessionID(name)}
		copies := make([]domain.Message, len(messages))
		for i, m := range messages {
			m.CopySource = &domain.MessageCopySource{SessionID: parent, MessageID: m.ID}
			m.ID = name + "-" + m.ID
			m.SessionID = child.ID
			copies[i] = m
		}
		markers := []storage.SessionTruncation{{SessionID: parent, Reason: storage.TruncationFork, ForkSessionID: name, CutoffMessageID: messages[0].ID, TailMessageID: messages[len(messages)-1].ID}, {SessionID: child.ID, Reason: storage.TruncationForkedFrom, ForkSessionID: string(parent), CutoffMessageID: copies[0].ID, TailMessageID: copies[len(copies)-1].ID}}
		if _, err := b.(storage.HistoryMutationStore).CommitSessionFork(ctx, child, copies, markers, nil); err != nil {
			t.Fatal(err)
		}
		// A private copied view cannot invalidate its parent.
		marker := storage.SessionTruncation{SessionID: child.ID, CutoffMessageID: copies[0].ID, TailMessageID: copies[0].ID, Reason: storage.TruncationRewind, CreatedAt: -1}
		if err := b.RecordSessionTruncation(ctx, marker); err != nil {
			t.Fatal(err)
		}
		if name == "context-copy-child" {
			private, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
			if err != nil || private.DispositionDigest != before.DispositionDigest {
				t.Fatalf("private copied view affected parent %+v %v", private, err)
			}
		}
		childSource := Issue58Source(child.ID, false)
		own := Issue58Completed(t, b, childSource, name+"-own")
		if _, err := b.(storage.SubjectSourceStore).SealSubjectInteraction(ctx, s.SubjectHistoryBinding, own); err != nil {
			t.Fatal(err)
		}
		parent, messages = child.ID, copies
	}
	snapshot, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Dispositions) != 2 {
		t.Fatalf("fork provenance became disposition %+v", snapshot)
	}
	for _, d := range snapshot.Dispositions {
		if d.SessionID == s.SessionID || len(d.Targets) != 1 {
			t.Fatalf("parent view mutated %+v", d)
		}
		target := d.Targets[0]
		if target.SourceSessionID != s.SessionID || target.InteractionID != in.ID || len(target.MessageIDs) != 1 || target.MessageIDs[0] != original[0].ID || target.ViewMessageIDs[0] == target.MessageIDs[0] {
			t.Fatalf("inexact nested copy target %+v", target)
		}
	}
	got, err := b.(storage.SubjectSourceStore).ReadSubjectInteraction(ctx, s.SubjectHistoryBinding, in.ID)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := storage.SubjectEvidenceDigest(got)
	if digest != in.SourceDigest {
		t.Fatal("copied-view rewind changed original digest")
	}
	t.Logf("manifest nested views=%d through=%d parent=%s original=%s disposition=%s", len(snapshot.Dispositions), snapshot.Through, in.ID, digest, snapshot.DispositionDigest)
}

func RunIssue58SubjectContextSnapshotRace(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	Issue58Identity(t, b)
	s, in := contextSource(t, b, "context-race")
	q := contextStore(t, b)
	marker := storage.SessionTruncation{SessionID: s.SessionID, CutoffMessageID: "user_" + string(in.RunID), TailMessageID: "user_" + string(in.RunID), Reason: storage.TruncationRewind, CreatedAt: 5}
	if err := b.RecordSessionTruncation(ctx, marker); err != nil {
		t.Fatal(err)
	}
	baseline, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil {
		t.Fatal(err)
	}
	unrelated := Issue58Completed(t, b, s, "context-unrelated")
	if _, err := b.(storage.SubjectSourceStore).SealSubjectInteraction(ctx, s.SubjectHistoryBinding, unrelated); err != nil {
		t.Fatal(err)
	}
	stable, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil || stable.Through != 2 || stable.DispositionDigest != baseline.DispositionDigest {
		t.Fatalf("moving ceiling changed disposition %+v %v", stable, err)
	}
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 12; i++ {
			a := Issue58Admission(s, fmt.Sprintf("context-concurrent-%d", i))
			m := marker
			m.Reason = storage.TruncationEdit
			m.RunID = a.Run.ID
			m.CreatedAt = int64(-i)
			a.Edit = &m
			if _, err := b.(storage.RunAdmissionStore).CommitRunAdmission(ctx, a); err != nil {
				done <- err
				return
			}
			seq, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{{RunID: a.Run.ID, Type: domain.EventRunCompleted, PayloadVersion: 1, CreatedAt: int64(-i), Payload: []byte(`{"outcome":"completed"}`)}}})
			if err != nil {
				done <- err
				return
			}
			if err = b.SetRunStatus(ctx, a.Run.ID, domain.RunCompleted); err != nil {
				done <- err
				return
			}
			digest, err := storage.SubjectEvidenceDigest([]domain.Message{a.Message})
			if err != nil {
				done <- err
				return
			}
			sealed, err := b.(storage.SubjectSourceStore).SealSubjectInteraction(ctx, s.SubjectHistoryBinding, storage.SubjectInteraction{ID: storage.SubjectInteractionID(a.Run.ID, seq), SessionID: s.SessionID, RunID: a.Run.ID, TerminalSeq: seq, SourceDigest: digest})
			if err != nil {
				done <- err
				return
			}

			// A negative rewind timestamp is an explicit fixture label for the
			// preceding durable seal, not a chronological ordering rule.
			rewind := marker
			rewind.CreatedAt = -int64(sealed.Sequence)
			if err := b.RecordSessionTruncation(ctx, rewind); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for i := 0; i < 24; i++ {
		snapshot, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
		if errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
			if snapshot.Through != 0 || len(snapshot.Sources) != 0 {
				t.Fatalf("pending cut leaked partial metadata %+v", snapshot)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if (snapshot.Through < 2 || snapshot.Through > 14) || len(snapshot.Dispositions) < 1 {
			t.Fatalf("incoherent ceiling %+v", snapshot)
		}
		for _, d := range snapshot.Dispositions {

			if d.Marker.Reason == storage.TruncationRewind && d.Marker.CreatedAt < 0 && uint64(-d.Marker.CreatedAt) > snapshot.Through {
				t.Fatalf("mixed pre-seal ceiling with post-seal marker %+v", snapshot)
			}
			if len(d.Targets) != 1 || d.Targets[0].Sequence != 1 || d.Targets[0].MessageIDs[0] != marker.CutoffMessageID {
				t.Fatalf("overlapping union changed exact target %+v", d)
			}
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	final, err := q.CaptureSubjectContext(ctx, s.SubjectHistoryBinding)
	if err != nil || len(final.Dispositions) != 25 || final.Through != 14 {
		t.Fatalf("lost overlapping/equal/backward markers %+v %v", final, err)
	}
}

// Mutating retained metadata here models a corrupt/incomplete copy mapping;
// the producer must return no partial cut rather than guessing from bodies.
func RunIssue58SubjectContextInvalidMapping(t *testing.T, b storage.Engine, removeLink func(string) error) {
	RunIssue58SubjectContextCopyScope(t, b)
	rows, err := b.ListMessages(context.Background(), "context-copy-nested")
	if err != nil || len(rows) == 0 {
		t.Fatalf("nested fixture %+v %v", rows, err)
	}
	if err := removeLink(rows[0].ID); err != nil {
		t.Fatal(err)
	}
	snapshot, err := contextStore(t, b).CaptureSubjectContext(context.Background(), Issue58Source("context-copy-parent", false).SubjectHistoryBinding)
	if !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) || snapshot.Through != 0 || len(snapshot.Sources) != 0 || len(snapshot.Dispositions) != 0 {
		t.Fatalf("inconsistent copy mapping returned partial capture %+v %v", snapshot, err)
	}
}
