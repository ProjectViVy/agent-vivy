package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

func appendRewindFixture(t *testing.T, svc *Service, sessionID domain.SessionID) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.deps.Sessions.GetSession(ctx, sessionID); errors.Is(err, storage.ErrNotFound) {
		if err := svc.deps.Sessions.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	} else if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	messages := []domain.Message{
		{ID: "msg-1", SessionID: sessionID, Role: domain.RoleUser, Content: "one", CreatedAt: 1},
		{ID: "msg-2", SessionID: sessionID, Role: domain.RoleAssistant, Content: "two", CreatedAt: 2},
		{ID: "msg-3", SessionID: sessionID, Role: domain.RoleUser, Content: "three", CreatedAt: 3},
		{ID: "msg-4", SessionID: sessionID, Role: domain.RoleAssistant, Content: "four", CreatedAt: 4},
	}
	for _, m := range messages {
		if err := svc.deps.Messages.AppendMessage(ctx, m); err != nil {
			t.Fatalf("AppendMessage %s: %v", m.ID, err)
		}
	}
}

func TestRewindSessionMarksAndFilters(t *testing.T) {
	svc, backend, sink := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	appendRewindFixture(t, svc, "sess-rw")

	result, err := svc.RewindSession(ctx, "sess-rw", "msg-3")
	if err != nil {
		t.Fatalf("RewindSession: %v", err)
	}
	if result.CutoffMessageID != "msg-3" || result.RemainingCount != 2 {
		t.Fatalf("RewindSession = %+v, want cutoff msg-3 with 2 remaining", result)
	}
	marker, ok, err := backend.LatestSessionTruncation(ctx, "sess-rw")
	if err != nil || !ok {
		t.Fatalf("LatestSessionTruncation = ok=%v, %v; want true, nil", ok, err)
	}
	if marker.CutoffMessageID != "msg-3" || marker.TailMessageID != "msg-4" || marker.Reason != storage.TruncationRewind {
		t.Fatalf("marker = %+v, want msg-3 rewind with tail msg-4", marker)
	}
	folded := mustEffectiveMessages(t, svc, ctx, "sess-rw")
	if len(folded) != 2 || folded[0].ID != "msg-1" || folded[1].ID != "msg-2" {
		t.Fatalf("effective view = %+v, want msg-1..msg-2", folded)
	}
	// The edit flow appends a fresh turn after the rewind: beyond the tail
	// anchor it must stay visible, or the retry would vanish forever.
	if err := svc.deps.Messages.AppendMessage(ctx, domain.Message{ID: "msg-5", SessionID: "sess-rw", Role: domain.RoleUser, Content: "five", CreatedAt: 5}); err != nil {
		t.Fatalf("AppendMessage post-rewind turn: %v", err)
	}
	retried := mustEffectiveMessages(t, svc, ctx, "sess-rw")
	if len(retried) != 3 || retried[0].ID != "msg-1" || retried[1].ID != "msg-2" || retried[2].ID != "msg-5" {
		t.Fatalf("post-rewind view = %+v, want msg-1..msg-2 plus the new turn", retried)
	}
	truncated := 0
	for _, ev := range sink.snapshot() {
		if ev.Type == domain.EventSessionTruncated {
			truncated++
			if !strings.HasPrefix(string(ev.RunID), "tr_") {
				t.Fatalf("session.truncated run id = %s, want tr_ prefix", ev.RunID)
			}
			if !strings.Contains(string(ev.Payload), `"cutoff_message_id":"msg-3"`) {
				t.Fatalf("session.truncated payload = %s, want cutoff msg-3", ev.Payload)
			}
		}
	}
	if truncated != 1 {
		t.Fatalf("published session.truncated events = %d, want 1", truncated)
	}
}

func TestRewindSessionValidation(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	appendRewindFixture(t, svc, "sess-rw")

	if _, err := svc.RewindSession(ctx, "sess-rw", "msg-gone"); err != ErrInvalidCutoff {
		t.Fatalf("unknown cutoff = %v, want ErrInvalidCutoff", err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: "run-busy", SessionID: "sess-rw", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateRun busy: %v", err)
	}
	if _, err := svc.RewindSession(ctx, "sess-rw", "msg-2"); err != ErrSessionBusy {
		t.Fatalf("busy session = %v, want ErrSessionBusy", err)
	}
	if err := backend.SetRunStatus(ctx, "run-busy", domain.RunCompleted); err != nil {
		t.Fatalf("SetRunStatus: %v", err)
	}
	// A queued run on another session must not block this one.
	if err := backend.CreateRun(ctx, domain.Run{ID: "run-other", SessionID: "sess-other", Status: domain.RunQueued, CreatedAt: 2}); err != nil {
		t.Fatalf("CreateRun other: %v", err)
	}
	if _, err := svc.RewindSession(ctx, "sess-rw", "msg-2"); err != nil {
		t.Fatalf("rewind with foreign active run = %v, want nil", err)
	}
}

func TestRewindSessionNotWired(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	svc.deps.Truncations = nil
	appendRewindFixture(t, svc, "sess-rw")
	if _, err := svc.RewindSession(context.Background(), "sess-rw", "msg-1"); err != ErrRewindNotWired {
		t.Fatalf("unwired rewind = %v, want ErrRewindNotWired", err)
	}
}

func TestForkSessionCopiesHistoryAndAnchors(t *testing.T) {
	svc, backend, sink := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	workspace := t.TempDir()
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-src", Title: "origin", CreatedAt: 1, SandboxMode: string(domain.SandboxModeReadOnly), ApprovalPolicy: string(domain.ApprovalPolicyNever), WorkspacePath: workspace}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	appendRewindFixture(t, svc, "sess-src")

	if err := backend.CreateRun(ctx, domain.Run{ID: "run-busy", SessionID: "sess-src", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if _, err := svc.ForkSession(ctx, "sess-src", "msg-2", ""); err != ErrSessionBusy {
		t.Fatalf("busy fork = %v, want ErrSessionBusy", err)
	}
	if err := backend.SetRunStatus(ctx, "run-busy", domain.RunCompleted); err != nil {
		t.Fatalf("SetRunStatus: %v", err)
	}

	result, err := svc.ForkSession(ctx, "sess-src", "msg-2", "")
	if err != nil {
		t.Fatalf("ForkSession: %v", err)
	}
	if !strings.HasPrefix(result.SessionID, "sess_") || result.ForkPointMessageID != "msg-2" || result.CopiedCount != 2 {
		t.Fatalf("fork result = %+v, want new sess_ id with msg-2 and 2 copied rows", result)
	}
	child, err := backend.GetSession(ctx, domain.SessionID(result.SessionID))
	if err != nil {
		t.Fatalf("GetSession child: %v", err)
	}
	if child.Title != "origin (fork)" || child.SandboxMode != string(domain.SandboxModeReadOnly) || child.ApprovalPolicy != string(domain.ApprovalPolicyNever) || child.WorkspacePath != workspace || child.UpdatedAt <= 0 {
		t.Fatalf("child session = %+v, want inherited knobs, workspace, and default fork title", child)
	}
	childMsgs := mustListMessages(t, svc, domain.SessionID(result.SessionID))
	if len(childMsgs) != 2 || childMsgs[0].Content != "one" || childMsgs[1].Content != "two" {
		t.Fatalf("child history = %+v, want copied history up to the fork point", childMsgs)
	}
	if childMsgs[0].ID == "msg-1" || childMsgs[1].ID == "msg-2" || !strings.HasPrefix(childMsgs[1].ID, "msg_") {
		t.Fatalf("child ids = %s/%s, want fresh globally-unique msg_ ids", childMsgs[0].ID, childMsgs[1].ID)
	}
	// Neither marker filters: the child keeps its copied view, the parent
	// keeps its full view.
	if childView := mustEffectiveMessages(t, svc, ctx, domain.SessionID(result.SessionID)); len(childView) != 2 {
		t.Fatalf("child view after forked-from marker = %d rows, want unfiltered", len(childView))
	}
	if parentView := mustEffectiveMessages(t, svc, ctx, "sess-src"); len(parentView) != 4 {
		t.Fatalf("parent view after fork marker = %d rows, want unfiltered", len(parentView))
	}
	parentMarker, ok, err := backend.LatestSessionTruncation(ctx, "sess-src")
	if err != nil || !ok || parentMarker.Reason != storage.TruncationFork || parentMarker.ForkSessionID != result.SessionID {
		t.Fatalf("parent marker = %+v, ok=%v, err=%v; want fork anchor naming the child", parentMarker, ok, err)
	}
	childMarker, ok, err := backend.LatestSessionTruncation(ctx, domain.SessionID(result.SessionID))
	if err != nil || !ok || childMarker.Reason != storage.TruncationForkedFrom || childMarker.ForkSessionID != "sess-src" || childMarker.CutoffMessageID != childMsgs[1].ID {
		t.Fatalf("child marker = %+v, ok=%v, err=%v; want forked-from anchor on the child's fork-point copy", childMarker, ok, err)
	}
	forked := 0
	for _, ev := range sink.snapshot() {
		if ev.Type == domain.EventSessionForked {
			forked++
			if !strings.Contains(string(ev.Payload), `"parent_session_id":"sess-src"`) {
				t.Fatalf("session.forked payload = %s, want parent sess-src", ev.Payload)
			}
		}
	}
	if forked != 1 {
		t.Fatalf("published session.forked events = %d, want 1", forked)
	}
	if _, err := svc.ForkSession(ctx, "sess-src", "msg-gone", ""); err != ErrInvalidCutoff {
		t.Fatalf("unknown fork point = %v, want ErrInvalidCutoff", err)
	}
}

func TestForkAndRewindKeepSpentGoalRoundsWithoutChildAuthority(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	const sessionID domain.SessionID = "sess-history-work"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "history", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	appendRewindFixture(t, svc, sessionID)
	svc.deps.Work = backend
	svc.deps.GoalRuns = backend
	ref := domain.GoalRef{ID: "goal-history", Revision: 1}
	created, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 0, RequestID: "create-history-goal", RequestHash: "create-history-goal",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "finish the history probe", MaxRounds: 2,
	})
	if err != nil {
		t.Fatalf("create Goal: %v", err)
	}

	for round := 1; round <= 2; round++ {
		runID := domain.RunID(fmt.Sprintf("run-history-%d", round))
		messageID := fmt.Sprintf("msg-history-round-%d", round)
		at := int64(4 + round)
		admission := storage.GoalRunCommit{
			Mutation: domain.WorkMutation{
				SessionID: sessionID, ExpectedVersion: domain.WorkVersion(round),
				RequestID: fmt.Sprintf("admit-history-%d", round), RequestHash: fmt.Sprintf("admit-history-%d", round),
				Kind:      domain.WorkEventGoalRoundAdmitted,
				Admission: domain.GoalRunAdmission{SessionID: sessionID, Goal: ref, Round: round, RunID: runID},
			},
			Message: domain.Message{ID: messageID, SessionID: sessionID, RunID: runID, Role: domain.RoleUser, CreatedAt: at, Content: fmt.Sprintf("continue round %d", round)},
			Run:     domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, Kind: domain.RunKindPrimary, CreatedAt: at},
			Started: domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, CreatedAt: at, PayloadVersion: 1, Payload: []byte(`{"provider":"test","model":"test"}`)},
		}
		admitted, err := backend.CommitGoalRun(ctx, admission)
		if err != nil {
			t.Fatalf("CommitGoalRun round %d: %v", round, err)
		}
		if admitted.Work.Event.Seq != domain.WorkSeq(round+1) || admitted.Run.ID != runID {
			t.Fatalf("round %d admission = %+v, want work seq %d", round, admitted, round+1)
		}
		if err := backend.SetRunStatus(ctx, runID, domain.RunCompleted); err != nil {
			t.Fatalf("complete fixture run %s: %v", runID, err)
		}
	}
	if err := backend.AppendMessage(ctx, domain.Message{ID: "msg-history-b", SessionID: sessionID, Role: domain.RoleUser, CreatedAt: 7, Content: "message B"}); err != nil {
		t.Fatalf("AppendMessage B: %v", err)
	}
	parentMessages, err := backend.ListMessages(ctx, sessionID)
	if err != nil || len(parentMessages) != 7 {
		t.Fatalf("parent history before fork = %d rows, %v; want 7", len(parentMessages), err)
	}
	if parentMessages[0].WorkSeq != 0 || parentMessages[4].WorkSeq != 1 || parentMessages[5].WorkSeq != 2 || parentMessages[6].WorkSeq != 3 {
		t.Fatalf("message anchors = [%d %d %d %d], want A=0, rounds=1/2, B=3", parentMessages[0].WorkSeq, parentMessages[4].WorkSeq, parentMessages[5].WorkSeq, parentMessages[6].WorkSeq)
	}

	fork, err := svc.ForkSession(ctx, sessionID, "msg-history-b", "history branch")
	if err != nil {
		t.Fatalf("ForkSession through B: %v", err)
	}
	childID := domain.SessionID(fork.SessionID)
	childMessages, err := backend.ListMessages(ctx, childID)
	if err != nil || len(childMessages) != 7 {
		t.Fatalf("child history = %d rows, %v; want copied prefix", len(childMessages), err)
	}
	for i, message := range childMessages {
		if message.WorkSeq != 0 {
			t.Fatalf("child message %d WorkSeq = %d, want zero in a child with no work events", i, message.WorkSeq)
		}
	}
	childWork, err := backend.ReadWork(ctx, childID)
	if err != nil || childWork.Version != 0 || childWork.Goal != nil || childWork.Plan.Active {
		t.Fatalf("child work state = %+v / %v, want no inherited authority", childWork, err)
	}
	parentMarker, ok, err := backend.LatestSessionTruncation(ctx, sessionID)
	if err != nil || !ok || parentMarker.WorkSeq != 3 {
		t.Fatalf("parent fork marker = %+v / %v / %v, want source WorkSeq 3", parentMarker, ok, err)
	}
	childMarker, ok, err := backend.LatestSessionTruncation(ctx, childID)
	if err != nil || !ok || childMarker.WorkSeq != 0 {
		t.Fatalf("child fork marker = %+v / %v / %v, want WorkSeq 0", childMarker, ok, err)
	}

	if _, err := svc.RewindSession(ctx, sessionID, "msg-1"); err != nil {
		t.Fatalf("RewindSession to A: %v", err)
	}
	parentWork, err := backend.ReadWork(ctx, sessionID)
	if err != nil || parentWork.Version != 3 || parentWork.Goal == nil || parentWork.Goal.RoundsStarted != 2 {
		t.Fatalf("parent work after rewind = %+v / %v; want two spent rounds preserved", parentWork, err)
	}
	events, err := backend.ReplayWork(ctx, sessionID, 0, 10)
	if err != nil || len(events) != 3 || events[2].Seq != 3 {
		t.Fatalf("work journal after rewind = %+v / %v; want all three events", events, err)
	}
	marker, ok, err := backend.LatestSessionTruncation(ctx, sessionID)
	if err != nil || !ok || marker.Reason != storage.TruncationRewind || marker.WorkSeq != 0 {
		t.Fatalf("rewind marker = %+v / %v / %v, want cutoff anchor 0", marker, ok, err)
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs after rewind = %d, %v; want no automatically admitted run", len(runs), err)
	}
	active, err := backend.ListActiveRuns(ctx)
	if err != nil || len(active) != 0 {
		t.Fatalf("active runs after rewind = %+v / %v; want none", active, err)
	}
	if created.Event.Seq != 1 {
		t.Fatalf("Goal creation seq = %d, want first work event", created.Event.Seq)
	}
}

// The e2e replay caught fork resurrecting rows a rewind had folded: the
// edit flow (rewind + fresh turn) then forked at the retry, and the child
// received the rejected turn too. Rewind/fork targets and fork copies must
// all resolve against the EFFECTIVE view, never the raw stored list.
func TestRewindAndForkRespectEffectiveView(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-ef", Title: "view", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	appendRewindFixture(t, svc, "sess-ef")

	if _, err := svc.RewindSession(ctx, "sess-ef", "msg-2"); err != nil {
		t.Fatalf("RewindSession msg-2: %v", err)
	}
	if _, err := svc.RewindSession(ctx, "sess-ef", "msg-3"); err != ErrInvalidCutoff {
		t.Fatalf("rewind behind the fold = %v, want ErrInvalidCutoff", err)
	}
	if _, err := svc.ForkSession(ctx, "sess-ef", "msg-4", ""); err != ErrInvalidCutoff {
		t.Fatalf("fork at a hidden message = %v, want ErrInvalidCutoff", err)
	}

	fork, err := svc.ForkSession(ctx, "sess-ef", "msg-1", "")
	if err != nil {
		t.Fatalf("ForkSession msg-1: %v", err)
	}
	if fork.CopiedCount != 1 {
		t.Fatalf("fork copied %d rows, want only the visible msg-1", fork.CopiedCount)
	}
	childMsgs := mustListMessages(t, svc, domain.SessionID(fork.SessionID))
	if len(childMsgs) != 1 || childMsgs[0].Content != "one" {
		t.Fatalf("child history = %+v, want the single visible row (no resurrection)", childMsgs)
	}
	// The fork anchor on the parent must not shadow the rewind marker:
	// folded rows stay folded after a fork.
	if view := mustEffectiveMessages(t, svc, ctx, "sess-ef"); len(view) != 1 || view[0].ID != "msg-1" {
		t.Fatalf("parent view after fork anchor = %+v, want the rewind still in force", view)
	}

	remaining, err := svc.RewindSession(ctx, "sess-ef", "msg-1")
	if err != nil {
		t.Fatalf("RewindSession msg-1: %v", err)
	}
	if remaining.RemainingCount != 0 {
		t.Fatalf("remaining = %d, want 0 (view-relative count)", remaining.RemainingCount)
	}
	if view := mustEffectiveMessages(t, svc, ctx, "sess-ef"); len(view) != 0 {
		t.Fatalf("parent view = %+v, want empty", view)
	}
}

// Successive rewinds accumulate: each discard removes its closed range
// from the view; rows appended between rewinds stay unless a later range
// captures them.
func TestSuccessiveRewindsAccumulate(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	appendRewindFixture(t, svc, "sess-ac")

	if _, err := svc.RewindSession(ctx, "sess-ac", "msg-2"); err != nil {
		t.Fatalf("RewindSession msg-2: %v", err)
	}
	if err := svc.deps.Messages.AppendMessage(ctx, domain.Message{ID: "msg-5", SessionID: "sess-ac", Role: domain.RoleUser, Content: "five", CreatedAt: 5}); err != nil {
		t.Fatalf("AppendMessage msg-5: %v", err)
	}
	remaining, err := svc.RewindSession(ctx, "sess-ac", "msg-5")
	if err != nil {
		t.Fatalf("RewindSession msg-5: %v", err)
	}
	if remaining.RemainingCount != 1 {
		t.Fatalf("remaining = %d, want 1 (only msg-1 left)", remaining.RemainingCount)
	}
	view := mustEffectiveMessages(t, svc, ctx, "sess-ac")
	if len(view) != 1 || view[0].ID != "msg-1" {
		t.Fatalf("view = %+v, want msg-1 (both discard ranges folded)", view)
	}
}

func TestEditSessionCommitsReplacementAndRunTogether(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	appendRewindFixture(t, svc, "sess-edit")
	runID, err := svc.EditSession(ctx, "sess-edit", "msg-2", "replacement", RunOptions{})
	if err != nil {
		t.Fatalf("EditSession: %v", err)
	}
	view := mustEffectiveMessages(t, svc, ctx, "sess-edit")
	if len(view) != 2 || view[0].ID != "msg-1" || view[1].Content != "replacement" || view[1].RunID != runID {
		t.Fatalf("edited view = %+v", view)
	}
	if _, err := svc.deps.Runs.GetRun(ctx, runID); err != nil {
		t.Fatalf("GetRun: %v", err)
	}
}

func mustListMessages(t *testing.T, svc *Service, sessionID domain.SessionID) []domain.Message {
	t.Helper()
	messages, err := svc.deps.Messages.ListMessages(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	return messages
}

func mustEffectiveMessages(t *testing.T, svc *Service, ctx context.Context, sessionID domain.SessionID) []domain.Message {
	t.Helper()
	messages, err := svc.effectiveSessionMessages(ctx, sessionID, mustListMessages(t, svc, sessionID))
	if err != nil {
		t.Fatalf("effectiveSessionMessages: %v", err)
	}
	return messages
}
