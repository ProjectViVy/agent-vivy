// Package conformance is the D-032 backend suite (CN-01..CN-33).
package conformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// DualOpenMode is how CN-14 interprets a second Open on the same Journal.
type DualOpenMode int

const (
	// DualOpenShared is the SQLite file model: two handles may write; the
	// journal stays contiguous.
	DualOpenShared DualOpenMode = iota
	// DualOpenExclusive is the server model: the second Open must fail with
	// storage.ErrLeaseHeld.
	DualOpenExclusive
)

// Slot is one isolated database plus reopen/second-open hooks.
type Slot struct {
	Engine     storage.Engine
	Reopen     func() (storage.Engine, error)
	OpenSecond func() (storage.Engine, error)
}

// Harness binds the suite to one backend.
type Harness struct {
	DualOpen DualOpenMode
	Setup    func(t *testing.T) Slot
}

// Run executes the backend conformance suite.
func Run(t *testing.T, h Harness) {
	t.Helper()
	cases := []struct {
		id   string
		name string
		run  func(*testing.T, Harness)
	}{
		{"CN-01", "atomic append", cnAtomicAppend},
		{"CN-02", "monotonic sequence", cnMonotonicSequence},
		{"CN-03", "expected-version conflict", cnExpectedVersionConflict},
		{"CN-04", "idempotent replay", cnIdempotentReplay},
		{"CN-05", "retry divergence refused", cnRetryDivergenceRefused},
		{"CN-06", "exactly one terminal", cnExactlyOneTerminal},
		{"CN-07", "first-writer-wins approval", cnFirstWriterWinsApproval},
		{"CN-08", "restart repair view", cnRestartRepairView},
		{"CN-09", "torn final write survives reopen", cnTornFinalWrite},
		{"CN-10", "malformed payload round-trip", cnMalformedPayload},
		{"CN-11", "orphan checkpoint generations", cnOrphanCheckpoint},
		{"CN-12", "approval decision after kill", cnApprovalAfterKill},
		{"CN-13", "payload byte fidelity (secret audit anchor)", cnPayloadByteFidelity},
		{"CN-14", "dual-handle process lock safety", cnDualHandleSafety},
		{"CN-15", "monotonic replay under concurrent writers", cnConcurrentWriters},
		{"CN-16", "replay after disconnect (after_seq tail)", cnReplayAfterDisconnect},
		{"CN-17", "message provenance round-trip", cnMessageProvenance},
		{"CN-18", "file version chain + stale-read tracker", cnFileVersionChain},
		{"CN-19", "runs listed by session", cnRunsBySession},
		{"CN-20", "compactions listed by session", cnCompactionsBySession},
		{"CN-21", "session truncation markers", cnSessionTruncationMarkers},
		{"CN-22", "message projection idempotence and conflicts", cnMessageProjectionIdempotence},
		{"CN-23", "concurrent duplicate message projection", cnConcurrentMessageProjection},
		{"CN-24", "durable session activity timestamp", cnSessionActivity},
		{"CN-25", "bounded modified-file sidebar projection", cnModifiedFiles},
		{"CN-26", "attributed model usage projection", cnAttributedModelUsage},
		{"CN-27", "durable immutable session workspace", cnSessionWorkspace},
		{"CN-28", "tool operation identity and atomic claim", cnToolOperationIdentity},
		{"CN-29", "tool operation recovery after reopen", cnToolOperationRecovery},
		{"CN-30", "continuable child admission and reauthorization", cnChildSessionAdmission},
		{"CN-31", "ordered durable child mailbox and receipt", cnChildMailbox},
		{"CN-32", "parent deletion fences child session tree", cnChildSessionDelete},
		{"CN-33", "immutable workflow revision admission", cnWorkflowRevisionAdmission},
	}
	if len(cases) != 33 {
		t.Fatalf("conformance suite must carry exactly 33 cases, got %d", len(cases))
	}
	for _, c := range cases {
		t.Run(c.id+" "+c.name, func(t *testing.T) { c.run(t, h) })
	}
}

func cnWorkflowRevisionAdmission(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	const sessionID = domain.SessionID("session-cn-workflow-parent")
	const parentID = domain.RunID("run-cn-workflow-parent")
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, Title: "workflow parent", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: parentID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2, RootID: parentID}); err != nil {
		t.Fatal(err)
	}

	const descriptor = `{"schema_version":1,"nodes":[{"key":"a","task":"inspect"}],"outputs":["a"]}`
	makeAdmission := func(runID domain.RunID) storage.WorkflowAdmission {
		descriptorHash := sha256.Sum256([]byte(descriptor))
		authority := []byte(`{"policy_profile":"default","tool_names":[]}`)
		authorityHash := sha256.Sum256(authority)
		created := int64(3)
		return storage.WorkflowAdmission{
			Revision: domain.WorkflowRevision{
				RunID: runID, ParentRunID: parentID, ParentSessionID: sessionID, RootRunID: parentID,
				OperationKey: "operation-1", DescriptorDigest: hex.EncodeToString(descriptorHash[:]),
				AuthorityDigest: hex.EncodeToString(authorityHash[:]), DescriptorJSON: []byte(descriptor), AuthorityJSON: authority, SchemaVersion: 1, CreatedAt: created,
			},
			Run:     domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: created, Kind: domain.RunKindWorkflow, ParentID: parentID, RootID: parentID, Depth: 1},
			Started: domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, CreatedAt: created, PayloadVersion: 1, Payload: []byte(`{"mode":"workflow"}`)},
		}
	}

	const workers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	results := make([]storage.WorkflowAdmissionResult, workers)
	var firstErr error
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input := makeAdmission(domain.RunID(fmt.Sprintf("run-cn-workflow-%d", i)))
			result, err := b.CommitWorkflowAdmission(ctx, input)
			mu.Lock()
			defer mu.Unlock()
			results[i] = result
			if result.Created {
				created++
			}
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}(i)
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("concurrent admission: %v", firstErr)
	}
	if created != 1 {
		t.Fatalf("created %d workflow revisions, want exactly one", created)
	}
	wantRunID := results[0].Run.ID
	for i, result := range results {
		if result.Run.ID != wantRunID || result.Revision.RunID != wantRunID || result.Started.Seq != 1 {
			t.Fatalf("result %d did not resolve to the same durable Run: %+v", i, result)
		}
	}
	stored, err := b.GetWorkflowRevisionByOperation(ctx, parentID, "operation-1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.RunID != wantRunID || stored.DescriptorDigest != results[0].Revision.DescriptorDigest {
		t.Fatalf("stored revision mismatch: %+v", stored)
	}

	conflict := makeAdmission("run-cn-workflow-conflict")
	conflict.Revision.DescriptorJSON = []byte(`{"schema_version":1,"nodes":[{"key":"a","task":"changed"}],"outputs":["a"]}`)
	conflictHash := sha256.Sum256(conflict.Revision.DescriptorJSON)
	conflict.Revision.DescriptorDigest = hex.EncodeToString(conflictHash[:])
	if _, err := b.CommitWorkflowAdmission(ctx, conflict); !errors.Is(err, storage.ErrWorkflowRevisionConflict) {
		t.Fatalf("changed retry = %v, want workflow conflict", err)
	}
	for i, kind := range []domain.RunKind{domain.RunKindChild, domain.RunKindWorkflow, domain.RunKindChild} {
		childMode := domain.ChildModeOneShot
		if kind == domain.RunKindWorkflow {
			childMode = ""
		}
		if err := b.CreateRun(ctx, domain.Run{
			ID: domain.RunID(fmt.Sprintf("run-cn-workflow-cap-%d", i)), SessionID: sessionID,
			Status: domain.RunActive, CreatedAt: int64(20 + i), Kind: kind, ChildMode: childMode,
			ParentID: parentID, RootID: parentID, Depth: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	capAdmission := makeAdmission("run-cn-workflow-cap-admission")
	capAdmission.Revision.OperationKey = "operation-cap"
	if _, err := b.CommitWorkflowAdmission(ctx, capAdmission); !errors.Is(err, storage.ErrChildConcurrencyLimit) {
		t.Fatalf("workflow admission above direct-child cap = %v, want concurrency limit", err)
	}

	if err := b.DeleteSession(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetWorkflowRevision(ctx, wantRunID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted parent revision lookup = %v, want not found", err)
	}
}

func cnChildSessionAdmission(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	const parentSessionID = domain.SessionID("session-cn-child-parent")
	const parentRunID = domain.RunID("run-cn-child-parent")
	if err := b.CreateSession(ctx, domain.Session{ID: parentSessionID, Title: "parent", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: parentRunID, SessionID: parentSessionID, Status: domain.RunActive, CreatedAt: 2, RootID: parentRunID}); err != nil {
		t.Fatal(err)
	}
	input := conformanceChildAdmission(parentSessionID, parentRunID, "child-first", "operation-first", "digest-first", 3, 4)
	const workers = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	var firstErr error
	results := make([]storage.ChildSessionAdmissionResult, workers)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := b.CommitChildSessionAdmission(ctx, input)
			mu.Lock()
			defer mu.Unlock()
			results[i] = result
			if result.Created {
				created++
			}
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}(i)
	}
	wg.Wait()
	if firstErr != nil || created != 1 {
		t.Fatalf("concurrent child admission created=%d err=%v, want one committed child", created, firstErr)
	}
	for _, result := range results {
		if result.Binding.ChildSessionID != "child-first" || result.Run.ID != "run-child-first" || result.Started.Seq != 1 {
			t.Fatalf("duplicate admission returned inconsistent identity: %+v", result)
		}
	}
	if runs, err := b.ListRunsBySession(ctx, "child-first"); err != nil || len(runs) != 1 {
		t.Fatalf("child activation rows=%d err=%v, want one", len(runs), err)
	}
	listedSessions, err := b.ListSessions(ctx)
	if err != nil || len(listedSessions) != 1 || listedSessions[0].ID != parentSessionID {
		t.Fatalf("top-level session list = %+v err=%v, want parent only", listedSessions, err)
	}
	conflict := conformanceChildAdmission(parentSessionID, parentRunID, "child-conflict", "operation-first", "different-digest", 5, 6)
	if _, err := b.CommitChildSessionAdmission(ctx, conflict); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("reused operation key with changed request = %v, want conflict", err)
	}

	if err := b.SetRunStatus(ctx, parentRunID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	const authorizerID = domain.RunID("run-cn-child-reauthorizer")
	if err := b.CreateRun(ctx, domain.Run{ID: authorizerID, SessionID: parentSessionID, Status: domain.RunActive, CreatedAt: 7, RootID: parentRunID}); err != nil {
		t.Fatal(err)
	}
	activation := conformanceChildActivation("child-first", authorizerID, parentRunID, 8, 9)
	if _, err := b.CommitChildSessionActivation(ctx, activation); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("follow-up activation while previous child run is active = %v, want conflict", err)
	}
	if err := b.SetRunStatus(ctx, "run-child-first", domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	result, err := b.CommitChildSessionActivation(ctx, activation)
	if err != nil || !result.Created {
		t.Fatalf("continuation after origin run termination: result=%+v err=%v", result, err)
	}
	if result.Binding.OriginParentRunID != parentRunID || result.Binding.AuthorizerRunID != authorizerID || result.Binding.InitialActivationRunID != "run-child-first" || result.Binding.ActivationRunID != "run-child-followup" {
		t.Fatalf("child activation lineage = %+v", result.Binding)
	}
	if result.Binding.AuthorityCeiling.PolicyProfile != domain.PolicyProfileDefault || result.Binding.AuthorityCeiling.SandboxMode != domain.SandboxModeWorkspaceWrite ||
		len(result.Binding.AuthorityCeiling.ToolNames) != 1 || result.Binding.AuthorityCeiling.ToolNames[0] != "read_file" ||
		result.Binding.ActivationOperationKey != "operation-followup" || len(result.Binding.ActivationToolNames) != 1 || result.Binding.ActivationToolNames[0] != "read_file" {
		t.Fatalf("persisted child authority or activation scope = %+v", result.Binding)
	}
	retried, err := b.CommitChildSessionActivation(ctx, activation)
	if err != nil || retried.Created || retried.Run.ID != "run-child-followup" {
		t.Fatalf("idempotent child activation = %+v err=%v", retried, err)
	}
	changedActivation := activation
	changedActivation.Admission.Message = activation.Admission.Message
	changedActivation.Admission.Message.ID = "message-child-followup-changed"
	changedActivation.Admission.Message.Content = "different continuation"
	changedActivation.Admission.Message.RunID = "run-child-followup-changed"
	changedActivation.Admission.Run = activation.Admission.Run
	changedActivation.Admission.Run.ID = "run-child-followup-changed"
	changedActivation.Admission.Run.CreatedAt++
	changedActivation.Admission.Started = activation.Admission.Started
	changedActivation.Admission.Started.RunID = changedActivation.Admission.Run.ID
	changedActivation.Admission.Started.CreatedAt++
	changedActivation.RequestDigest, err = domain.ChildRequestDigest(changedActivation.OperationKey, changedActivation.Admission.Message.Content, changedActivation.ToolNames)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.CommitChildSessionActivation(ctx, changedActivation); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("reused activation key with changed task = %v, want conflict", err)
	}
	widenedActivation := activation
	widenedActivation.OperationKey = "operation-widened"
	widenedActivation.ToolNames = []string{"shell"}
	widenedActivation.Admission.Message.ID = "message-child-widened"
	widenedActivation.Admission.Message.Content = "widen tools"
	widenedActivation.Admission.Message.RunID = "run-child-widened"
	widenedActivation.Admission.Run.ID = "run-child-widened"
	widenedActivation.Admission.Run.CreatedAt++
	widenedActivation.Admission.Started.RunID = widenedActivation.Admission.Run.ID
	widenedActivation.Admission.Started.CreatedAt++
	widenedActivation.RequestDigest, err = domain.ChildRequestDigest(widenedActivation.OperationKey, widenedActivation.Admission.Message.Content, widenedActivation.ToolNames)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.CommitChildSessionActivation(ctx, widenedActivation); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("activation outside original tool ceiling = %v, want conflict", err)
	}
	if err := b.SetRunStatus(ctx, "run-child-followup", domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	for i, kind := range []domain.RunKind{domain.RunKindChild, domain.RunKindWorkflow, domain.RunKindChild, domain.RunKindChild} {
		childMode := domain.ChildModeOneShot
		if kind == domain.RunKindWorkflow {
			childMode = ""
		}
		if err := b.CreateRun(ctx, domain.Run{
			ID: domain.RunID(fmt.Sprintf("run-cn-child-cap-%d", i)), SessionID: parentSessionID,
			Status: domain.RunActive, CreatedAt: int64(20 + i), Kind: kind, ChildMode: childMode,
			ParentID: authorizerID, RootID: parentRunID, Depth: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	capActivation := conformanceChildActivation("child-first", authorizerID, parentRunID, 30, 31)
	capActivation.OperationKey = "operation-cap"
	capActivation.RequestDigest, err = domain.ChildRequestDigest(capActivation.OperationKey, capActivation.Admission.Message.Content, capActivation.ToolNames)
	if err != nil {
		t.Fatal(err)
	}
	capActivation.Admission.Run.ID = "run-child-followup-cap"
	capActivation.Admission.Message.ID = "message-child-followup-cap"
	capActivation.Admission.Message.RunID = capActivation.Admission.Run.ID
	capActivation.Admission.Started.RunID = capActivation.Admission.Run.ID
	if _, err := b.CommitChildSessionActivation(ctx, capActivation); !errors.Is(err, storage.ErrChildConcurrencyLimit) {
		t.Fatalf("follow-up admission above direct-child cap = %v, want concurrency limit", err)
	}
	capChild := conformanceChildAdmission(parentSessionID, authorizerID, "child-cap", "operation-newchild-cap", "digest-cap", 32, 33)
	capChild.Admission.Run.RootID = parentRunID
	if _, err := b.CommitChildSessionAdmission(ctx, capChild); !errors.Is(err, storage.ErrChildConcurrencyLimit) {
		t.Fatalf("continuable child admission above direct-child cap = %v, want concurrency limit", err)
	}
	oneShotID := domain.RunID("run-child-oneshot-cap")
	oneShotAdmission := storage.RunAdmission{
		Message: domain.Message{ID: "message-child-oneshot-cap", SessionID: parentSessionID, RunID: oneShotID, Role: domain.RoleUser, CreatedAt: 34, Content: "one shot"},
		Run:     domain.Run{ID: oneShotID, SessionID: parentSessionID, Status: domain.RunAccepted, CreatedAt: 35, Kind: domain.RunKindChild, ChildMode: domain.ChildModeOneShot, ParentID: authorizerID, RootID: parentRunID, Depth: 1},
		Started: domain.RunEvent{RunID: oneShotID, Type: domain.EventRunStarted, CreatedAt: 35, PayloadVersion: 1, Payload: []byte(`{"provider":"fixture"}`)},
	}
	runAdmissions, ok := b.(storage.RunAdmissionStore)
	if !ok {
		t.Fatal("backend does not expose RunAdmissionStore")
	}
	if _, err := runAdmissions.CommitRunAdmission(ctx, oneShotAdmission); !errors.Is(err, storage.ErrChildConcurrencyLimit) {
		t.Fatalf("one-shot admission above direct-child cap = %v, want concurrency limit", err)
	}
	legacy := domain.Run{ID: "run-legacy-child", SessionID: parentSessionID, Status: domain.RunActive, CreatedAt: 10, Kind: domain.RunKindChild, ParentID: authorizerID, RootID: parentRunID, Depth: 1}
	if err := b.CreateRun(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	storedLegacy, err := b.GetRun(ctx, legacy.ID)
	if err != nil || storedLegacy.EffectiveChildMode() != domain.ChildModeOneShot {
		t.Fatalf("legacy child mode = %+v err=%v, want one-shot", storedLegacy, err)
	}
}

func cnChildMailbox(t *testing.T, h Harness) {
	slot := h.Setup(t)
	b := slot.Engine
	initialBackend := b
	t.Cleanup(func() { _ = initialBackend.Close() })
	ctx := context.Background()
	parentSessionID := domain.SessionID("session-cn-mail-parent")
	parentRunID := domain.RunID("run-cn-mail-parent")
	if err := b.CreateSession(ctx, domain.Session{ID: parentSessionID, Title: "parent", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: parentRunID, SessionID: parentSessionID, Status: domain.RunActive, CreatedAt: 2, RootID: parentRunID}); err != nil {
		t.Fatal(err)
	}
	created, err := b.CommitChildSessionAdmission(ctx, conformanceChildAdmission(parentSessionID, parentRunID, "child-mail", "operation-mail", "digest-mail", 3, 4))
	if err != nil {
		t.Fatal(err)
	}
	first := domain.ChildMailboxMessage{ID: "mail-1", ChildSessionID: "child-mail", SenderSessionID: parentSessionID, RecipientSessionID: "child-mail", IdempotencyKey: "parent-msg-1", Body: []byte("first"), CreatedAt: 5}
	first, inserted, err := b.EnqueueChildMessage(ctx, first)
	if err != nil || !inserted || first.Sequence != 1 {
		t.Fatalf("first mail=%+v inserted=%v err=%v", first, inserted, err)
	}
	retry := first
	retry.ID = "client-generated-different-id"
	retry.Sequence = 0
	retry.Status = ""
	retry.ConsumedAt = 0
	retry.ConsumedByRunID = ""
	retry, inserted, err = b.EnqueueChildMessage(ctx, retry)
	if err != nil || inserted || retry.ID != first.ID || retry.Sequence != first.Sequence {
		t.Fatalf("idempotent mail retry=%+v inserted=%v err=%v", retry, inserted, err)
	}
	conflict := domain.ChildMailboxMessage{ID: "mail-conflict", ChildSessionID: "child-mail", SenderSessionID: parentSessionID, RecipientSessionID: "child-mail", IdempotencyKey: "parent-msg-1", Body: []byte("changed")}
	if _, _, err := b.EnqueueChildMessage(ctx, conflict); !errors.Is(err, storage.ErrChildMessageConflict) {
		t.Fatalf("mail idempotency conflict = %v, want conflict", err)
	}
	second, inserted, err := b.EnqueueChildMessage(ctx, domain.ChildMailboxMessage{ID: "mail-2", ChildSessionID: "child-mail", SenderSessionID: parentSessionID, RecipientSessionID: "child-mail", IdempotencyKey: "parent-msg-2", Body: []byte("second"), CreatedAt: 6})
	if err != nil || !inserted || second.Sequence != 2 {
		t.Fatalf("second mail=%+v inserted=%v err=%v", second, inserted, err)
	}
	if _, _, err := b.RecordChildMessageReceipt(ctx, domain.ChildMessageReceipt{ChildSessionID: "child-mail", MessageID: second.ID, ConsumerRunID: created.Run.ID, State: domain.ChildMessageReceiptConsumed, CreatedAt: 7, UpdatedAt: 7}); !errors.Is(err, storage.ErrChildMessageConflict) {
		t.Fatalf("out-of-order consumption = %v, want conflict", err)
	}
	receipt := domain.ChildMessageReceipt{ChildSessionID: "child-mail", MessageID: first.ID, ConsumerRunID: created.Run.ID, State: domain.ChildMessageReceiptInProgress, CreatedAt: 8, UpdatedAt: 8}
	if _, inserted, err := b.RecordChildMessageReceipt(ctx, receipt); err != nil || !inserted {
		t.Fatalf("begin receipt inserted=%v err=%v", inserted, err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen mailbox: %v", err)
	}
	b = reopened
	t.Cleanup(func() { _ = reopened.Close() })
	storedReceipt, err := b.GetChildMessageReceipt(ctx, "child-mail", first.ID, created.Run.ID)
	if err != nil || storedReceipt.State != domain.ChildMessageReceiptInProgress {
		t.Fatalf("receipt after reopen=%+v err=%v", storedReceipt, err)
	}
	pending, err := b.ListPendingChildMessages(ctx, "child-mail", "child-mail", 0, 10)
	if err != nil || len(pending) != 2 || pending[0].Sequence != 1 || pending[1].Sequence != 2 {
		t.Fatalf("pending after reopen=%+v err=%v", pending, err)
	}
	receipt.State = domain.ChildMessageReceiptConsumed
	receipt.UpdatedAt = 9
	if _, _, err := b.RecordChildMessageReceipt(ctx, receipt); err != nil {
		t.Fatalf("consume first receipt: %v", err)
	}
	binding, err := b.GetChildSessionBinding(ctx, "child-mail")
	if err != nil || binding.ConsumedMessageSequence != 1 {
		t.Fatalf("consumed cursor=%d err=%v", binding.ConsumedMessageSequence, err)
	}
	childReply, inserted, err := b.EnqueueChildMessage(ctx, domain.ChildMailboxMessage{ID: "mail-reply", ChildSessionID: "child-mail", SenderSessionID: "child-mail", RecipientSessionID: parentSessionID, IdempotencyKey: "child-reply-1", Body: []byte("reply"), CreatedAt: 9})
	if err != nil || !inserted || childReply.Sequence != 1 {
		t.Fatalf("child reply=%+v inserted=%v err=%v; want parent inbox sequence 1", childReply, inserted, err)
	}
	parentInbox, err := b.ListPendingChildMessages(ctx, "child-mail", parentSessionID, 0, 10)
	if err != nil || len(parentInbox) != 1 || parentInbox[0].ID != childReply.ID {
		t.Fatalf("parent inbox=%+v err=%v; want only child reply", parentInbox, err)
	}
	if _, err := b.ListPendingChildMessages(ctx, "child-mail", "session-cn-unrelated", 0, 10); !errors.Is(err, storage.ErrChildMessageConflict) {
		t.Fatalf("unrelated session mailbox read = %v, want conflict", err)
	}
	childInbox, err := b.ListPendingChildMessages(ctx, "child-mail", "child-mail", 0, 10)
	if err != nil || len(childInbox) != 1 || childInbox[0].ID != second.ID {
		t.Fatalf("child inbox=%+v err=%v; want second parent message", childInbox, err)
	}
	wrongRecipient := domain.ChildMailboxMessage{ID: "mail-wrong-route", ChildSessionID: "child-mail", SenderSessionID: parentSessionID, RecipientSessionID: parentSessionID, IdempotencyKey: "wrong-route", Body: []byte("must reject")}
	if _, _, err := b.EnqueueChildMessage(ctx, wrongRecipient); !errors.Is(err, storage.ErrChildMessageConflict) {
		t.Fatalf("parent message addressed to parent itself = %v, want conflict", err)
	}
	wrongConsumer := domain.ChildMessageReceipt{ChildSessionID: "child-mail", MessageID: childReply.ID, ConsumerRunID: created.Run.ID, State: domain.ChildMessageReceiptConsumed, CreatedAt: 10, UpdatedAt: 10}
	if _, _, err := b.RecordChildMessageReceipt(ctx, wrongConsumer); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("child activation consumed a parent inbox message = %v, want conflict", err)
	}
	parentReceipt := domain.ChildMessageReceipt{ChildSessionID: "child-mail", MessageID: childReply.ID, ConsumerRunID: parentRunID, State: domain.ChildMessageReceiptConsumed, CreatedAt: 10, UpdatedAt: 10}
	if _, _, err := b.RecordChildMessageReceipt(ctx, parentReceipt); err != nil {
		t.Fatalf("parent safe-point receipt: %v", err)
	}
	binding, err = b.GetChildSessionBinding(ctx, "child-mail")
	if err != nil || binding.ConsumedParentMessageSequence != 1 {
		t.Fatalf("parent consumed cursor=%d err=%v; want 1", binding.ConsumedParentMessageSequence, err)
	}
	for sequence := binding.NextMessageSequence; sequence <= storage.MaxChildMailboxMessagesPerRecipient; sequence++ {
		message, inserted, err := b.EnqueueChildMessage(ctx, domain.ChildMailboxMessage{
			ID: fmt.Sprintf("mail-cap-%d", sequence), ChildSessionID: "child-mail",
			SenderSessionID: parentSessionID, RecipientSessionID: "child-mail",
			IdempotencyKey: fmt.Sprintf("parent-cap-%d", sequence), Body: []byte("bounded mailbox"),
		})
		if err != nil || !inserted || message.Sequence != sequence {
			t.Fatalf("mailbox cap admission seq=%d message=%+v inserted=%v err=%v", sequence, message, inserted, err)
		}
	}
	fullRetry := first
	fullRetry.ID, fullRetry.Sequence, fullRetry.Status = "retry-after-cap", 0, ""
	fullRetry.ConsumedAt, fullRetry.ConsumedByRunID = 0, ""
	if message, inserted, err := b.EnqueueChildMessage(ctx, fullRetry); err != nil || inserted || message.ID != first.ID {
		t.Fatalf("mailbox full rejected idempotent retry: message=%+v inserted=%v err=%v", message, inserted, err)
	}
	if _, _, err := b.EnqueueChildMessage(ctx, domain.ChildMailboxMessage{
		ID: "mail-over-cap", ChildSessionID: "child-mail", SenderSessionID: parentSessionID,
		RecipientSessionID: "child-mail", IdempotencyKey: "parent-over-cap", Body: []byte("must reject"),
	}); !errors.Is(err, storage.ErrChildMailboxFull) {
		t.Fatalf("mail admission beyond lifetime bound = %v, want mailbox full", err)
	}
	if err := b.CloseChildSession(ctx, "child-mail", 10); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.EnqueueChildMessage(ctx, domain.ChildMailboxMessage{ID: "mail-3", ChildSessionID: "child-mail", SenderSessionID: parentSessionID, RecipientSessionID: "child-mail", IdempotencyKey: "parent-msg-3", Body: []byte("after close")}); !errors.Is(err, storage.ErrChildSessionClosed) {
		t.Fatalf("mail after close = %v, want closed", err)
	}
	if pending, err := b.ListPendingChildMessages(ctx, "child-mail", "child-mail", 0, 10); err != nil || len(pending) != 0 {
		t.Fatalf("pending after close=%+v err=%v", pending, err)
	}
}

func cnChildSessionDelete(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	parentID := domain.SessionID("session-cn-delete-parent")
	parentRunID := domain.RunID("run-cn-delete-parent")
	if err := b.CreateSession(ctx, domain.Session{ID: parentID, Title: "parent", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: parentRunID, SessionID: parentID, Status: domain.RunActive, CreatedAt: 2, RootID: parentRunID}); err != nil {
		t.Fatal(err)
	}
	child, err := b.CommitChildSessionAdmission(ctx, conformanceChildAdmission(parentID, parentRunID, "child-delete", "operation-delete", "digest-delete", 3, 4))
	if err != nil {
		t.Fatal(err)
	}
	grandchildInput := conformanceChildAdmission("child-delete", child.Run.ID, "grandchild-delete", "operation-grandchild", "digest-grandchild", 5, 6)
	grandchildInput.Admission.Run.RootID = parentRunID
	grandchildInput.Admission.Run.Depth = 2
	grandchild, err := b.CommitChildSessionAdmission(ctx, grandchildInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.EnqueueChildMessage(ctx, domain.ChildMailboxMessage{ID: "mail-delete", ChildSessionID: "child-delete", SenderSessionID: parentID, RecipientSessionID: "child-delete", IdempotencyKey: "delete-key", Body: []byte("pending"), CreatedAt: 7}); err != nil {
		t.Fatal(err)
	}
	if err := b.DeleteSession(ctx, parentID); err != nil {
		t.Fatal(err)
	}
	for _, sessionID := range []domain.SessionID{parentID, "child-delete", "grandchild-delete"} {
		if _, err := b.GetSession(ctx, sessionID); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("GetSession(%s) = %v, want not found", sessionID, err)
		}
		if _, err := b.GetChildSessionBinding(ctx, sessionID); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("GetChildSessionBinding(%s) = %v, want not found", sessionID, err)
		}
	}
	for _, runID := range []domain.RunID{parentRunID, child.Run.ID, grandchild.Run.ID} {
		if _, err := b.GetRun(ctx, runID); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("GetRun(%s) = %v, want not found", runID, err)
		}
	}
}

func conformanceChildAdmission(parentSessionID domain.SessionID, parentRunID domain.RunID, childSessionID, operationKey, requestDigest string, sessionAt, runAt int64) storage.ChildSessionAdmission {
	childRunID := domain.RunID("run-" + childSessionID)
	task := "task-" + requestDigest
	authority := domain.ChildAuthorityCeiling{
		PolicyProfile: domain.PolicyProfileDefault, PolicyHash: "policy-hash-default",
		SandboxMode: domain.SandboxModeWorkspaceWrite, ApprovalPolicy: domain.ApprovalPolicyAsk,
		ToolNames: []string{"read_file"},
	}
	authorityDigest, err := authority.Digest()
	if err != nil {
		panic(err)
	}
	activationDigest, err := domain.ChildRequestDigest(operationKey, task, []string{"read_file"})
	if err != nil {
		panic(err)
	}
	return storage.ChildSessionAdmission{
		Session: domain.Session{ID: domain.SessionID(childSessionID), Title: "child task", CreatedAt: sessionAt, UpdatedAt: sessionAt},
		Binding: domain.ChildSessionBinding{
			ChildSessionID: domain.SessionID(childSessionID), OriginParentSessionID: parentSessionID, OriginParentRunID: parentRunID,
			AuthorizerRunID: parentRunID, InitialActivationRunID: childRunID, ActivationRunID: childRunID,
			OperationKey: operationKey, RequestDigest: activationDigest, AuthorityCeilingDigest: authorityDigest,
			AuthorityCeiling: authority, ActivationOperationKey: operationKey, ActivationRequestDigest: activationDigest,
			ActivationToolNames: []string{"read_file"},
			State:               domain.ChildSessionOpen, CreatedAt: runAt, UpdatedAt: runAt,
		},
		Admission: storage.RunAdmission{
			Message: domain.Message{ID: "message-" + childSessionID, SessionID: domain.SessionID(childSessionID), RunID: childRunID, Role: domain.RoleUser, CreatedAt: runAt, Content: task},
			Run:     domain.Run{ID: childRunID, SessionID: domain.SessionID(childSessionID), Status: domain.RunAccepted, CreatedAt: runAt, Kind: domain.RunKindChild, ChildMode: domain.ChildModeContinuable, ParentID: parentRunID, RootID: parentRunID, Depth: 1},
			Started: domain.RunEvent{RunID: childRunID, Type: domain.EventRunStarted, CreatedAt: runAt, PayloadVersion: 1, Payload: []byte(`{"provider":"fixture"}`)},
		},
	}
}

func conformanceChildActivation(childSessionID string, authorizerID, rootID domain.RunID, messageAt, runAt int64) storage.ChildSessionActivation {
	childRunID := domain.RunID("run-child-followup")
	sessionID := domain.SessionID(childSessionID)
	toolNames := []string{"read_file"}
	requestDigest, err := domain.ChildRequestDigest("operation-followup", "continue", toolNames)
	if err != nil {
		panic(err)
	}
	return storage.ChildSessionActivation{
		ChildSessionID:  sessionID,
		AuthorizerRunID: authorizerID,
		OperationKey:    "operation-followup",
		RequestDigest:   requestDigest,
		ToolNames:       toolNames,
		Admission: storage.RunAdmission{
			Message: domain.Message{ID: "message-child-followup", SessionID: sessionID, RunID: childRunID, Role: domain.RoleUser, CreatedAt: messageAt, Content: "continue"},
			Run:     domain.Run{ID: childRunID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: runAt, Kind: domain.RunKindChild, ChildMode: domain.ChildModeContinuable, ParentID: authorizerID, RootID: rootID, Depth: 1},
			Started: domain.RunEvent{RunID: childRunID, Type: domain.EventRunStarted, CreatedAt: runAt, PayloadVersion: 1, Payload: []byte(`{"provider":"fixture"}`)},
		},
	}
}

func cnToolOperationIdentity(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	const sessionID = domain.SessionID("session-cn-tool-operation")
	const runID = domain.RunID("run-cn-tool-operation")
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, Title: "operation", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)}}}); err != nil {
		t.Fatal(err)
	}
	const workers = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	var firstErr error
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, inserted, _, err := b.AdmitToolOperation(ctx, conformanceToolOperation(runID, "call-1"))
			mu.Lock()
			defer mu.Unlock()
			if inserted {
				created++
			}
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}()
	}
	wg.Wait()
	if firstErr != nil || created != 1 {
		t.Fatalf("duplicate admission count=%d err=%v, want one admission", created, firstErr)
	}
	conflict := conformanceToolOperation(runID, "call-1")
	conflict.EffectiveArguments = []byte(`{"text":"changed"}`)
	conflict.ArgumentsDigest = conformanceToolOperationDigest(string(conflict.EffectiveArguments))
	if _, _, _, err := b.AdmitToolOperation(ctx, conflict); !errors.Is(err, storage.ErrToolOperationConflict) {
		t.Fatalf("same key with changed payload = %v, want ErrToolOperationConflict", err)
	}
	if _, inserted, _, err := b.AdmitToolOperation(ctx, conformanceToolOperation(runID, "call-2")); err != nil || !inserted {
		t.Fatalf("distinct same-argument admission: inserted=%v err=%v", inserted, err)
	}
	claimed := 0
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, acquired, _, err := b.ClaimToolOperation(ctx, runID, "call-1", fmt.Sprintf("owner-%d", i))
			if err != nil {
				t.Errorf("ClaimToolOperation: %v", err)
				return
			}
			if acquired {
				mu.Lock()
				claimed++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if claimed != 1 {
		t.Fatalf("duplicate claim count = %d, want exactly one", claimed)
	}
}

func cnToolOperationRecovery(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	b := slot.Engine
	t.Cleanup(func() { _ = b.Close() })
	const sessionID = domain.SessionID("session-cn-operation-recovery")
	const runID = domain.RunID("run-cn-operation-recovery")
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, Title: "operation", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)}}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"call-unknown", "call-complete"} {
		if _, _, _, err := b.AdmitToolOperation(ctx, conformanceToolOperation(runID, id)); err != nil {
			t.Fatal(err)
		}
		if _, claimed, _, err := b.ClaimToolOperation(ctx, runID, id, "owner-before-reopen"); err != nil || !claimed {
			t.Fatalf("claim %s: claimed=%v err=%v", id, claimed, err)
		}
	}
	if _, _, err := b.CompleteToolOperation(ctx, runID, "call-complete", "owner-before-reopen", "persisted", ""); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	unknown, err := reopened.GetToolOperation(ctx, runID, "call-unknown")
	if err != nil || unknown.State != domain.ToolOperationClaimed {
		t.Fatalf("unknown operation after reopen = %+v err=%v", unknown, err)
	}
	if _, acquired, _, err := reopened.ClaimToolOperation(ctx, runID, "call-unknown", "owner-after-reopen"); err != nil || acquired {
		t.Fatalf("unknown operation reclaimed after reopen: acquired=%v err=%v", acquired, err)
	}
	completed, err := reopened.GetToolOperation(ctx, runID, "call-complete")
	if err != nil || completed.State != domain.ToolOperationCompleted || completed.Result != "persisted" {
		t.Fatalf("completed operation after reopen = %+v err=%v", completed, err)
	}
}

func conformanceToolOperationDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func conformanceToolOperation(runID domain.RunID, id string) domain.ToolOperation {
	args := []byte(`{"text":"same"}`)
	return domain.ToolOperation{RunID: runID, OperationID: id, ToolName: "write_note",
		RequestDigest: conformanceToolOperationDigest("request:" + string(args)), MiddlewareInputArguments: append([]byte(nil), args...),
		ArgumentsDigest: conformanceToolOperationDigest(string(args)), EffectiveArguments: args}
}

// cnSessionWorkspace catches three storage regressions: dropping the selected
// directory during projection, allowing a started conversation to drift to a
// different directory, and treating an unknown session as a conflict.
func cnSessionWorkspace(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	workspaceStore, ok := b.(storage.SessionWorkspaceStore)
	if !ok {
		t.Fatal("backend does not implement SessionWorkspaceStore")
	}
	session := domain.Session{
		ID: "sess-workspace", Title: "workspace", CreatedAt: 1,
		WorkspacePath: "/projects/alpha",
	}
	if err := b.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	loaded, err := b.GetSession(ctx, session.ID)
	if err != nil || loaded.WorkspacePath != session.WorkspacePath {
		t.Fatalf("GetSession workspace = %q, err=%v; want %q", loaded.WorkspacePath, err, session.WorkspacePath)
	}
	listed, err := b.ListSessions(ctx)
	if err != nil || len(listed) != 1 || listed[0].WorkspacePath != session.WorkspacePath {
		t.Fatalf("ListSessions = %+v, err=%v", listed, err)
	}
	if err := workspaceStore.UpdateSessionWorkspace(ctx, session.ID, "/projects/beta"); err != nil {
		t.Fatalf("UpdateSessionWorkspace before first run: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-workspace", SessionID: session.ID, Status: domain.RunCompleted, CreatedAt: 2}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := workspaceStore.UpdateSessionWorkspace(ctx, session.ID, "/projects/gamma"); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("UpdateSessionWorkspace after first run = %v, want ErrConflict", err)
	}
	loaded, err = b.GetSession(ctx, session.ID)
	if err != nil || loaded.WorkspacePath != "/projects/beta" {
		t.Fatalf("started session workspace = %q, err=%v; want unchanged", loaded.WorkspacePath, err)
	}
	if err := workspaceStore.UpdateSessionWorkspace(ctx, "sess-missing", "/projects/nope"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("unknown session workspace update = %v, want ErrNotFound", err)
	}
}

func fresh(t *testing.T, h Harness) storage.Engine {
	t.Helper()
	slot := h.Setup(t)
	t.Cleanup(func() { _ = slot.Engine.Close() })
	return slot.Engine
}

func replayAll(t *testing.T, b storage.Journal, runID domain.RunID, after domain.EventSeq) []domain.RunEvent {
	t.Helper()
	it, err := b.Replay(context.Background(), runID, after)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	defer func() { _ = it.Close() }()
	var out []domain.RunEvent
	for it.Next() {
		out = append(out, it.Value().Event)
	}
	if it.Err() != nil {
		t.Fatalf("iterator: %v", it.Err())
	}
	return out
}

func ev(typ domain.EventType) domain.RunEvent {
	return domain.RunEvent{Type: typ, CreatedAt: 1, PayloadVersion: 1, Payload: []byte(`{}`)}
}

func cnAtomicAppend(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-1",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventModelCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := replayAll(t, b, "run-1", 0); len(got) != 3 {
		t.Fatalf("events = %d, want all 3 of the commit", len(got))
	}
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-1",
		Events: []domain.RunEvent{
			ev(domain.EventRunCompleted), ev(domain.EventRunFailed),
		},
	}); !errors.Is(err, storage.ErrCommitInvalid) {
		t.Fatalf("two-terminal commit: err = %v, want ErrCommitInvalid", err)
	}
	if got := replayAll(t, b, "run-1", 0); len(got) != 3 {
		t.Fatalf("rejected commit leaked rows: %d events", len(got))
	}
}

func cnMonotonicSequence(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		seq, err := b.Append(ctx, storage.Commit{
			RunID:  "run-seq",
			Events: []domain.RunEvent{ev(domain.EventModelDelta)},
		})
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		if seq != domain.EventSeq(i+1) {
			t.Fatalf("seq = %d, want %d", seq, i+1)
		}
	}
	got := replayAll(t, b, "run-seq", 0)
	for i, e := range got {
		if e.Seq != domain.EventSeq(i+1) {
			t.Fatalf("replay seq[%d] = %d, want contiguous", i, e.Seq)
		}
	}
}

func cnExpectedVersionConflict(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	snap := b.Snapshot()
	if err := snap.Put(ctx, "k", []byte("v"), 1); !errors.Is(err, storage.ErrVersionConflict) {
		t.Fatalf("create with expectVersion=1: err = %v, want conflict", err)
	}
	if err := snap.Put(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := snap.Put(ctx, "k", []byte("v2"), 0); !errors.Is(err, storage.ErrVersionConflict) {
		t.Fatalf("stale create: err = %v, want conflict", err)
	}
}

func cnIdempotentReplay(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-replay",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	first := replayAll(t, b, "run-replay", 0)
	second := replayAll(t, b, "run-replay", 0)
	if len(first) != len(second) {
		t.Fatalf("replay lengths diverged: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Seq != second[i].Seq || first[i].Type != second[i].Type {
			t.Fatalf("replay pass diverged at %d", i)
		}
	}
}

func cnRetryDivergenceRefused(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-retry",
		Events: []domain.RunEvent{ev(domain.EventRunStarted), ev(domain.EventRunCompleted)},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-retry",
		Events: []domain.RunEvent{ev(domain.EventModelDelta)},
	}); !errors.Is(err, storage.ErrRunClosed) {
		t.Fatalf("divergent retry: err = %v, want ErrRunClosed", err)
	}
	if got := replayAll(t, b, "run-retry", 0); len(got) != 2 {
		t.Fatalf("journal grew under a refused retry: %d events", len(got))
	}
}

func cnExactlyOneTerminal(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-one",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	for _, typ := range []domain.EventType{
		domain.EventRunCompleted, domain.EventRunFailed, domain.EventRunCancelled, domain.EventModelDelta,
	} {
		if _, err := b.Append(ctx, storage.Commit{
			RunID: "run-one", Events: []domain.RunEvent{ev(typ)},
		}); !errors.Is(err, storage.ErrRunClosed) {
			t.Fatalf("append %s after terminal: err = %v, want ErrRunClosed", typ, err)
		}
	}
	terminals := 0
	for _, e := range replayAll(t, b, "run-one", 0) {
		if e.Type.Terminal() {
			terminals++
		}
	}
	if terminals != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", terminals)
	}
}

func cnFirstWriterWinsApproval(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	apr := domain.Approval{
		ID: "apr-fw", RunID: "run-fw", ToolCallID: "tc-1",
		Decision: domain.ApprovalPending, ExpiresAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := b.CreateApproval(ctx, apr); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := b.DecideApproval(ctx, apr.ID, domain.ApprovalApproved)
			if err != nil {
				t.Errorf("DecideApproval: %v", err)
				return
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for ok := range results {
		if ok {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent decisions won = %d, want exactly 1", wins)
	}
	if pending, _ := b.ListPendingApprovals(ctx); len(pending) != 0 {
		t.Fatalf("pending after decision = %d, want 0", len(pending))
	}
}

func cnRestartRepairView(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	b := slot.Engine
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-rr", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-rr", SessionID: "sess-rr", Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := b.CreateApproval(ctx, domain.Approval{
		ID: "apr-rr", RunID: "run-rr", Decision: domain.ApprovalPending, ExpiresAt: 9999,
	}); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	active, err := b.ListActiveRuns(ctx)
	if err != nil || len(active) != 1 || active[0].ID != "run-rr" {
		t.Fatalf("active after reopen = %+v, %v", active, err)
	}
	pending, err := b.ListPendingApprovals(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != "apr-rr" {
		t.Fatalf("pending after reopen = %+v, %v", pending, err)
	}
}

func cnTornFinalWrite(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	b := slot.Engine
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-torn",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventModelCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	got := replayAll(t, b, "run-torn", 0)
	if len(got) != 3 || got[2].Type != domain.EventModelCompleted {
		t.Fatalf("reopen replay = %d events, tail %v; want the committed stream", len(got), got[len(got)-1].Type)
	}
}

func cnMalformedPayload(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	malformed := []byte(`{"broken":`)
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-bad",
		Events: []domain.RunEvent{
			{Type: domain.EventModelDelta, CreatedAt: 1, PayloadVersion: 1, Payload: malformed},
		},
	}); err != nil {
		t.Fatalf("Append malformed payload: %v", err)
	}
	got := replayAll(t, b, "run-bad", 0)
	if len(got) != 1 || !bytes.Equal(got[0].Payload, malformed) {
		t.Fatalf("malformed payload was rewritten: %q", got[0].Payload)
	}
}

func cnOrphanCheckpoint(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if err := b.Blobs().Put(ctx, "ckpt-orphan", []byte("gen-1")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	orphaner, ok := b.(storage.CheckpointOrphaner)
	if !ok {
		t.Fatal("engine does not implement CheckpointOrphaner")
	}
	if err := orphaner.DropCheckpointPointer(ctx, "ckpt-orphan"); err != nil {
		t.Fatalf("drop pointer: %v", err)
	}
	n, err := orphaner.CountCheckpointGenerations(ctx, "ckpt-orphan")
	if err != nil {
		t.Fatalf("count generations: %v", err)
	}
	if n != 1 {
		t.Fatalf("orphan generations = %d, want 1 visible for recovery", n)
	}
	if err := orphaner.DeleteCheckpointGenerations(ctx, "ckpt-orphan"); err != nil {
		t.Fatalf("cleanup orphan: %v", err)
	}
	n, err = orphaner.CountCheckpointGenerations(ctx, "ckpt-orphan")
	if err != nil {
		t.Fatalf("recount: %v", err)
	}
	if n != 0 {
		t.Fatalf("orphan cleanup left %d rows", n)
	}
}

func cnApprovalAfterKill(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	b := slot.Engine
	if err := b.CreateApproval(ctx, domain.Approval{
		ID: "apr-kill", RunID: "run-kill", ToolCallID: "tc-1",
		Decision: domain.ApprovalPending, ExpiresAt: 9999, ResumeTarget: "rt",
	}); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	ok, err := b.DecideApproval(ctx, "apr-kill", domain.ApprovalApproved)
	if err != nil || !ok {
		t.Fatalf("decide after reopen = %v/%v, want granted", ok, err)
	}
	apr, err := b.GetApproval(ctx, "apr-kill")
	if err != nil || apr.Decision != domain.ApprovalApproved {
		t.Fatalf("approval after decide = %+v, %v", apr, err)
	}
}

func cnPayloadByteFidelity(t *testing.T, h Harness) {
	b := fresh(t, h)
	canary := []byte(`{"secret":"cn13-byte-fidelity-canary"}`)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-canary",
		Events: []domain.RunEvent{
			{Type: domain.EventModelDelta, CreatedAt: 1, PayloadVersion: 1, Payload: canary},
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got := replayAll(t, b, "run-canary", 0)
	if len(got) != 1 || !bytes.Equal(got[0].Payload, canary) {
		t.Fatalf("payload mutated in storage: %q", got[0].Payload)
	}
}

func cnDualHandleSafety(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	first := slot.Engine
	t.Cleanup(func() { _ = first.Close() })
	second, err := slot.OpenSecond()
	switch h.DualOpen {
	case DualOpenExclusive:
		if !errors.Is(err, storage.ErrLeaseHeld) {
			if second != nil {
				_ = second.Close()
			}
			t.Fatalf("second Open = %v, want ErrLeaseHeld", err)
		}
		if _, err := first.Append(ctx, storage.Commit{
			RunID: "run-dual", Events: []domain.RunEvent{ev(domain.EventRunStarted)},
		}); err != nil {
			t.Fatalf("append via first handle: %v", err)
		}
		if got := replayAll(t, first, "run-dual", 0); len(got) != 1 {
			t.Fatalf("exclusive journal = %d events, want the first commit", len(got))
		}
		return
	default:
		if err != nil {
			t.Fatalf("second Open must not fail or corrupt: %v", err)
		}
		t.Cleanup(func() { _ = second.Close() })
		if _, err := first.Append(ctx, storage.Commit{
			RunID: "run-dual", Events: []domain.RunEvent{ev(domain.EventRunStarted)},
		}); err != nil {
			t.Fatalf("append via first handle: %v", err)
		}
		if _, err := second.Append(ctx, storage.Commit{
			RunID: "run-dual", Events: []domain.RunEvent{ev(domain.EventRunCompleted)},
		}); err != nil {
			t.Fatalf("append via second handle: %v", err)
		}
		got := replayAll(t, first, "run-dual", 0)
		if len(got) != 2 || got[1].Type != domain.EventRunCompleted {
			t.Fatalf("dual-handle journal = %d events, want both commits", len(got))
		}
	}
}

func cnConcurrentWriters(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	const writers, perRun = 8, 10

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			runID := domain.RunID(fmt.Sprintf("run-cw-%d", w))
			for i := 0; i < perRun; i++ {
				if _, err := b.Append(ctx, storage.Commit{
					RunID:  runID,
					Events: []domain.RunEvent{ev(domain.EventModelDelta)},
				}); err != nil {
					errs <- fmt.Errorf("writer %d append %d: %w", w, i, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for w := 0; w < writers; w++ {
		got := replayAll(t, b, domain.RunID(fmt.Sprintf("run-cw-%d", w)), 0)
		if len(got) != perRun {
			t.Fatalf("run %d events = %d, want %d", w, len(got), perRun)
		}
		for i, e := range got {
			if e.Seq != domain.EventSeq(i+1) {
				t.Fatalf("run %d seq[%d] = %d, want contiguous", w, i, e.Seq)
			}
		}
	}
}

func cnReplayAfterDisconnect(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-tail",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted),
			ev(domain.EventModelDelta),
			ev(domain.EventModelDelta),
			ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	tail := replayAll(t, b, "run-tail", 2)
	if len(tail) != 2 || tail[0].Seq != 3 || tail[1].Seq != 4 {
		t.Fatalf("tail = %+v, want exactly seq 3..4", tail)
	}
	if tail[1].Type != domain.EventRunCompleted {
		t.Fatalf("tail terminal = %s, want run.completed", tail[1].Type)
	}
}

func cnMessageProvenance(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-prov", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	channelMsg := domain.Message{
		ID: "msg-prov-channel", SessionID: "sess-prov", Role: domain.RoleUser,
		CreatedAt: 2, Content: "hello from the world",
		Source: "channel", Channel: "telegram", ChatID: "chat-123", ChannelMessageID: "tg-456",
	}
	legacyMsg := domain.Message{
		ID: "msg-prov-legacy", SessionID: "sess-prov", Role: domain.RoleUser,
		CreatedAt: 3, Content: "hello from the ui",
	}
	for i, m := range []domain.Message{channelMsg, legacyMsg} {
		if err := b.AppendMessage(ctx, m); err != nil {
			t.Fatalf("AppendMessage %d: %v", i, err)
		}
	}
	got, err := b.ListMessages(ctx, "sess-prov")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("messages = %d, want 2", len(got))
	}
	c := got[0]
	if c.ID != channelMsg.ID || c.Role != domain.RoleUser || c.Content != channelMsg.Content {
		t.Fatalf("channel row base fields drifted: %+v", c)
	}
	if c.Source != "channel" || c.Channel != "telegram" || c.ChatID != "chat-123" || c.ChannelMessageID != "tg-456" {
		t.Fatalf("channel provenance did not round-trip: %+v", c)
	}
	if c.EffectiveSource() != "channel" {
		t.Fatalf("EffectiveSource = %q, want channel", c.EffectiveSource())
	}
	l := got[1]
	if l.ID != legacyMsg.ID || l.Role != domain.RoleUser || l.Content != legacyMsg.Content {
		t.Fatalf("legacy row base fields drifted: %+v", l)
	}
	if l.Source != "" {
		t.Fatalf("legacy row Source = %q, want empty", l.Source)
	}
	if l.EffectiveSource() != "ui" {
		t.Fatalf("legacy EffectiveSource = %q, want ui", l.EffectiveSource())
	}
}

func cnFileVersionChain(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-fv", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Record mutations without error and keep the tracker round-trip
	// exact. Chain contents (baseline/intermediate/dedupe/retention) are
	// asserted by backend-local tests that can query the table directly;
	// the interface intentionally exposes no version reads until a restore
	// consumer exists (RB-L2-DEFER).
	mutations := []struct{ old, new string }{
		{"", "v1"},
		{"v1", "v2"},
		{"v2", "v3"},
	}
	for i, m := range mutations {
		if err := b.RecordFileMutation(ctx, "sess-fv", "run-fv", "a.go", []byte(m.old), []byte(m.new)); err != nil {
			t.Fatalf("RecordFileMutation %d: %v", i, err)
		}
	}

	if _, ok, err := b.LastFileAccess(ctx, "sess-fv", "a.go"); ok || err != nil {
		t.Fatalf("LastFileAccess before tracking = ok=%v err=%v, want ok=false err=nil", ok, err)
	}
	if err := b.TrackFileAccess(ctx, "sess-fv", "a.go", 100); err != nil {
		t.Fatalf("TrackFileAccess: %v", err)
	}
	if err := b.TrackFileAccess(ctx, "sess-fv", "a.go", 200); err != nil {
		t.Fatalf("TrackFileAccess (upsert): %v", err)
	}
	at, ok, err := b.LastFileAccess(ctx, "sess-fv", "a.go")
	if err != nil || !ok || at != 200 {
		t.Fatalf("LastFileAccess = (%d, %v, %v), want (200, true, nil)", at, ok, err)
	}
	if _, ok, _ := b.LastFileAccess(ctx, "sess-fv", "other.go"); ok {
		t.Fatalf("LastFileAccess for untracked path = ok, want not ok")
	}

	// Session deletion cascades both tables in one transaction.
	if err := b.DeleteSession(ctx, "sess-fv"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, ok, err := b.LastFileAccess(ctx, "sess-fv", "a.go"); ok || err != nil {
		t.Fatalf("LastFileAccess after DeleteSession = ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

func cnRunsBySession(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	runs := []domain.Run{
		{ID: "run-a", SessionID: "sess-pin", Status: domain.RunCompleted, CreatedAt: 1},
		{ID: "run-b", SessionID: "sess-pin", Status: domain.RunActive, CreatedAt: 2},
		{ID: "run-x", SessionID: "sess-other", Status: domain.RunActive, CreatedAt: 3},
	}
	for _, r := range runs {
		if err := b.CreateRun(ctx, r); err != nil {
			t.Fatalf("CreateRun %s: %v", r.ID, err)
		}
	}
	got, err := b.ListRunsBySession(ctx, "sess-pin")
	if err != nil {
		t.Fatalf("ListRunsBySession: %v", err)
	}
	if len(got) != 2 || got[0].ID != "run-a" || got[1].ID != "run-b" {
		t.Fatalf("ListRunsBySession(sess-pin) = %+v, want [run-a run-b] in creation order", got)
	}
	if got[1].Status != domain.RunActive || got[0].Status != domain.RunCompleted {
		t.Fatalf("ListRunsBySession must return all statuses, got %s then %s", got[0].Status, got[1].Status)
	}
	empty, err := b.ListRunsBySession(ctx, "sess-none")
	if err != nil || len(empty) != 0 {
		t.Fatalf("ListRunsBySession(unknown) = %+v, %v; want empty, nil", empty, err)
	}
	latestStore, ok := b.(storage.LatestPrimaryRunStore)
	if !ok {
		t.Fatal("first-party backend lacks LatestPrimaryRunStore")
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-child", SessionID: "sess-pin", Status: domain.RunActive, CreatedAt: 4, Kind: domain.RunKindChild, ParentID: "run-b", RootID: "run-b", Depth: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-c", SessionID: "sess-pin", Status: domain.RunCompleted, CreatedAt: 3, Kind: domain.RunKindPrimary}); err != nil {
		t.Fatal(err)
	}
	latest, err := latestStore.LatestPrimaryRunBySession(ctx, "sess-pin")
	if err != nil || latest.ID != "run-c" {
		t.Fatalf("LatestPrimaryRunBySession = %+v/%v, want run-c (not newer child)", latest, err)
	}
	if _, err := latestStore.LatestPrimaryRunBySession(ctx, "sess-none"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("LatestPrimaryRunBySession unknown err = %v, want ErrNotFound", err)
	}
}

func cnCompactionsBySession(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	records := []storage.SessionCompaction{
		{SessionID: "sess-cp", RunID: "run-c1", Summary: "older", TailFrom: 100, DroppedCount: 4, CreatedAt: 100},
		{SessionID: "sess-cp", RunID: "run-c2", Summary: "newer", TailFrom: 200, DroppedCount: 6, CreatedAt: 200},
		{SessionID: "sess-cp", RunID: "run-c9", Summary: "tie-newest", TailFrom: 300, DroppedCount: 2, CreatedAt: 200},
		{SessionID: "sess-other", RunID: "run-z", Summary: "elsewhere", TailFrom: 400, DroppedCount: 1, CreatedAt: 300},
	}
	for _, rec := range records {
		if err := b.SaveSessionCompaction(ctx, rec); err != nil {
			t.Fatalf("SaveSessionCompaction %s: %v", rec.RunID, err)
		}
	}
	got, err := b.ListSessionCompactions(ctx, "sess-cp", 10)
	if err != nil {
		t.Fatalf("ListSessionCompactions: %v", err)
	}
	wantOrder := []domain.RunID{"run-c9", "run-c2", "run-c1"}
	if len(got) != len(wantOrder) {
		t.Fatalf("ListSessionCompactions = %d rows, want %d", len(got), len(wantOrder))
	}
	for i, want := range wantOrder {
		if got[i].RunID != want {
			t.Fatalf("row %d = %s, want %s (newest first)", i, got[i].RunID, want)
		}
	}
	if got[0].Summary != "tie-newest" || got[0].DroppedCount != 2 || got[0].TailFrom != 300 {
		t.Fatalf("row 0 fields = %+v, want tie-newest record", got[0])
	}
	capped, err := b.ListSessionCompactions(ctx, "sess-cp", 2)
	if err != nil || len(capped) != 2 || capped[0].RunID != "run-c9" {
		t.Fatalf("limit=2 = %+v, %v; want top 2 newest", capped, err)
	}
	if none, err := b.ListSessionCompactions(ctx, "sess-none", 10); err != nil || len(none) != 0 {
		t.Fatalf("unknown session = %+v, %v; want empty, nil", none, err)
	}
	if zero, err := b.ListSessionCompactions(ctx, "sess-cp", 0); err != nil || len(zero) != 0 {
		t.Fatalf("limit=0 = %+v, %v; want empty, nil", zero, err)
	}
}

func cnSessionTruncationMarkers(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, ok, err := b.LatestSessionTruncation(ctx, "sess-tw"); err != nil || ok {
		t.Fatalf("no-marker read = ok=%v, %v; want false, nil", ok, err)
	}
	messages := []domain.Message{
		{ID: "msg-1", Role: domain.RoleUser, Content: "one"},
		{ID: "msg-2", Role: domain.RoleAssistant, Content: "two"},
		{ID: "msg-3", Role: domain.RoleUser, Content: "three"},
		{ID: "msg-4", Role: domain.RoleAssistant, Content: "four"},
	}
	for _, marker := range []storage.SessionTruncation{
		{SessionID: "sess-tw", CutoffMessageID: "msg-3", TailMessageID: "msg-4", Reason: storage.TruncationRewind, CreatedAt: 100},
		{SessionID: "sess-tw", CutoffMessageID: "msg-2", TailMessageID: "msg-4", Reason: storage.TruncationRewind, CreatedAt: 200},
		{SessionID: "sess-other", CutoffMessageID: "msg-1", TailMessageID: "msg-4", Reason: storage.TruncationEdit, CreatedAt: 300},
	} {
		if err := b.RecordSessionTruncation(ctx, marker); err != nil {
			t.Fatalf("RecordSessionTruncation %s: %v", marker.CutoffMessageID, err)
		}
	}
	got, ok, err := b.LatestSessionTruncation(ctx, "sess-tw")
	if err != nil || !ok {
		t.Fatalf("LatestSessionTruncation = ok=%v, %v; want true, nil", ok, err)
	}
	if got.CutoffMessageID != "msg-2" || got.TailMessageID != "msg-4" || got.Reason != storage.TruncationRewind {
		t.Fatalf("latest marker = %+v, want msg-2..msg-4 rewind (newest wins per session)", got)
	}
	folded := storage.ApplySessionTruncation(messages, got)
	if len(folded) != 1 || folded[0].ID != "msg-1" {
		t.Fatalf("rewind fold = %+v, want messages before the cutoff", folded)
	}
	// A turn appended after the rewind sits beyond the tail anchor and must
	// stay visible — the edit flow is rewind + a fresh turn/start.
	afterTurn := append(append([]domain.Message{}, messages...), domain.Message{ID: "msg-5", Role: domain.RoleUser, Content: "five"})
	refolded := storage.ApplySessionTruncation(afterTurn, got)
	if len(refolded) != 2 || refolded[0].ID != "msg-1" || refolded[1].ID != "msg-5" {
		t.Fatalf("post-rewind fold = %+v, want msg-1 + the new turn", refolded)
	}
	// A later fork anchor is audit-only: it shows up as the newest row for
	// audit reads but never enters the view fold nor shadows view markers.
	if err := b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: "sess-tw", CutoffMessageID: "msg-4", TailMessageID: "msg-4", Reason: storage.TruncationFork, ForkSessionID: "sess-fk", CreatedAt: 400}); err != nil {
		t.Fatalf("RecordSessionTruncation fork anchor: %v", err)
	}
	if audit, ok, err := b.LatestSessionTruncation(ctx, "sess-tw"); err != nil || !ok || audit.Reason != storage.TruncationFork {
		t.Fatalf("latest audit marker = %+v, ok=%v, err=%v; want the fork anchor", audit, ok, err)
	}
	viewMarkers, err := b.ListViewTruncations(ctx, "sess-tw")
	if err != nil || len(viewMarkers) != 2 {
		t.Fatalf("ListViewTruncations = %d markers, %v; want only the 2 rewind rows (fork anchor excluded)", len(viewMarkers), err)
	}
	foldedUnion := storage.ApplySessionTruncations(messages, viewMarkers)
	if len(foldedUnion) != 1 || foldedUnion[0].ID != "msg-1" {
		t.Fatalf("union fold = %+v, want msg-1 (successive rewinds accumulate)", foldedUnion)
	}
	// Non-contiguous ranges: a row appended between two rewinds stays
	// visible unless a later range captures it.
	if err := b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: "sess-un", CutoffMessageID: "msg-2", TailMessageID: "msg-2", Reason: storage.TruncationEdit, CreatedAt: 10}); err != nil {
		t.Fatalf("RecordSessionTruncation un/1: %v", err)
	}
	if err := b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: "sess-un", CutoffMessageID: "msg-4", TailMessageID: "msg-4", Reason: storage.TruncationEdit, CreatedAt: 20}); err != nil {
		t.Fatalf("RecordSessionTruncation un/2: %v", err)
	}
	unMarkers, err := b.ListViewTruncations(ctx, "sess-un")
	if err != nil || len(unMarkers) != 2 {
		t.Fatalf("sess-un ListViewTruncations = %d markers, %v; want 2", len(unMarkers), err)
	}
	unFolded := storage.ApplySessionTruncations(messages, unMarkers)
	if len(unFolded) != 2 || unFolded[0].ID != "msg-1" || unFolded[1].ID != "msg-3" {
		t.Fatalf("non-contiguous union fold = %+v, want msg-1 + msg-3", unFolded)
	}
	for _, reason := range []string{storage.TruncationFork, storage.TruncationForkedFrom} {
		kept := storage.ApplySessionTruncation(messages, storage.SessionTruncation{Reason: reason, CutoffMessageID: "msg-2"})
		if len(kept) != len(messages) {
			t.Fatalf("%s fold = %d rows, want unfiltered (provenance markers filter nothing)", reason, len(kept))
		}
	}
	failOpen := storage.ApplySessionTruncation(messages, storage.SessionTruncation{Reason: storage.TruncationRewind, CutoffMessageID: "msg-gone"})
	if len(failOpen) != len(messages) {
		t.Fatalf("stale-cutoff fold = %d rows, want unfiltered (fail-open)", len(failOpen))
	}
	failOpenTail := storage.ApplySessionTruncation(messages, storage.SessionTruncation{Reason: storage.TruncationRewind, CutoffMessageID: "msg-2", TailMessageID: "msg-gone"})
	if len(failOpenTail) != len(messages) {
		t.Fatalf("stale-tail fold = %d rows, want unfiltered (fail-open)", len(failOpenTail))
	}
	// Rewinding the LAST message makes cutoff and tail the SAME id; both
	// anchors must resolve or the fold silently fails open.
	lastFold := storage.ApplySessionTruncation(messages, storage.SessionTruncation{Reason: storage.TruncationEdit, CutoffMessageID: "msg-4", TailMessageID: "msg-4"})
	if len(lastFold) != 3 || lastFold[2].ID != "msg-3" {
		t.Fatalf("cutoff==tail fold = %+v, want msg-1..msg-3", lastFold)
	}
}

func cnMessageProjectionIdempotence(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	for _, session := range []domain.Session{
		{ID: "sess-proj-a", Title: "projection", CreatedAt: 1},
		{ID: "sess-proj-b", Title: "projection alternate", CreatedAt: 2},
	} {
		if err := b.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession %s: %v", session.ID, err)
		}
	}

	base := domain.Message{
		ID:         "msg-proj",
		SessionID:  "sess-proj-a",
		RunID:      "run-proj",
		Role:       domain.RoleAssistant,
		CreatedAt:  101,
		Content:    "projected answer",
		ToolCallID: "call-1",
		ToolName:   "read_file",
		// A nil tool-args slice is normalized to the same stored empty blob
		// as an explicitly empty slice.
		ToolArgs:         nil,
		Source:           "channel",
		Channel:          "telegram",
		ChatID:           "chat-1",
		ChannelMessageID: "tg-1",
	}
	inserted, err := b.AppendMessageIfAbsent(ctx, base)
	if err != nil || !inserted {
		t.Fatalf("first projection = inserted %v, err %v; want true, nil", inserted, err)
	}

	duplicate := base
	duplicate.ToolArgs = []byte{}
	inserted, err = b.AppendMessageIfAbsent(ctx, duplicate)
	if err != nil || inserted {
		t.Fatalf("normalized duplicate = inserted %v, err %v; want false, nil", inserted, err)
	}
	assertSingleProjectedMessage(t, b, ctx, base)

	variants := []struct {
		name   string
		mutate func(*domain.Message)
	}{
		{"session_id", func(m *domain.Message) { m.SessionID = "sess-proj-b" }},
		{"run_id", func(m *domain.Message) { m.RunID = "run-proj-other" }},
		{"role", func(m *domain.Message) { m.Role = domain.RoleTool }},
		{"created_at", func(m *domain.Message) { m.CreatedAt = 102 }},
		{"content", func(m *domain.Message) { m.Content = "different answer" }},
		{"tool_call_id", func(m *domain.Message) { m.ToolCallID = "call-2" }},
		{"tool_name", func(m *domain.Message) { m.ToolName = "write_file" }},
		{"tool_args", func(m *domain.Message) { m.ToolArgs = []byte(`{"path":"other"}`) }},
		{"source", func(m *domain.Message) { m.Source = "ui" }},
		{"channel", func(m *domain.Message) { m.Channel = "discord" }},
		{"chat_id", func(m *domain.Message) { m.ChatID = "chat-2" }},
		{"channel_message_id", func(m *domain.Message) { m.ChannelMessageID = "tg-2" }},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			candidate := base
			variant.mutate(&candidate)
			inserted, err := b.AppendMessageIfAbsent(ctx, candidate)
			if inserted || !errors.Is(err, storage.ErrProjectionConflict) {
				t.Fatalf("conflicting projection = inserted %v, err %v; want false, ErrProjectionConflict", inserted, err)
			}
			assertSingleProjectedMessage(t, b, ctx, base)
		})
	}

	for _, rejected := range []struct {
		name   string
		attach func(*domain.Message)
	}{
		{"attachment", func(m *domain.Message) {
			m.ID = "msg-proj-attachment"
			m.Attachments = []domain.Attachment{{Name: "x.png", MimeType: "image/png", Data: []byte{1}}}
		}},
		{"file_context", func(m *domain.Message) {
			m.ID = "msg-proj-file-context"
			m.FileContexts = []domain.FileContext{{Path: "x.go", Name: "x.go", Size: 1, Content: []byte("x")}}
		}},
	} {
		t.Run(rejected.name, func(t *testing.T) {
			candidate := base
			rejected.attach(&candidate)
			inserted, err := b.AppendMessageIfAbsent(ctx, candidate)
			if inserted || err == nil {
				t.Fatalf("projected row with %s = inserted %v, err %v; want false and an error", rejected.name, inserted, err)
			}
			assertSingleProjectedMessage(t, b, ctx, base)
		})
	}
}

func cnConcurrentMessageProjection(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-proj-concurrent", Title: "projection", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	message := domain.Message{
		ID:        "msg-proj-concurrent",
		SessionID: "sess-proj-concurrent",
		RunID:     "run-proj-concurrent",
		Role:      domain.RoleAssistant,
		CreatedAt: 201,
		Content:   "one durable projection",
		ToolArgs:  []byte(`{"normalized":"empty"}`),
	}

	const callers = 32
	start := make(chan struct{})
	var ready, wg sync.WaitGroup
	ready.Add(callers)
	wg.Add(callers)
	results := make(chan struct {
		inserted bool
		err      error
	}, callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			ready.Done()
			<-start
			inserted, err := b.AppendMessageIfAbsent(ctx, message)
			results <- struct {
				inserted bool
				err      error
			}{inserted: inserted, err: err}
		}()
	}
	ready.Wait()
	close(start)
	wg.Wait()
	close(results)

	wins := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent projection: %v", result.err)
		}
		if result.inserted {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent projection winners = %d, want exactly 1", wins)
	}
	assertSingleProjectedMessage(t, b, ctx, message)
}

func cnSessionActivity(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	sessionID := domain.SessionID("sess-activity")
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, Title: "activity", CreatedAt: 100, UpdatedAt: 100}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := b.GetSession(ctx, sessionID)
	if err != nil || got.UpdatedAt != 100 {
		t.Fatalf("created UpdatedAt = %d, %v; want 100", got.UpdatedAt, err)
	}
	activity, ok := b.(storage.SessionActivityStore)
	if !ok {
		t.Fatal("backend does not implement SessionActivityStore")
	}
	if err := activity.TouchSession(ctx, sessionID, 50); err != nil {
		t.Fatalf("TouchSession older: %v", err)
	}
	got, _ = b.GetSession(ctx, sessionID)
	if got.UpdatedAt != 100 {
		t.Fatalf("older touch moved UpdatedAt to %d, want 100", got.UpdatedAt)
	}
	if err := activity.TouchSession(ctx, sessionID, 200); err != nil {
		t.Fatalf("TouchSession newer: %v", err)
	}
	got, _ = b.GetSession(ctx, sessionID)
	if got.UpdatedAt != 200 {
		t.Fatalf("newer touch UpdatedAt = %d, want 200", got.UpdatedAt)
	}
	if err := activity.TouchSession(ctx, "sess-missing", 400); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("TouchSession missing = %v, want ErrNotFound", err)
	}
	projectedID := domain.SessionID("sess-projected-activity")
	if err := b.CreateSession(ctx, domain.Session{ID: projectedID, Title: "projected", CreatedAt: 100, UpdatedAt: 100}); err != nil {
		t.Fatalf("CreateSession projected: %v", err)
	}
	created, err := b.AppendMessageIfAbsent(ctx, domain.Message{ID: "msg-projected-activity", SessionID: projectedID, Role: domain.RoleAssistant, CreatedAt: 250, Content: "durable"})
	if err != nil || !created {
		t.Fatalf("AppendMessageIfAbsent = %v, %v; want created", created, err)
	}
	projected, err := b.GetSession(ctx, projectedID)
	if err != nil || projected.UpdatedAt != 250 {
		t.Fatalf("projected activity = %+v, %v; want UpdatedAt 250", projected, err)
	}
	if err := b.AppendMessage(ctx, domain.Message{ID: "msg-activity", SessionID: sessionID, Role: domain.RoleUser, CreatedAt: 300, Content: "hello"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	got, _ = b.GetSession(ctx, sessionID)
	if got.UpdatedAt != 300 {
		t.Fatalf("message activity UpdatedAt = %d, want 300", got.UpdatedAt)
	}
	if err := b.RenameSession(ctx, sessionID, "renamed"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	got, _ = b.GetSession(ctx, sessionID)
	if got.UpdatedAt < 300 || got.Title != "renamed" {
		t.Fatalf("rename session = %+v, want title renamed and non-decreasing activity", got)
	}
	if err := b.UpdateSandboxPolicy(ctx, sessionID, domain.SandboxModeReadOnly, domain.ApprovalPolicyNever); err != nil {
		t.Fatalf("UpdateSandboxPolicy: %v", err)
	}
	got, _ = b.GetSession(ctx, sessionID)
	if got.UpdatedAt < 300 {
		t.Fatalf("permission activity moved UpdatedAt backwards to %d", got.UpdatedAt)
	}
}

func cnModifiedFiles(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	store, ok := b.(storage.ModifiedFileStore)
	if !ok {
		t.Fatal("backend does not implement ModifiedFileStore")
	}
	if err := b.RecordFileMutation(ctx, "sess-sidebar-files", "run-sidebar", "a.go", []byte("old\nline\n"), []byte("old\nnew\nline\n")); err != nil {
		t.Fatalf("RecordFileMutation first: %v", err)
	}
	if err := b.RecordFileMutation(ctx, "sess-sidebar-files", "run-sidebar", "a.go", []byte("old\nnew\nline\n"), []byte("new\nline\n")); err != nil {
		t.Fatalf("RecordFileMutation second: %v", err)
	}
	files, err := store.ListModifiedFiles(ctx, "sess-sidebar-files", 1)
	if err != nil {
		t.Fatalf("ListModifiedFiles: %v", err)
	}
	if len(files) != 1 || files[0].Path != "a.go" || files[0].UpdatedAt <= 0 {
		t.Fatalf("modified files = %+v, want one timestamped a.go", files)
	}
	if files[0].Diff.Additions != 1 || files[0].Diff.Deletions != 1 {
		t.Fatalf("net diff = %+v, want +1/-1", files[0].Diff)
	}
	if empty, err := store.ListModifiedFiles(ctx, "sess-sidebar-files", 0); err != nil || len(empty) != 0 {
		t.Fatalf("zero limit = %+v, %v; want empty", empty, err)
	}
	if unknown, err := store.ListModifiedFiles(ctx, "sess-sidebar-unknown", 10); err != nil || len(unknown) != 0 {
		t.Fatalf("unknown session = %+v, %v; want empty", unknown, err)
	}
}

func cnAttributedModelUsage(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	usageStore, ok := b.(storage.TokenUsageStore)
	if !ok {
		t.Fatal("backend does not implement TokenUsageStore")
	}
	sessionUsageStore, ok := b.(storage.SessionTokenUsageStore)
	if !ok {
		t.Fatal("backend does not implement SessionTokenUsageStore")
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-usage", Title: "usage", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-usage", SessionID: "sess-usage", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	events := []domain.RunEvent{
		{Type: domain.EventRunStarted, CreatedAt: 1, PayloadVersion: 1, Payload: []byte(`{"provider":"first","model":"main"}`)},
		{Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{"provider":"duplicate","model":"wrong"}`)},
		{Type: domain.EventModelUsage, CreatedAt: 3, PayloadVersion: 1, Payload: []byte(`{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cached_tokens":2,"source":"main"}`)},
		{Type: domain.EventModelUsage, CreatedAt: 4, PayloadVersion: 1, Payload: []byte(`{"prompt_tokens":20,"completion_tokens":8,"total_tokens":28,"cached_tokens":3,"provider":"child-provider","model":"child-model","source":"child"}`)},
		{Type: domain.EventModelUsage, CreatedAt: 5, PayloadVersion: 1, Payload: []byte(`{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6,"source":"summary"}`)},
	}
	if _, err := b.Append(ctx, storage.Commit{RunID: "run-usage", Events: events}); err != nil {
		t.Fatalf("append usage fixture: %v", err)
	}
	rows, err := usageStore.ListModelUsage(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("usage rows = %+v, want exactly three", rows)
	}
	if rows[0].Provider != "first" || rows[0].Model != "main" || rows[0].Source != "main" || rows[0].CachedTokens != 2 {
		t.Fatalf("legacy/main attribution = %+v", rows[0])
	}
	if rows[1].Provider != "child-provider" || rows[1].Model != "child-model" || rows[1].Source != "child" || rows[1].CachedTokens != 3 {
		t.Fatalf("explicit child attribution = %+v", rows[1])
	}
	if rows[2].Provider != "" || rows[2].Model != "" || rows[2].Source != "summary" {
		t.Fatalf("ambiguous summary attribution did not fail closed: %+v", rows[2])
	}
	aggregated, err := sessionUsageStore.ListSessionModelUsage(ctx, "sess-usage")
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	for _, row := range aggregated {
		requests += row.RequestCount
	}
	if len(aggregated) != 3 || requests != 3 {
		t.Fatalf("session usage aggregates = %+v, want three routes/requests", aggregated)
	}
}

func assertSingleProjectedMessage(t *testing.T, b storage.Engine, ctx context.Context, want domain.Message) {
	t.Helper()
	got, err := b.ListMessages(ctx, want.SessionID)
	if err != nil {
		t.Fatalf("ListMessages(%s): %v", want.SessionID, err)
	}
	if len(got) != 1 {
		t.Fatalf("ListMessages(%s) = %d rows, want exactly 1: %+v", want.SessionID, len(got), got)
	}
	if !storage.SameProjectedMessage(got[0], want) {
		t.Fatalf("stored projection = %+v, want fields matching %+v", got[0], want)
	}
}
