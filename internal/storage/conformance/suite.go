// Package conformance is the D-032 backend suite (CN-01..CN-32).
package conformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
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

// Run executes CN-01..CN-31.
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
		{"CN-28", "session work journal idempotence", cnSessionWork},
		{"CN-29", "atomic Goal round admission", cnAtomicGoalRun},
		{"CN-30", "atomic first primary run creates session", cnAtomicPrimaryRun},
		{"CN-31", "same-time message work anchor", cnMessageWorkAnchor},
		{"CN-32", "history work isolation and delete", cnHistoryWorkIsolationAndDelete},
	}
	if len(cases) != 32 {
		t.Fatalf("conformance suite must carry exactly 32 cases, got %d", len(cases))
	}
	for _, c := range cases {
		t.Run(c.id+" "+c.name, func(t *testing.T) { c.run(t, h) })
	}
}

func cnHistoryWorkIsolationAndDelete(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	const sourceID domain.SessionID = "sess-history-source"
	const childID domain.SessionID = "sess-history-child"
	if err := b.CreateSession(ctx, domain.Session{ID: sourceID, Title: "source", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession source: %v", err)
	}
	work, ok := b.(storage.WorkStore)
	if !ok {
		t.Fatal("backend does not implement WorkStore")
	}
	mutations, ok := b.(storage.HistoryMutationStore)
	if !ok {
		t.Fatal("backend does not implement HistoryMutationStore")
	}
	if _, err := work.CommitWork(ctx, domain.WorkMutation{
		SessionID: sourceID, ExpectedVersion: 0,
		RequestID: "history-enter-plan", RequestHash: "history-enter-plan",
		Kind: domain.WorkEventPlanEntered,
	}); err != nil {
		t.Fatalf("CommitWork enter Plan: %v", err)
	}
	if _, err := work.CommitWork(ctx, domain.WorkMutation{
		SessionID: sourceID, ExpectedVersion: 1,
		RequestID: "history-submit-plan", RequestHash: "history-submit-plan",
		Kind: domain.WorkEventPlanSubmitted, PlanSubmissionID: "history-submission", PlanMarkdown: "preserve the evidence",
	}); err != nil {
		t.Fatalf("CommitWork submit Plan: %v", err)
	}
	if err := b.AppendMessage(ctx, domain.Message{
		ID: "msg-history-source", SessionID: sourceID, RunID: "run-history-source", Role: domain.RoleUser,
		CreatedAt: 2, Content: "source message",
	}); err != nil {
		t.Fatalf("AppendMessage source: %v", err)
	}
	beforeMessages, err := b.ListMessages(ctx, sourceID)
	if err != nil || len(beforeMessages) != 1 || beforeMessages[0].WorkSeq != 2 {
		t.Fatalf("source before fork = %+v, %v; want one message anchored at 2", beforeMessages, err)
	}
	beforeWork, err := work.ReadWork(ctx, sourceID)
	if err != nil || beforeWork.Version != 2 || beforeWork.Plan.SubmissionID != "history-submission" {
		t.Fatalf("source work before fork = %+v, %v; want submitted Plan at version 2", beforeWork, err)
	}

	copy := beforeMessages[0]
	copy.ID = "msg-history-child"
	copy.SessionID = childID
	// A caller-provided source anchor must never become authority in the child.
	copy.WorkSeq = beforeMessages[0].WorkSeq
	markers := []storage.SessionTruncation{{
		SessionID: sourceID, CutoffMessageID: beforeMessages[0].ID, TailMessageID: beforeMessages[0].ID,
		WorkSeq: beforeMessages[0].WorkSeq, Reason: storage.TruncationFork, ForkSessionID: string(childID), CreatedAt: 3,
	}, {
		SessionID: childID, CutoffMessageID: copy.ID, TailMessageID: copy.ID,
		WorkSeq: beforeMessages[0].WorkSeq, Reason: storage.TruncationForkedFrom, ForkSessionID: string(sourceID), CreatedAt: 3,
	}}
	if _, err := mutations.CommitSessionFork(ctx,
		domain.Session{ID: childID, Title: "child", CreatedAt: 3},
		[]domain.Message{copy}, markers, nil); err != nil {
		t.Fatalf("CommitSessionFork: %v", err)
	}
	afterMessages, err := b.ListMessages(ctx, sourceID)
	if err != nil || len(afterMessages) != 1 || afterMessages[0].ID != beforeMessages[0].ID || afterMessages[0].WorkSeq != beforeMessages[0].WorkSeq {
		t.Fatalf("source messages after fork = %+v, %v; want unchanged %+v", afterMessages, err, beforeMessages)
	}
	afterWork, err := work.ReadWork(ctx, sourceID)
	if err != nil || afterWork.Version != beforeWork.Version || afterWork.Plan.SubmissionID != beforeWork.Plan.SubmissionID {
		t.Fatalf("source work after fork = %+v, %v; want unchanged %+v", afterWork, err, beforeWork)
	}
	childMessages, err := b.ListMessages(ctx, childID)
	if err != nil || len(childMessages) != 1 || childMessages[0].WorkSeq != 0 || childMessages[0].RunID != beforeMessages[0].RunID {
		t.Fatalf("child messages = %+v, %v; want copied row with WorkSeq 0 and source RunID %q", childMessages, err, beforeMessages[0].RunID)
	}
	parentMarker, ok, err := b.LatestSessionTruncation(ctx, sourceID)
	if err != nil || !ok || parentMarker.WorkSeq != beforeMessages[0].WorkSeq {
		t.Fatalf("parent fork marker = %+v, ok=%v, err=%v; want source WorkSeq %d", parentMarker, ok, err, beforeMessages[0].WorkSeq)
	}
	childMarker, ok, err := b.LatestSessionTruncation(ctx, childID)
	if err != nil || !ok || childMarker.WorkSeq != 0 {
		t.Fatalf("child fork marker = %+v, ok=%v, err=%v; want WorkSeq 0", childMarker, ok, err)
	}
	childWork, err := work.ReadWork(ctx, childID)
	if err != nil || childWork.Version != 0 || childWork.Plan.Active || childWork.Goal != nil {
		t.Fatalf("child work = %+v, %v; want no copied authority", childWork, err)
	}

	const roundSessionID domain.SessionID = "sess-history-rounds"
	if err := b.CreateSession(ctx, domain.Session{ID: roundSessionID, Title: "rounds", CreatedAt: 4}); err != nil {
		t.Fatalf("CreateSession rounds: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-history-rounds", Revision: 1}
	if _, err := work.CommitWork(ctx, domain.WorkMutation{
		SessionID: roundSessionID, ExpectedVersion: 0,
		RequestID: "history-create-goal", RequestHash: "history-create-goal",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "retain charged rounds", MaxRounds: 2,
	}); err != nil {
		t.Fatalf("CommitWork create Goal: %v", err)
	}
	if _, err := work.CommitWork(ctx, domain.WorkMutation{
		SessionID: roundSessionID, ExpectedVersion: 1,
		RequestID: "history-admit-round", RequestHash: "history-admit-round",
		Kind:      domain.WorkEventGoalRoundAdmitted,
		Admission: domain.GoalRunAdmission{SessionID: roundSessionID, Goal: ref, Round: 1, RunID: "run-history-round"},
	}); err != nil {
		t.Fatalf("CommitWork admit round: %v", err)
	}
	if err := b.AppendMessage(ctx, domain.Message{
		ID: "msg-history-round", SessionID: roundSessionID, Role: domain.RoleUser,
		CreatedAt: 5, Content: "rewind target",
	}); err != nil {
		t.Fatalf("AppendMessage round target: %v", err)
	}
	if _, err := mutations.CommitSessionRewind(ctx, storage.SessionTruncation{
		SessionID: roundSessionID, CutoffMessageID: "msg-history-round", TailMessageID: "msg-history-round",
		WorkSeq: 2, Reason: storage.TruncationRewind, CreatedAt: 6,
	}, domain.RunEvent{
		RunID: "run-history-rewind", Type: domain.EventSessionTruncated,
		CreatedAt: 6, PayloadVersion: 1, Payload: []byte(`{"session_id":"sess-history-rounds"}`),
	}); err != nil {
		t.Fatalf("CommitSessionRewind: %v", err)
	}
	roundWork, err := work.ReadWork(ctx, roundSessionID)
	if err != nil || roundWork.Version != 2 || roundWork.Goal == nil || roundWork.Goal.RoundsStarted != 1 {
		t.Fatalf("work after rewind = %+v, %v; want complete evidence and one charged round", roundWork, err)
	}

	if err := b.DeleteSession(ctx, sourceID); err != nil {
		t.Fatalf("DeleteSession source: %v", err)
	}
	deletedWork, err := work.ReadWork(ctx, sourceID)
	if !errors.Is(err, storage.ErrNotFound) || deletedWork.Version != 0 || deletedWork.Plan.SubmissionID != "" || deletedWork.Goal != nil {
		t.Fatalf("deleted session work = %+v, %v; want ErrNotFound with no work or submission evidence", deletedWork, err)
	}
}

func cnMessageWorkAnchor(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-history-anchor", Title: "history anchor", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	work, ok := b.(storage.WorkStore)
	if !ok {
		t.Fatal("backend does not implement WorkStore")
	}
	result, err := work.CommitWork(ctx, domain.WorkMutation{
		SessionID: "sess-history-anchor", ExpectedVersion: 0,
		RequestID: "enter-plan-anchor", RequestHash: "enter-plan-anchor",
		Kind: domain.WorkEventPlanEntered,
	})
	if err != nil {
		t.Fatalf("CommitWork enter Plan: %v", err)
	}
	result, err = work.CommitWork(ctx, domain.WorkMutation{
		SessionID: "sess-history-anchor", ExpectedVersion: 1,
		RequestID: "submit-plan-anchor", RequestHash: "submit-plan-anchor",
		Kind: domain.WorkEventPlanSubmitted, PlanSubmissionID: "submission-anchor", PlanMarkdown: "inspect and report",
	})
	if err != nil {
		t.Fatalf("CommitWork submit Plan: %v", err)
	}
	result, err = work.CommitWork(ctx, domain.WorkMutation{
		SessionID: "sess-history-anchor", ExpectedVersion: 2,
		RequestID: "decide-plan-anchor", RequestHash: "decide-plan-anchor",
		Kind: domain.WorkEventPlanDecided, PlanSubmissionID: "submission-anchor",
		PlanAction: domain.PlanDecisionRevise, PlanFeedback: "shorten the plan",
	})
	if err != nil {
		t.Fatalf("CommitWork decide Plan: %v", err)
	}
	message := domain.Message{
		ID: "msg-history-anchor", SessionID: "sess-history-anchor", Role: domain.RoleUser,
		CreatedAt: result.Event.CreatedAt, Content: "same millisecond as plan decision",
	}
	if err := b.AppendMessage(ctx, message); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	messages, err := b.ListMessages(ctx, message.SessionID)
	if err != nil || len(messages) != 1 || messages[0].CreatedAt != result.Event.CreatedAt || messages[0].WorkSeq != result.Event.Seq {
		t.Fatalf("same-time message = %+v, %v; want WorkSeq %d", messages, err, result.Event.Seq)
	}
	if err := b.RecordSessionTruncation(ctx, storage.SessionTruncation{
		SessionID: message.SessionID, CutoffMessageID: message.ID, TailMessageID: message.ID,
		WorkSeq: messages[0].WorkSeq, Reason: storage.TruncationRewind, CreatedAt: result.Event.CreatedAt,
	}); err != nil {
		t.Fatalf("RecordSessionTruncation: %v", err)
	}
	marker, ok, err := b.LatestSessionTruncation(ctx, message.SessionID)
	if err != nil || !ok || marker.WorkSeq != result.Event.Seq {
		t.Fatalf("truncation anchor = %+v / %v / %v, want WorkSeq %d", marker, ok, err, result.Event.Seq)
	}
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

func cnSessionWork(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	work, ok := b.(storage.WorkStore)
	if !ok {
		t.Fatal("backend does not implement WorkStore")
	}
	sessionID := domain.SessionID("sess-work")
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, Title: "work", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-1", Revision: 1}
	create := domain.WorkMutation{
		SessionID:       sessionID,
		ExpectedVersion: 0,
		RequestID:       "work-create-1",
		RequestHash:     "hash-create-1",
		Kind:            domain.WorkEventGoalCreated,
		Goal:            ref,
		Objective:       "ship it",
		MaxRounds:       2,
	}
	first, err := work.CommitWork(ctx, create)
	if err != nil {
		t.Fatalf("CommitWork create: %v", err)
	}
	if first.Event.Seq != 1 || first.State.Version != 1 || first.Replayed {
		t.Fatalf("first commit = %+v, want seq/version 1 and fresh", first)
	}
	retry, err := work.CommitWork(ctx, create)
	if err != nil {
		t.Fatalf("CommitWork identical retry: %v", err)
	}
	if !retry.Replayed || retry.Event.Seq != first.Event.Seq {
		t.Fatalf("identical retry = %+v, want original event", retry)
	}
	conflict := create
	conflict.RequestHash = "hash-create-other"
	if _, err := work.CommitWork(ctx, conflict); !errors.Is(err, storage.ErrWorkRequestConflict) {
		t.Fatalf("request hash divergence = %v, want ErrWorkRequestConflict", err)
	}
	admit := domain.WorkMutation{
		SessionID:       sessionID,
		ExpectedVersion: 1,
		RequestID:       "work-round-1",
		RequestHash:     "hash-round-1",
		Kind:            domain.WorkEventGoalRoundAdmitted,
		Admission: domain.GoalRunAdmission{
			SessionID: sessionID,
			Goal:      ref,
			Round:     1,
			RunID:     "run-goal-1",
		},
	}
	second, err := work.CommitWork(ctx, admit)
	if err != nil {
		t.Fatalf("CommitWork round: %v", err)
	}
	if second.Event.Seq != 2 || second.State.Goal == nil || second.State.Goal.RoundsStarted != 1 {
		t.Fatalf("round commit = %+v, want seq 2 and one spent round", second)
	}
	stale := admit
	stale.RequestID = "work-round-stale"
	stale.ExpectedVersion = 1
	if _, err := work.CommitWork(ctx, stale); !errors.Is(err, storage.ErrWorkVersionConflict) {
		t.Fatalf("stale round = %v, want ErrWorkVersionConflict", err)
	}
	state, err := work.ReadWork(ctx, sessionID)
	if err != nil {
		t.Fatalf("ReadWork: %v", err)
	}
	if state.Version != 2 || state.Goal == nil || state.Goal.RoundsStarted != 1 {
		t.Fatalf("ReadWork = %+v, want version 2/one round", state)
	}
	cursor := domain.WorkState{SessionID: sessionID}
	events, next, err := work.ReplayWork(ctx, sessionID, cursor, 1)
	if err != nil || len(events) != 1 || events[0].Seq != 1 || next.Version != 1 {
		t.Fatalf("ReplayWork first page = %+v / %+v, %v; want seq/version 1", events, next, err)
	}
	tail, next, err := work.ReplayWork(ctx, sessionID, next, 1)
	if err != nil || len(tail) != 1 || tail[0].Seq != 2 || next.Version != 2 || next.Goal == nil || next.Goal.RoundsStarted != 1 {
		t.Fatalf("ReplayWork tail = %+v / %+v, %v; want round event and folded state", tail, next, err)
	}
	empty, final, err := work.ReplayWork(ctx, sessionID, next, 1)
	if err != nil || len(empty) != 0 || final.Version != 2 {
		t.Fatalf("ReplayWork exhausted = %+v / %+v, %v; want stable version 2", empty, final, err)
	}
	edit := domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 2, RequestID: "work-edit-1", RequestHash: "hash-edit-1",
		Kind: domain.WorkEventGoalEdited, Goal: ref, Objective: "ship the revised result", MaxRounds: 3,
	}
	edited, err := work.CommitWork(ctx, edit)
	if err != nil || edited.State.Goal == nil || edited.State.Goal.Ref != (domain.GoalRef{ID: ref.ID, Revision: 2}) ||
		edited.State.Goal.RoundsStarted != 1 {
		t.Fatalf("CommitWork edit = %+v, %v; want revision 2 and retained spent round", edited, err)
	}
	page, replayed, err := work.ReplayWork(ctx, sessionID, final, 1)
	if err != nil || len(page) != 1 || page[0].Seq != 3 || replayed.Goal == nil ||
		replayed.Goal.Ref.Revision != 2 || replayed.Goal.RoundsStarted != 1 {
		t.Fatalf("ReplayWork edit page = %+v / %+v, %v; want folded revision 2", page, replayed, err)
	}
	loaded, err := work.ReadWork(ctx, sessionID)
	if err != nil || loaded.Version != 3 || loaded.Goal == nil || loaded.Goal.Ref.Revision != 2 ||
		loaded.Goal.Objective != "ship the revised result" || loaded.Goal.RoundsStarted != 1 {
		t.Fatalf("ReadWork after edit = %+v, %v; want durable revision 2 and spent round", loaded, err)
	}
}

func cnPromptAdmission(t *testing.T, runID domain.RunID, sessionID domain.SessionID) (storage.RunPromptSnapshot, storage.MaskCaptureCheck, []byte) {
	t.Helper()
	promptPayload, err := json.Marshal(storage.RunPromptPayload{Instruction: "storage conformance prompt"})
	if err != nil {
		t.Fatalf("marshal prompt payload: %v", err)
	}
	promptHash := sha256.Sum256(promptPayload)
	prompt := storage.RunPromptSnapshot{
		RunID: runID, SchemaVersion: 1, ComposerVersion: "mask-prompt/1",
		GenerationID: "conformance-generation", Payload: promptPayload,
		PayloadSHA256: hex.EncodeToString(promptHash[:]),
	}
	startedPayload, err := json.Marshal(struct {
		Provider     string `json:"provider"`
		Model        string `json:"model"`
		PromptSchema int    `json:"prompt_schema"`
		PromptDigest string `json:"prompt_digest"`
	}{"test", "test", prompt.SchemaVersion, prompt.PayloadSHA256})
	if err != nil {
		t.Fatalf("marshal run.started prompt marker: %v", err)
	}
	return prompt, storage.MaskCaptureCheck{SessionID: sessionID}, startedPayload
}

func cnAtomicGoalRun(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	work, ok := b.(storage.WorkStore)
	if !ok {
		t.Fatal("backend does not implement WorkStore")
	}
	goalRuns, ok := b.(storage.GoalRunStore)
	if !ok {
		t.Fatal("backend does not implement GoalRunStore")
	}
	sessionID := domain.SessionID("sess-goal-atomic")
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, Title: "goal", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-atomic", Revision: 1}
	if _, err := work.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "goal-create", RequestHash: "goal-create-hash",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "ship", MaxRounds: 2,
	}); err != nil {
		t.Fatalf("create Goal: %v", err)
	}
	admission := storage.GoalRunCommit{
		Mutation: domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: 1,
			RequestID: "goal-round-1", RequestHash: "goal-round-1-hash",
			Kind:      domain.WorkEventGoalRoundAdmitted,
			Admission: domain.GoalRunAdmission{SessionID: sessionID, Goal: ref, Round: 1, RunID: "run-goal-atomic"},
		},
		Message: domain.Message{
			ID: "msg-goal-atomic", SessionID: sessionID, RunID: "run-goal-atomic",
			Role: domain.RoleUser, CreatedAt: 2, Content: "continue",
		},
		Run: domain.Run{
			ID: "run-goal-atomic", SessionID: sessionID, Status: domain.RunActive,
			Kind: domain.RunKindPrimary, CreatedAt: 2,
		},
		Started: domain.RunEvent{
			RunID: "run-goal-atomic", Type: domain.EventRunStarted, CreatedAt: 2,
			PayloadVersion: 1, Payload: []byte("{\"provider\":\"test\",\"model\":\"test\"}"),
		},
	}
	prompt, expectedMask, startedPayload := cnPromptAdmission(t, admission.Run.ID, sessionID)
	admission.Prompt = &prompt
	admission.ExpectedMask = &expectedMask
	admission.Started.Payload = startedPayload
	bad := admission
	badRunID := domain.RunID("run-goal-mask-conflict")
	badPrompt, badExpectedMask, badStartedPayload := cnPromptAdmission(t, badRunID, sessionID)
	bad.Prompt = &badPrompt
	bad.ExpectedMask = &badExpectedMask
	bad.Started.Payload = badStartedPayload
	bad.ExpectedMask.SelectionRevision = 1
	bad.Mutation.RequestID = "goal-round-mask-conflict"
	bad.Mutation.RequestHash = "goal-round-mask-conflict-hash"
	bad.Mutation.Admission.RunID = badRunID
	bad.Message.ID = "msg-goal-mask-conflict"
	bad.Message.RunID = badRunID
	bad.Run.ID = badRunID
	bad.Started.RunID = badRunID
	if _, err := goalRuns.CommitGoalRun(ctx, bad); err == nil {
		t.Fatal("CommitGoalRun accepted a stale mask capture")
	} else {
		var maskErr *maskcontract.Error
		if !errors.As(err, &maskErr) || maskErr.Code != maskcontract.CodeRevisionConflict {
			t.Fatalf("stale mask capture error = %v, want revision conflict", err)
		}
	}
	state, err := work.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 1 || state.Goal == nil || state.Goal.RoundsStarted != 0 {
		t.Fatalf("stale mask capture changed work state = %+v, %v", state, err)
	}
	messages, err := b.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 0 {
		t.Fatalf("stale mask capture changed messages = %+v, %v", messages, err)
	}
	first, err := goalRuns.CommitGoalRun(ctx, admission)
	if err != nil {
		t.Fatalf("CommitGoalRun: %v", err)
	}
	if first.Work.Event.Seq != 2 || first.Work.State.Goal == nil || first.Work.State.Goal.RoundsStarted != 1 {
		t.Fatalf("first admission = %+v, want work seq 2 and one round", first)
	}
	promptStore, ok := b.(storage.RunAdmissionStore)
	if !ok {
		t.Fatal("backend does not implement RunAdmissionStore")
	}
	loadedPrompt, err := promptStore.LoadRunPrompt(ctx, admission.Run.ID)
	if err != nil || loadedPrompt.PayloadSHA256 != prompt.PayloadSHA256 || !bytes.Equal(loadedPrompt.Payload, prompt.Payload) {
		t.Fatalf("Goal prompt after admission = %+v, %v; want immutable admitted snapshot", loadedPrompt, err)
	}
	messages, err = b.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 1 || messages[0].ID != admission.Message.ID || messages[0].WorkSeq != 1 {
		t.Fatalf("messages after admission = %+v, %v; want one user row anchored at pre-admission version 1", messages, err)
	}
	runs, err := b.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].Status != domain.RunActive {
		t.Fatalf("runs after admission = %+v, %v; want one active run", runs, err)
	}
	started := replayAll(t, b, admission.Run.ID, 0)
	if len(started) != 1 || started[0].Type != domain.EventRunStarted || started[0].Seq != 1 {
		t.Fatalf("run journal after admission = %+v; want one run.started", started)
	}

	retry, err := goalRuns.CommitGoalRun(ctx, admission)
	if err != nil {
		t.Fatalf("CommitGoalRun retry: %v", err)
	}
	if !retry.Work.Replayed || retry.Run.ID != admission.Run.ID || retry.Started.Seq != 1 {
		t.Fatalf("retry = %+v, want original run and event", retry)
	}
	runs, err = b.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("retry duplicated run rows = %+v, %v", runs, err)
	}
	stale := admission
	stale.Mutation.RequestID = "goal-round-2"
	stale.Mutation.RequestHash = "goal-round-2-hash"
	stale.Mutation.ExpectedVersion = 2
	stale.Mutation.Admission.Round = 2
	stale.Mutation.Admission.RunID = "run-goal-atomic-2"
	stale.Message.ID = "msg-goal-atomic-2"
	stale.Message.RunID = stale.Mutation.Admission.RunID
	stale.Run.ID = stale.Mutation.Admission.RunID
	stale.Started.RunID = stale.Mutation.Admission.RunID
	stalePrompt := *admission.Prompt
	stalePrompt.RunID = stale.Mutation.Admission.RunID
	stale.Prompt = &stalePrompt
	if _, err := goalRuns.CommitGoalRun(ctx, stale); !errors.Is(err, storage.ErrWorkRunConflict) {
		t.Fatalf("active run admission = %v, want ErrWorkRunConflict", err)
	}
	state, err = work.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 2 || state.Goal == nil || state.Goal.RoundsStarted != 1 {
		t.Fatalf("failed admission changed work state = %+v, %v", state, err)
	}
	messages, err = b.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 1 {
		t.Fatalf("failed admission changed messages = %+v, %v", messages, err)
	}
}

func cnAtomicPrimaryRun(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	primaryRuns, ok := b.(storage.PrimaryRunStore)
	if !ok {
		t.Fatal("backend does not implement PrimaryRunStore")
	}
	const sessionID domain.SessionID = "sess-primary-first-run"
	const runID domain.RunID = "run-primary-first-run"
	prompt, expectedMask, startedPayload := cnPromptAdmission(t, runID, sessionID)
	started := domain.RunEvent{
		RunID: runID, Type: domain.EventRunStarted, CreatedAt: 2,
		PayloadVersion: 1, Payload: startedPayload,
	}
	badPromptRunID := domain.RunID("run-primary-mask-conflict")
	badPrompt, badExpectedMask, badStartedPayload := cnPromptAdmission(t, badPromptRunID, sessionID)
	bad := storage.PrimaryRunCommit{
		Message: domain.Message{ID: "msg-primary-mask-conflict", SessionID: sessionID, RunID: badPromptRunID, Role: domain.RoleUser, CreatedAt: 2, Content: "stale"},
		Run:     domain.Run{ID: badPromptRunID, SessionID: sessionID, Status: domain.RunActive, Kind: domain.RunKindPrimary, CreatedAt: 2},
		Started: domain.RunEvent{RunID: badPromptRunID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: badStartedPayload},
		Prompt:  &badPrompt, ExpectedMask: &badExpectedMask,
	}
	bad.ExpectedMask.SelectionRevision = 1
	if _, err := primaryRuns.CommitPrimaryRun(ctx, bad); err == nil {
		t.Fatal("CommitPrimaryRun accepted a stale mask capture")
	} else {
		var maskErr *maskcontract.Error
		if !errors.As(err, &maskErr) || maskErr.Code != maskcontract.CodeRevisionConflict {
			t.Fatalf("stale mask capture error = %v, want revision conflict", err)
		}
	}
	if _, err := b.GetSession(ctx, sessionID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("failed admission left a session row: %v", err)
	}
	if _, err := primaryRuns.CommitPrimaryRun(ctx, storage.PrimaryRunCommit{
		Message: domain.Message{
			ID: "msg-primary-first-run", SessionID: sessionID, RunID: runID,
			Role: domain.RoleUser, CreatedAt: 2, Content: "hello",
		},
		Run: domain.Run{
			ID: runID, SessionID: sessionID, Status: domain.RunActive,
			Kind: domain.RunKindPrimary, CreatedAt: 2,
		},
		Started: started,
		Prompt:  &prompt, ExpectedMask: &expectedMask,
	}); err != nil {
		t.Fatalf("CommitPrimaryRun: %v", err)
	}
	promptStore, ok := b.(storage.RunAdmissionStore)
	if !ok {
		t.Fatal("backend does not implement RunAdmissionStore")
	}
	loadedPrompt, err := promptStore.LoadRunPrompt(ctx, runID)
	if err != nil || loadedPrompt.PayloadSHA256 != prompt.PayloadSHA256 || !bytes.Equal(loadedPrompt.Payload, prompt.Payload) {
		t.Fatalf("primary prompt after admission = %+v, %v; want immutable admitted snapshot", loadedPrompt, err)
	}
	session, err := b.GetSession(ctx, sessionID)
	if err != nil || session.ID != sessionID || session.CreatedAt != 2 {
		t.Fatalf("session after first admission = %+v, %v; want committed default session", session, err)
	}
	if mode, policy := session.EffectiveSandbox(); mode != domain.SandboxModeWorkspaceWrite || policy != domain.ApprovalPolicyAsk {
		t.Fatalf("first-run session permissions = %s/%s, want product defaults", mode, policy)
	}
	messages, err := b.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 1 || messages[0].RunID != runID {
		t.Fatalf("messages after admission = %+v, %v; want one user message", messages, err)
	}
	run, err := b.GetRun(ctx, runID)
	if err != nil || run.Status != domain.RunActive {
		t.Fatalf("primary run after admission = %+v, %v; want active", run, err)
	}
	events := replayAll(t, b, runID, 0)
	if len(events) != 1 || events[0].Type != domain.EventRunStarted || events[0].Seq != 1 {
		t.Fatalf("primary journal after admission = %+v; want one run.started", events)
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
	if err := b.CreateRun(ctx, domain.Run{ID: "run-conflict", SessionID: "sess-pin", Status: domain.RunAccepted, CreatedAt: 3}); !errors.Is(err, storage.ErrWorkRunConflict) {
		t.Fatalf("second active primary CreateRun = %v, want ErrWorkRunConflict", err)
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
