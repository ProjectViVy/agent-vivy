package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func Issue58Source(sid domain.SessionID, main bool) storage.SubjectSourceBinding {
	s := storage.SubjectSourceBinding{SubjectHistoryBinding: storage.SubjectHistoryBinding{SubjectID: "11111111-1111-4111-8111-111111111111", DomainID: "subject-personal-v1"}, SessionID: sid, SchemaVersion: 1, PolicyID: "coding-v1", ToolPolicyRef: "generation/tools/v1", Scope: "personal", Grant: storage.SubjectGrantSnapshot{Reference: "host/personal-sources", Version: 1, Snapshot: json.RawMessage(`{"decision":"allow","scope":"personal"}`)}}
	if main {
		s.ConversationID = "main-conversation"
		s.PolicyID = "life-recap-v1"
		s.Grant.WholeDomainRead = true
	}
	return s
}

func Issue58Admission(s storage.SubjectSourceBinding, id string) storage.RunAdmission {
	return storage.RunAdmission{Subject: &s, Message: domain.Message{ID: "user_" + id, SessionID: s.SessionID, RunID: domain.RunID(id), Role: domain.RoleUser, CreatedAt: 1, Content: "exact source " + id}, Run: domain.Run{ID: domain.RunID(id), SessionID: s.SessionID, Status: domain.RunActive, Kind: domain.RunKindPrimary, CreatedAt: 1}, Started: domain.RunEvent{RunID: domain.RunID(id), Type: domain.EventRunStarted, PayloadVersion: 1, CreatedAt: 1, Payload: []byte(`{}`)}}
}

// Issue58Identity registers the one organism identity at a native canonical
// owner path. domain.SubjectIdentity.Valid requires a platform-absolute, clean
// path, so a POSIX-only literal cannot stand in on Windows; the path emulates
// the trusted ACTMEM authority location and is never a second identity source.
func Issue58Identity(t *testing.T, b storage.Engine) {
	t.Helper()
	id := domain.SubjectIdentity{
		Version: 1, SubjectID: "11111111-1111-4111-8111-111111111111",
		DomainID: domain.SubjectPersonalDomain, AuthorityID: "22222222-2222-4222-8222-222222222222",
		OwnerPath: filepath.Join(t.TempDir(), "owner"),
	}
	if !id.Valid() {
		t.Fatal("fixture subject identity is not valid on this platform")
	}
	raw, err := json.Marshal(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Snapshot().Put(context.Background(), domain.SubjectIdentityKey, raw, 0); err != nil {
		t.Fatal(err)
	}
}

func Issue58Completed(t *testing.T, b storage.Engine, s storage.SubjectSourceBinding, id string) storage.SubjectInteraction {
	t.Helper()
	ctx := context.Background()
	a := Issue58Admission(s, id)
	if _, err := b.(storage.RunAdmissionStore).CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	event := domain.RunEvent{RunID: a.Run.ID, Type: domain.EventRunCompleted, PayloadVersion: 1, CreatedAt: 1, Payload: []byte(`{"outcome":"completed"}`)}
	seq, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{event}})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetRunStatus(ctx, a.Run.ID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	digest, err := storage.SubjectEvidenceDigest([]domain.Message{a.Message})
	if err != nil {
		t.Fatal(err)
	}
	return storage.SubjectInteraction{ID: storage.SubjectInteractionID(a.Run.ID, seq), SessionID: s.SessionID, RunID: a.Run.ID, TerminalSeq: seq, SourceDigest: digest}
}

func RunIssue58InteractionOrderAndDedup(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	q, ok := b.(storage.SubjectSourceStore)
	if !ok {
		t.Fatal("backend does not implement trusted subject metadata index")
	}
	Issue58Identity(t, b)
	s := Issue58Source("subject-order", false)
	if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	const n = 8
	inputs := make([]storage.SubjectInteraction, n)
	for i := range inputs {
		inputs[i] = Issue58Completed(t, b, s, fmt.Sprintf("subject-run-%d", i))
	}
	var wg sync.WaitGroup
	for _, in := range inputs {
		wg.Go(func() {
			if _, err := q.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, in); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	through, err := q.CaptureSubjectHistory(ctx, s.SubjectHistoryBinding)
	if err != nil || through != n {
		t.Fatalf("captured watermark %d, %v", through, err)
	}
	page, err := q.ListSubjectInteractions(ctx, s.SubjectHistoryBinding, 0, through, n)
	if err != nil || len(page) != n {
		t.Fatalf("page %v, %v", page, err)
	}
	for i, row := range page {
		if row.Sequence != uint64(i+1) {
			t.Fatalf("hole: %+v", page)
		}
		again, err := q.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, row)
		if err != nil || again != row {
			t.Fatalf("retry %+v, %v", again, err)
		}
	}
	changed := page[0]
	changed.SourceDigest = "changed"
	if _, err := q.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, changed); !errors.Is(err, storage.ErrSubjectInteractionConflict) {
		t.Fatalf("changed digest accepted: %v", err)
	}
	late := Issue58Completed(t, b, s, "late-run")
	if _, err := q.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, late); err != nil {
		t.Fatal(err)
	}
	old, err := q.ListSubjectInteractions(ctx, s.SubjectHistoryBinding, 0, through, n+1)
	if err != nil || len(old) != n {
		t.Fatalf("watermark drift %d, %v", len(old), err)
	}
	if err := b.DeleteSession(ctx, s.SessionID); !errors.Is(err, storage.ErrRetainedSession) {
		t.Fatalf("retained source deleted: %v", err)
	}
	if err := q.ArchiveRetainedSession(ctx, s.SessionID); err != nil {
		t.Fatal(err)
	}
	sessions, err := b.ListSessions(ctx)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("archive visible: %v %v", sessions, err)
	}
	messages, err := q.ReadSubjectInteraction(ctx, s.SubjectHistoryBinding, page[0].ID)
	if err != nil || len(messages) != 1 {
		t.Fatalf("archived source lost: %v %v", messages, err)
	}
}

func RunIssue58SubjectIsolation(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	q, ok := b.(storage.SubjectSourceStore)
	if !ok {
		t.Fatal("backend does not implement trusted subject metadata index")
	}
	Issue58Identity(t, b)
	s := Issue58Source("subject-isolated", true)
	if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	in := Issue58Completed(t, b, s, "isolated-run")
	if _, err := q.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, in); err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []storage.SubjectHistoryBinding{{SubjectID: "foreign", DomainID: s.DomainID}, {SubjectID: s.SubjectID, DomainID: "restricted"}, {}} {
		rows, err := q.ListSubjectInteractions(ctx, foreign, 0, 0, 10)
		if err == nil || len(rows) != 0 {
			t.Fatalf("foreign list leaked %v %v", rows, err)
		}
		messages, err := q.ReadSubjectInteraction(ctx, foreign, in.ID)
		if err == nil || len(messages) != 0 {
			t.Fatalf("foreign read leaked %v %v", messages, err)
		}
		count, err := q.CaptureSubjectHistory(ctx, foreign)
		if err == nil || count != 0 {
			t.Fatalf("foreign count leaked %d %v", count, err)
		}
	}
	// Admission already protects canceled/unsealed originals.
	ordinary := Issue58Source("unsealed-source", false)
	if err := b.CreateSession(ctx, domain.Session{ID: ordinary.SessionID}); err != nil {
		t.Fatal(err)
	}
	a := Issue58Admission(ordinary, "unsealed-run")
	if _, err := b.(storage.RunAdmissionStore).CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := b.DeleteSession(ctx, ordinary.SessionID); !errors.Is(err, storage.ErrRetainedSession) {
		t.Fatalf("unsealed originals not protected: %v", err)
	}
	if _, err := q.SealSubjectInteraction(ctx, ordinary.SubjectHistoryBinding, storage.SubjectInteraction{ID: "unsealed-run:1", SessionID: ordinary.SessionID, RunID: "unsealed-run", TerminalSeq: 1, SourceDigest: "made-up"}); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
		t.Fatalf("unsealed accepted: %v", err)
	}
}

func RunIssue58AdmissionBoundaries(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	q, ok := b.(storage.SubjectSourceStore)
	if !ok {
		t.Fatal("missing SubjectSourceStore")
	}
	Issue58Identity(t, b)
	admission := b.(storage.RunAdmissionStore)
	for _, test := range []string{"child", "restricted", "foreign", "read-escalation", "strategy", "no-context"} {
		t.Run(test, func(t *testing.T) {
			s := Issue58Source(domain.SessionID("denied-"+test), false)
			if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
				t.Fatal(err)
			}
			a := Issue58Admission(s, "denied-run-"+test)
			switch test {
			case "child":
				a.Run.Kind = domain.RunKindChild
				a.Run.ParentID = "parent"
			case "restricted":
				a.Subject.Scope = "restricted"
			case "foreign":
				a.Subject.SubjectID = "foreign"
			case "read-escalation":
				a.Subject.Grant.WholeDomainRead = true
			case "strategy":
				a.Run.Kind = domain.RunKindWorkflow
			case "no-context":
				a.Started.Payload = []byte(`{"no_context":true}`)
			}
			if _, err := admission.CommitRunAdmission(ctx, a); !errors.Is(err, storage.ErrSubjectBinding) {
				t.Fatalf("invalid authority accepted: %v", err)
			}
			if _, err := q.GetSubjectSource(ctx, s.SessionID); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("failed registration survived: %v", err)
			}
		})
	}
	s := Issue58Source("immutable-source", false)
	if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	original := Issue58Admission(s, "original-source-run")
	if _, err := admission.CommitRunAdmission(ctx, original); err != nil {
		t.Fatal(err)
	}
	for _, test := range []string{"grant", "policy", "domain", "nil"} {
		altered := Issue58Admission(s, "altered-source-"+test)
		switch test {
		case "grant":
			altered.Subject.Grant.Version++
		case "policy":
			altered.Subject.ToolPolicyRef = "changed"
		case "domain":
			altered.Subject.DomainID = "changed"
		case "nil":
			altered.Subject = nil
		}
		if _, err := admission.CommitRunAdmission(ctx, altered); !errors.Is(err, storage.ErrSubjectBinding) {
			t.Fatalf("immutable %s accepted: %v", test, err)
		}
	}
	// Invalid message insertion occurs after registration; the whole transaction
	// must roll back both membership and retention on that failure.
	rollback := Issue58Source("rollback-source", true)
	if err := b.CreateSession(ctx, domain.Session{ID: rollback.SessionID}); err != nil {
		t.Fatal(err)
	}
	bad := Issue58Admission(rollback, "rollback-run")
	bad.Message.ID = original.Message.ID
	if _, err := admission.CommitRunAdmission(ctx, bad); err == nil {
		t.Fatal("expected duplicate message rejection")
	}
	if _, err := q.GetSubjectSource(ctx, rollback.SessionID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rolled-back source remained: %v", err)
	}
	if err := b.DeleteSession(ctx, rollback.SessionID); err != nil {
		t.Fatalf("rolled-back admission retained source: %v", err)
	}
	// Internal storage segments share one durable conversation and never create
	// another ordinary Sessions entry.
	for i := 0; i < 2; i++ {
		main := Issue58Source(domain.SessionID(fmt.Sprintf("main-segment-%d", i)), true)
		if err := b.CreateSession(ctx, domain.Session{ID: main.SessionID}); err != nil {
			t.Fatal(err)
		}
		a := Issue58Admission(main, fmt.Sprintf("main-segment-run-%d", i))
		if _, err := admission.CommitRunAdmission(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	sessions, err := b.ListSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range sessions {
		if strings.HasPrefix(string(session.ID), "main-segment-") {
			t.Fatal("internal segment became visible main")
		}
	}
}

func RunIssue58ContinuityReplayBinding(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	Issue58Identity(t, b)
	s := Issue58Source("continuity-subject", false)
	if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	a := continuityAdmission(s.SessionID, "subject-continuity-run", "subject-continuity-message", "subject-continuity-request", "same-hash")
	a.Subject = &s
	c := b.(storage.ContinuityStore)
	if _, err := c.CommitContinuityRun(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CommitContinuityRun(ctx, a); err != nil {
		t.Fatalf("valid replay failed: %v", err)
	}
	changed := s
	changed.Grant.Version++
	a.Subject = &changed
	if _, err := c.CommitContinuityRun(ctx, a); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("continuity replay accepted changed grant: %v", err)
	}
}

// RunIssue58FormattedGrantReplay keeps the producer's actual grant bytes fixed
// across admission, storage reload and replay, including JSON formatting/escaping.
func RunIssue58FormattedGrantReplay(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	Issue58Identity(t, b)
	source := Issue58Source("formatted-grant", true)
	source.Grant.Snapshot = json.RawMessage("{\n  \"decision\": \"allow\", \"reason\": \"<personal>&trusted\"\n}")
	original := string(source.Grant.Snapshot)
	if err := b.CreateSession(ctx, domain.Session{ID: source.SessionID}); err != nil {
		t.Fatal(err)
	}
	admission := Issue58Admission(source, "formatted-grant-run")
	store := b.(storage.RunAdmissionStore)
	first, err := store.CommitRunAdmission(ctx, admission)
	if err != nil {
		t.Fatal(err)
	}
	again, err := store.CommitRunAdmission(ctx, admission)
	if err != nil || again.Seq != first.Seq {
		t.Fatalf("unchanged formatted/HTML grant replay rejected: %v", err)
	}
	pin, err := b.(storage.SubjectSourceStore).GetRunSubjectSource(ctx, admission.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var decision struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if json.Unmarshal(pin.Grant.Snapshot, &decision) != nil || decision.Decision != "allow" || decision.Reason != "<personal>&trusted" {
		t.Fatalf("actual grant decision changed: %s", pin.Grant.Snapshot)
	}
	changed := source
	changed.Grant.Snapshot = json.RawMessage("{ \"decision\": \"deny\", \"reason\": \"<personal>&trusted\" }")
	admission.Subject = &changed
	if _, err := store.CommitRunAdmission(ctx, admission); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("changed grant decision accepted on replay: %v", err)
	}
	if string(source.Grant.Snapshot) != original {
		t.Fatal("producer grant snapshot was mutated")
	}
}

// Issue58Stuck builds a failed terminal run whose tool operation was claimed
// and never completed (cancelled mid-tool, crash, or unretainable result).
func Issue58Stuck(t *testing.T, b storage.Engine, s storage.SubjectSourceBinding, id string) storage.SubjectInteraction {
	t.Helper()
	ctx := context.Background()
	a := Issue58Admission(s, id)
	if _, err := b.(storage.RunAdmissionStore).CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	args := []byte(`{"text":"same arguments"}`)
	digest := func(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
	ops := b.(storage.ToolOperationStore)
	op := domain.ToolOperation{RunID: a.Run.ID, OperationID: "stuck-call", ToolName: "write_note", RequestDigest: digest("request:" + string(args)), MiddlewareInputArguments: args, ArgumentsDigest: digest(string(args)), EffectiveArguments: args}
	if _, _, _, err := ops.AdmitToolOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ops.ClaimToolOperation(ctx, a.Run.ID, "stuck-call", "owner"); err != nil {
		t.Fatal(err)
	}
	failed := domain.RunEvent{RunID: a.Run.ID, Type: domain.EventRunFailed, PayloadVersion: 1, CreatedAt: 1, Payload: []byte(`{"outcome":"failed"}`)}
	seq, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{failed}})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetRunStatus(ctx, a.Run.ID, domain.RunFailed); err != nil {
		t.Fatal(err)
	}
	return storage.SubjectInteraction{ID: storage.SubjectInteractionID(a.Run.ID, seq), SessionID: s.SessionID, RunID: a.Run.ID, TerminalSeq: seq}
}
