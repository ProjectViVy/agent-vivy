package conformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Workflow-step commit contract (S11-C, G4). AssertWorkflowStepContract
// exercises the storage-level guarantees one backend must keep for the INOFY
// RunStore adapter: atomic commits, commit-id idempotency, writer epochs,
// state/terminal invariants, contiguous journal events and restart/load.

func stepStore(t *testing.T, b storage.Engine) storage.WorkflowStepStore {
	t.Helper()
	s, ok := b.(storage.WorkflowStepStore)
	if !ok {
		t.Fatal("backend does not implement WorkflowStepStore")
	}
	return s
}

// StepDigest builds a stable 64-hex digest for test tags.
func StepDigest(tag string) string {
	sum := sha256.Sum256([]byte("workflow-step:" + tag))
	return hex.EncodeToString(sum[:])
}

// digestHex hashes raw bytes for descriptor/authority digest fields, which
// the admission validator checks against the stored bytes.
func digestHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// workflowStepFixture admits one INOFY (schema_version=2) workflow revision
// under an active parent run and returns the admitted workflow run id.
// WorkflowStepFixture admits one schema_version=2 workflow revision for tests.
func WorkflowStepFixture(t *testing.T, slot Slot, tag string) (domain.RunID, storage.WorkflowStepStore) {
	t.Helper()
	ctx := context.Background()
	s := stepStore(t, slot.Engine)
	sessionID := domain.SessionID("sess-step-" + tag)
	parentID := domain.RunID("run-step-parent-" + tag)
	wfID := domain.RunID("workflow-step-" + tag)
	if err := slot.Engine.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := slot.Engine.CreateRun(ctx, domain.Run{
		ID: parentID, SessionID: sessionID, Status: domain.RunActive,
		Kind: domain.RunKindPrimary, CreatedAt: 2,
	}); err != nil {
		t.Fatalf("create parent run: %v", err)
	}
	started := domain.RunEvent{RunID: wfID, Type: domain.EventRunStarted, CreatedAt: 3,
		PayloadVersion: 1, Payload: []byte(`{"provider":"test","model":"m","mode":"normal","face":"web"}`)}
	descriptorJSON := []byte(`{"descriptor":1}`)
	authorityJSON := []byte(`{"authority":1}`)
	revision := domain.WorkflowRevision{
		RunID:            wfID,
		ParentRunID:      parentID,
		ParentSessionID:  sessionID,
		RootRunID:        parentID,
		OperationKey:     "op-" + tag,
		DescriptorDigest: digestHex(descriptorJSON),
		AuthorityDigest:  digestHex(authorityJSON),
		DescriptorJSON:   descriptorJSON,
		AuthorityJSON:    authorityJSON,
		SchemaVersion:    2,
		CreatedAt:        3,
		ProgramDigest:    StepDigest("program-" + tag),
		CatalogDigest:    StepDigest("catalog-" + tag),
		CompilerVersion:  "inofy@6acfcc6",
		EinoBuild:        "v0.9.13",
		InputDigest:      StepDigest("input-" + tag),
		InputJSON:        []byte(`{}`),
		EffectiveLimits:  []byte(`{"max_nodes":12,"max_attempts":1}`),
		HostBindingID:    StepDigest("binding-" + tag),
	}
	run := domain.Run{ID: wfID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: 3,
		Kind: domain.RunKindWorkflow, ParentID: parentID, RootID: parentID, Depth: 1}
	admitted, err := slot.Engine.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision, Run: run, Started: started,
	})
	if err != nil || !admitted.Created {
		t.Fatalf("workflow admission: %+v err=%v", admitted, err)
	}
	return wfID, s
}

// NewStepCommit builds a workflow step commit bound to the fixture tag.
func NewStepCommit(runID domain.RunID, commitID string, epoch uint64, expected, target storage.WorkflowStepStatus, tag string) storage.WorkflowStepCommit {
	return storage.WorkflowStepCommit{
		RunID:         runID,
		CommitID:      commitID,
		Digest:        StepDigest("commit-" + commitID),
		Epoch:         epoch,
		ProgramDigest: StepDigest("program-" + tag),
		HostBindingID: StepDigest("binding-" + tag),
		Expected:      expected,
		Target:        target,
	}
}

// StepEvent builds one domain-bound workflow step event.
func StepEvent(typ domain.EventType, payload string) storage.WorkflowStepEvent {
	return storage.WorkflowStepEvent{Type: typ, CreatedAt: 100, PayloadVersion: 1, Payload: []byte(payload)}
}

// StepAdmitEvent is the workflow.admitted event matching the fixture revision.
func StepAdmitEvent(tag string) storage.WorkflowStepEvent {
	return StepEvent(domain.EventWorkflowAdmitted, fmt.Sprintf(
		`{"input_digest":%q,"limits":{"max_nodes":12,"max_attempts":1}}`, StepDigest("input-"+tag)))
}

func startedEvent() storage.WorkflowStepEvent {
	return StepEvent(domain.EventWorkflowStarted, `{"revision_digest":"`+StepDigest("rev")+`","node_count":1}`)
}

func journalSeqs(t *testing.T, b storage.Engine, runID domain.RunID) []domain.EventSeq {
	t.Helper()
	it, err := b.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer func() { _ = it.Close() }()
	var seqs []domain.EventSeq
	for it.Next() {
		seqs = append(seqs, it.Value().Event.Seq)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	return seqs
}

func requireContiguous(t *testing.T, seqs []domain.EventSeq) {
	t.Helper()
	for i, s := range seqs {
		if s != domain.EventSeq(i+1) {
			t.Fatalf("journal seqs not contiguous: %v", seqs)
		}
	}
}

// AssertWorkflowStepContract runs every S11-C case against one slot.
func AssertWorkflowStepContract(t *testing.T, slot Slot) {
	t.Helper()
	t.Run("initial commit and contiguous receipts", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "lifecycle")
		ctx := context.Background()
		// Load before any commit: admitted identity from the revision.
		st, err := s.LoadWorkflowStep(ctx, wf)
		if err != nil {
			t.Fatalf("load fresh: %v", err)
		}
		if st.Projection != nil || st.Revision.ProgramDigest != StepDigest("program-lifecycle") ||
			st.Run.Status != domain.RunActive {
			t.Fatalf("fresh state = %+v", st)
		}

		c1 := NewStepCommit(wf, "c1", 1, "", storage.WorkflowStepAdmitted, "lifecycle")
		c1.Events = []storage.WorkflowStepEvent{StepAdmitEvent("lifecycle")}
		r1, err := s.CommitWorkflowStep(ctx, c1)
		if err != nil {
			t.Fatalf("commit admitted: %v", err)
		}
		if r1.FirstSequence != 2 || r1.LastSequence != 2 || r1.Revision != 1 {
			t.Fatalf("receipt1 = %+v", r1)
		}
		c2 := NewStepCommit(wf, "c2", 1, storage.WorkflowStepAdmitted, storage.WorkflowStepRunning, "lifecycle")
		c2.Events = []storage.WorkflowStepEvent{startedEvent()}
		r2, err := s.CommitWorkflowStep(ctx, c2)
		if err != nil {
			t.Fatalf("commit running: %v", err)
		}
		if r2.FirstSequence != 3 || r2.LastSequence != 3 || r2.Revision != 2 {
			t.Fatalf("receipt2 = %+v", r2)
		}
		st, err = s.LoadWorkflowStep(ctx, wf)
		if err != nil || st.Projection == nil {
			t.Fatalf("load running: %+v err=%v", st, err)
		}
		if st.Projection.Status != storage.WorkflowStepRunning || st.Projection.Epoch != 1 ||
			st.Projection.Revision != 2 {
			t.Fatalf("projection = %+v", st.Projection)
		}
		requireContiguous(t, journalSeqs(t, slot.Engine, wf))
	})

	t.Run("duplicate commit id returns same receipt", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "dup")
		c := NewStepCommit(wf, "dup-1", 1, "", storage.WorkflowStepAdmitted, "dup")
		c.Events = []storage.WorkflowStepEvent{StepAdmitEvent("dup")}
		r1, err := s.CommitWorkflowStep(context.Background(), c)
		if err != nil {
			t.Fatalf("first commit: %v", err)
		}
		r2, err := s.CommitWorkflowStep(context.Background(), c)
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		if r1 != r2 {
			t.Fatalf("receipts differ: %+v vs %+v", r1, r2)
		}
		if n := len(journalSeqs(t, slot.Engine, wf)); n != 2 {
			t.Fatalf("duplicate replay appended events, journal len = %d", n)
		}
	})

	t.Run("conflicting commit id rejected", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "conflict")
		c := NewStepCommit(wf, "same-id", 1, "", storage.WorkflowStepAdmitted, "conflict")
		c.Events = []storage.WorkflowStepEvent{StepAdmitEvent("conflict")}
		if _, err := s.CommitWorkflowStep(context.Background(), c); err != nil {
			t.Fatalf("first commit: %v", err)
		}
		c.Digest = StepDigest("different-body")
		_, err := s.CommitWorkflowStep(context.Background(), c)
		if !errors.Is(err, storage.ErrWorkflowStepIdempotency) {
			t.Fatalf("want ErrWorkflowStepIdempotency, got %v", err)
		}
	})

	t.Run("stale epoch rejected, higher epoch claims", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "epoch")
		ctx := context.Background()
		c1 := NewStepCommit(wf, "e1", 1, "", storage.WorkflowStepAdmitted, "epoch")
		c1.Events = []storage.WorkflowStepEvent{StepAdmitEvent("epoch")}
		if _, err := s.CommitWorkflowStep(ctx, c1); err != nil {
			t.Fatalf("admit: %v", err)
		}
		c2 := NewStepCommit(wf, "e2", 3, storage.WorkflowStepAdmitted, storage.WorkflowStepRunning, "epoch")
		c2.Events = []storage.WorkflowStepEvent{startedEvent()}
		if _, err := s.CommitWorkflowStep(ctx, c2); err != nil {
			t.Fatalf("higher epoch claim: %v", err)
		}
		stale := NewStepCommit(wf, "e3", 2, storage.WorkflowStepRunning, storage.WorkflowStepRunning, "epoch")
		if _, err := s.CommitWorkflowStep(ctx, stale); !errors.Is(err, storage.ErrStaleWriter) {
			t.Fatalf("want ErrStaleWriter, got %v", err)
		}
		st, err := s.LoadWorkflowStep(ctx, wf)
		if err != nil || st.Projection == nil || st.Projection.Epoch != 3 {
			t.Fatalf("stored epoch = %+v err=%v", st.Projection, err)
		}
	})

	t.Run("expected-state mismatch rejected", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "state")
		c := NewStepCommit(wf, "s1", 1, storage.WorkflowStepRunning, storage.WorkflowStepRunning, "state")
		if _, err := s.CommitWorkflowStep(context.Background(), c); !errors.Is(err, storage.ErrWorkflowStepState) {
			t.Fatalf("want ErrWorkflowStepState on missing projection, got %v", err)
		}
		c2 := NewStepCommit(wf, "s2", 1, "", storage.WorkflowStepAdmitted, "state")
		c2.Events = []storage.WorkflowStepEvent{StepAdmitEvent("state")}
		if _, err := s.CommitWorkflowStep(context.Background(), c2); err != nil {
			t.Fatalf("admit: %v", err)
		}
		c3 := NewStepCommit(wf, "s3", 1, storage.WorkflowStepRunning, storage.WorkflowStepWaiting, "state")
		if _, err := s.CommitWorkflowStep(context.Background(), c3); !errors.Is(err, storage.ErrWorkflowStepState) {
			t.Fatalf("want ErrWorkflowStepState on status mismatch, got %v", err)
		}
	})

	t.Run("terminal invariant and native run status", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "terminal")
		ctx := context.Background()
		for i, c := range []storage.WorkflowStepCommit{
			NewStepCommit(wf, "t1", 1, "", storage.WorkflowStepAdmitted, "terminal"),
			NewStepCommit(wf, "t2", 1, storage.WorkflowStepAdmitted, storage.WorkflowStepRunning, "terminal"),
			NewStepCommit(wf, "t3", 1, storage.WorkflowStepRunning, storage.WorkflowStepSucceeded, "terminal"),
		} {
			switch i {
			case 0:
				c.Events = []storage.WorkflowStepEvent{StepAdmitEvent("terminal")}
			case 1:
				c.Events = []storage.WorkflowStepEvent{startedEvent()}
			case 2:
				c.Events = []storage.WorkflowStepEvent{StepEvent(domain.EventRunCompleted, `{"summary":"done"}`)}
			}
			if _, err := s.CommitWorkflowStep(ctx, c); err != nil {
				t.Fatalf("commit %d: %v", i, err)
			}
		}
		run, err := slot.Engine.GetRun(ctx, wf)
		if err != nil || run.Status != domain.RunCompleted {
			t.Fatalf("native status = %+v err=%v", run, err)
		}
		// New commit after terminal projection is refused.
		post := NewStepCommit(wf, "t4", 1, storage.WorkflowStepSucceeded, storage.WorkflowStepRunning, "terminal")
		if _, err := s.CommitWorkflowStep(ctx, post); !errors.Is(err, storage.ErrWorkflowStepTerminal) &&
			!errors.Is(err, storage.ErrWorkflowStepState) {
			t.Fatalf("want terminal/state rejection, got %v", err)
		}
		// A second terminal-typed event must never append.
		dup := NewStepCommit(wf, "t5", 1, storage.WorkflowStepSucceeded, storage.WorkflowStepFailed, "terminal")
		dup.Events = []storage.WorkflowStepEvent{StepEvent(domain.EventRunFailed, `{"cause_category":"internal_error","message":"x"}`)}
		if _, err := s.CommitWorkflowStep(ctx, dup); err == nil {
			t.Fatal("second terminal event was appended")
		}
	})

	t.Run("atomic rollback leaves no partial commit", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "atomic")
		ctx := context.Background()
		c1 := NewStepCommit(wf, "a1", 1, "", storage.WorkflowStepAdmitted, "atomic")
		c1.Events = []storage.WorkflowStepEvent{StepAdmitEvent("atomic")}
		if _, err := s.CommitWorkflowStep(ctx, c1); err != nil {
			t.Fatalf("admit: %v", err)
		}
		// Two results on the same (path, attempt) inside one commit force a
		// mid-transaction constraint failure; nothing may persist.
		bad := NewStepCommit(wf, "a2", 1, storage.WorkflowStepAdmitted, storage.WorkflowStepRunning, "atomic")
		bad.Events = []storage.WorkflowStepEvent{startedEvent()}
		bad.Results = []storage.WorkflowStepResult{
			{Path: "n1", Attempt: 1, Digest: StepDigest("r1"), BlobID: "wf/a2/r/1", Bytes: 4},
			{Path: "n1", Attempt: 1, Digest: StepDigest("r2"), BlobID: "wf/a2/r/2", Bytes: 4},
		}
		if _, err := s.CommitWorkflowStep(ctx, bad); err == nil {
			t.Fatal("conflicting results commit succeeded")
		}
		st, err := s.LoadWorkflowStep(ctx, wf)
		if err != nil || st.Projection == nil {
			t.Fatalf("load after failed tx: %+v err=%v", st, err)
		}
		if st.Projection.Status != storage.WorkflowStepAdmitted || len(st.Results) != 0 {
			t.Fatalf("partial state leaked: %+v", st.Projection)
		}
		if n := len(journalSeqs(t, slot.Engine, wf)); n != 2 {
			t.Fatalf("failed tx appended events, journal len = %d", n)
		}
	})

	t.Run("identity mismatch rejected", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "identity")
		c := NewStepCommit(wf, "i1", 1, "", storage.WorkflowStepAdmitted, "identity")
		c.Events = []storage.WorkflowStepEvent{StepAdmitEvent("identity")}
		c.ProgramDigest = StepDigest("other-program")
		if _, err := s.CommitWorkflowStep(context.Background(), c); !errors.Is(err, storage.ErrWorkflowStepIdentity) {
			t.Fatalf("program mismatch: %v", err)
		}
		c2 := NewStepCommit(wf, "i2", 1, "", storage.WorkflowStepAdmitted, "identity")
		c2.Events = []storage.WorkflowStepEvent{StepAdmitEvent("identity")}
		c2.HostBindingID = StepDigest("other-binding")
		if _, err := s.CommitWorkflowStep(context.Background(), c2); !errors.Is(err, storage.ErrWorkflowStepIdentity) {
			t.Fatalf("binding mismatch: %v", err)
		}
	})

	t.Run("waiting commit persists checkpoint waits and unresolved", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "waits")
		ctx := context.Background()
		c1 := NewStepCommit(wf, "w1", 1, "", storage.WorkflowStepAdmitted, "waits")
		c1.Events = []storage.WorkflowStepEvent{StepAdmitEvent("waits")}
		if _, err := s.CommitWorkflowStep(ctx, c1); err != nil {
			t.Fatalf("admit: %v", err)
		}
		c2 := NewStepCommit(wf, "w2", 1, storage.WorkflowStepAdmitted, storage.WorkflowStepRunning, "waits")
		c2.Events = []storage.WorkflowStepEvent{startedEvent()}
		if _, err := s.CommitWorkflowStep(ctx, c2); err != nil {
			t.Fatalf("start: %v", err)
		}
		c3 := NewStepCommit(wf, "w3", 1, storage.WorkflowStepRunning, storage.WorkflowStepWaiting, "waits")
		c3.Events = []storage.WorkflowStepEvent{
			StepEvent(domain.EventWorkflowNodeAttempt, `{"node_key":"n1","attempt":1}`),
			StepEvent(domain.EventWorkflowWaiting, `{"waits":[{"request_id":"wq1","kind":"question","continuation_ref":"k"}],"interrupts":{"wq1":"intr-1"},"gates":["g1"]}`),
		}
		c3.Checkpoint = &storage.WorkflowStepCheckpoint{
			Envelope: []byte(`{"continuation_generation":1}`), BlobID: "wf/waits/c/1", Digest: StepDigest("cp1"),
		}
		if _, err := s.CommitWorkflowStep(ctx, c3); err != nil {
			t.Fatalf("waiting commit: %v", err)
		}
		st, err := s.LoadWorkflowStep(ctx, wf)
		if err != nil || st.Projection == nil {
			t.Fatalf("load waiting: %+v err=%v", st, err)
		}
		p := st.Projection
		if p.Status != storage.WorkflowStepWaiting || p.CheckpointBlobID != "wf/waits/c/1" {
			t.Fatalf("waiting projection = %+v", p)
		}
		var waits []struct {
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(p.WaitsJSON, &waits); err != nil || len(waits) != 1 || waits[0].RequestID != "wq1" {
			t.Fatalf("waits = %s err=%v", p.WaitsJSON, err)
		}
		var unresolved []struct {
			Path    string `json:"path"`
			Attempt int    `json:"attempt"`
		}
		if err := json.Unmarshal(p.UnresolvedJSON, &unresolved); err != nil || len(unresolved) != 1 || unresolved[0].Path != "n1" {
			t.Fatalf("unresolved = %s err=%v", p.UnresolvedJSON, err)
		}
		// Resume claim: epoch +1, waits cleared, resume key stored.
		c4 := NewStepCommit(wf, "w4", 2, storage.WorkflowStepWaiting, storage.WorkflowStepRunning, "waits")
		c4.Events = []storage.WorkflowStepEvent{StepEvent(domain.EventWorkflowResumed,
			`{"idempotency_key":"rk1","answers_digest":"`+StepDigest("ans")+`","resumed_waits":["wq1"]}`)}
		if _, err := s.CommitWorkflowStep(ctx, c4); err != nil {
			t.Fatalf("resume claim: %v", err)
		}
		st, err = s.LoadWorkflowStep(ctx, wf)
		if err != nil || st.Projection == nil {
			t.Fatalf("load resumed: %+v err=%v", st, err)
		}
		if st.Projection.Epoch != 2 || st.Projection.Status != storage.WorkflowStepRunning ||
			st.Projection.ResumeKey != "rk1" {
			t.Fatalf("resumed projection = %+v", st.Projection)
		}
		if len(st.Projection.WaitsJSON) != 0 && string(st.Projection.WaitsJSON) != "null" && !bytes.Equal(st.Projection.WaitsJSON, []byte("[]")) {
			t.Fatalf("waits not cleared: %s", st.Projection.WaitsJSON)
		}
		// The superseded epoch is now stale.
		old := NewStepCommit(wf, "w5", 1, storage.WorkflowStepRunning, storage.WorkflowStepRunning, "waits")
		if _, err := s.CommitWorkflowStep(ctx, old); !errors.Is(err, storage.ErrStaleWriter) {
			t.Fatalf("superseded epoch: %v", err)
		}
	})

	t.Run("terminal events require terminal target", func(t *testing.T) {
		wf, s := WorkflowStepFixture(t, slot, "termrule")
		ctx := context.Background()
		c1 := NewStepCommit(wf, "tr1", 1, "", storage.WorkflowStepAdmitted, "termrule")
		c1.Events = []storage.WorkflowStepEvent{
			StepAdmitEvent("termrule"),
			StepEvent(domain.EventRunCompleted, `{"summary":"early"}`),
		}
		if _, err := s.CommitWorkflowStep(ctx, c1); err == nil {
			t.Fatal("terminal event inside non-terminal commit was accepted")
		}
		// A terminal-target commit without its terminal event is also refused.
		c2 := NewStepCommit(wf, "tr2", 1, "", storage.WorkflowStepAdmitted, "termrule")
		c2.Events = []storage.WorkflowStepEvent{StepAdmitEvent("termrule")}
		if _, err := s.CommitWorkflowStep(ctx, c2); err != nil {
			t.Fatalf("admit: %v", err)
		}
		c3 := NewStepCommit(wf, "tr3", 1, storage.WorkflowStepAdmitted, storage.WorkflowStepSucceeded, "termrule")
		c3.Events = []storage.WorkflowStepEvent{StepEvent(domain.EventWorkflowNodeCompleted, `{"node_key":"n1","child_run_id":"c","result_digest":"`+StepDigest("r")+`"}`)}
		if _, err := s.CommitWorkflowStep(ctx, c3); err == nil {
			t.Fatal("terminal target without terminal event was accepted")
		}
	})

	t.Run("legacy schema_version 1 row is not an INOFY execution", func(t *testing.T) {
		ctx := context.Background()
		s := stepStore(t, slot.Engine)
		sessionID := domain.SessionID("sess-legacy-step")
		parentID := domain.RunID("run-legacy-parent")
		wfID := domain.RunID("workflow-legacy")
		if err := slot.Engine.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
		if err := slot.Engine.CreateRun(ctx, domain.Run{
			ID: parentID, SessionID: sessionID, Status: domain.RunActive,
			Kind: domain.RunKindPrimary, CreatedAt: 2,
		}); err != nil {
			t.Fatal(err)
		}
		started := domain.RunEvent{RunID: wfID, Type: domain.EventRunStarted, CreatedAt: 3, PayloadVersion: 1, Payload: []byte(`{}`)}
		legacyDescriptor, legacyAuthority := []byte(`{"legacy":true}`), []byte(`{"legacy":true}`)
		revision := domain.WorkflowRevision{
			RunID: wfID, ParentRunID: parentID, ParentSessionID: sessionID, RootRunID: parentID,
			OperationKey: "legacy-op", DescriptorDigest: digestHex(legacyDescriptor), AuthorityDigest: digestHex(legacyAuthority),
			DescriptorJSON: legacyDescriptor, AuthorityJSON: legacyAuthority,
			SchemaVersion: 1, CreatedAt: 3,
		}
		run := domain.Run{ID: wfID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: 3,
			Kind: domain.RunKindWorkflow, ParentID: parentID, RootID: parentID, Depth: 1}
		if _, err := slot.Engine.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
			Revision: revision, Run: run, Started: started,
		}); err != nil {
			t.Fatalf("legacy admission: %v", err)
		}
		if _, err := s.LoadWorkflowStep(ctx, wfID); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("legacy load: %v", err)
		}
		c := NewStepCommit(wfID, "l1", 1, "", storage.WorkflowStepAdmitted, "legacy")
		if _, err := s.CommitWorkflowStep(ctx, c); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("legacy commit: %v", err)
		}
	})

	t.Run("incomplete inofy identity rejected at admission", func(t *testing.T) {
		ctx := context.Background()
		sessionID := domain.SessionID("sess-incomplete")
		parentID := domain.RunID("run-incomplete-parent")
		wfID := domain.RunID("workflow-incomplete")
		if err := slot.Engine.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
		if err := slot.Engine.CreateRun(ctx, domain.Run{
			ID: parentID, SessionID: sessionID, Status: domain.RunActive,
			Kind: domain.RunKindPrimary, CreatedAt: 2,
		}); err != nil {
			t.Fatal(err)
		}
		started := domain.RunEvent{RunID: wfID, Type: domain.EventRunStarted, CreatedAt: 3, PayloadVersion: 1, Payload: []byte(`{}`)}
		revision := domain.WorkflowRevision{
			RunID: wfID, ParentRunID: parentID, ParentSessionID: sessionID, RootRunID: parentID,
			OperationKey: "incomplete-op", DescriptorDigest: digestHex([]byte(`{}`)), AuthorityDigest: digestHex([]byte(`{}`)),
			DescriptorJSON: []byte(`{}`), AuthorityJSON: []byte(`{}`), SchemaVersion: 2, CreatedAt: 3,
			// ProgramDigest, CatalogDigest, InputDigest, EffectiveLimits and
			// HostBindingID intentionally absent.
		}
		run := domain.Run{ID: wfID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: 3,
			Kind: domain.RunKindWorkflow, ParentID: parentID, RootID: parentID, Depth: 1}
		if _, err := slot.Engine.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
			Revision: revision, Run: run, Started: started,
		}); err == nil {
			t.Fatal("schema_version=2 admission without identity fields succeeded")
		}
	})

	t.Run("restart load preserves projection", func(t *testing.T) {
		if slot.Reopen == nil {
			t.Skip("slot does not support reopen")
		}
		wf, s := WorkflowStepFixture(t, slot, "restart")
		ctx := context.Background()
		c1 := NewStepCommit(wf, "re1", 1, "", storage.WorkflowStepAdmitted, "restart")
		c1.Events = []storage.WorkflowStepEvent{StepAdmitEvent("restart")}
		if _, err := s.CommitWorkflowStep(ctx, c1); err != nil {
			t.Fatalf("admit: %v", err)
		}
		reopened, err := slot.Reopen()
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		s2 := stepStore(t, reopened)
		st, err := s2.LoadWorkflowStep(ctx, wf)
		if err != nil || st.Projection == nil || st.Projection.Status != storage.WorkflowStepAdmitted {
			t.Fatalf("reopened load = %+v err=%v", st, err)
		}
		if st.Revision.ProgramDigest != StepDigest("program-restart") {
			t.Fatalf("reopened revision identity lost: %+v", st.Revision)
		}
	})
}
