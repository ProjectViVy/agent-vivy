package sqlite

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// channelAnswerFixture settles the owning run + pending question and builds
// the winning transition commit the runtime would supply.
func channelAnswerFixture(t *testing.T, b *Backend, runID, questionID string) storage.ChannelTaskAnswerCommit {
	t.Helper()
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: domain.SessionID("sess_q_" + questionID), Title: "q", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: domain.RunID(runID), SessionID: domain.SessionID("sess_q_" + questionID), Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := b.CreateQuestion(ctx, domain.Question{
		ID: questionID, RunID: domain.RunID(runID), ToolCallID: "call-1",
		Prompt: "pick", Status: domain.QuestionPending,
		ExpiresAt: 9999999999999, ResumeTarget: "resume-1",
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
	return storage.ChannelTaskAnswerCommit{
		Scope:     domain.ChannelTaskScope{InstanceKey: ctScopeAInstance, PrincipalID: ctPrincipalOne},
		MessageID: "ans-" + questionID,
		InputHash: sha256.Sum256([]byte("answer " + questionID)),
		Transition: storage.QuestionTransitionCommit{
			RunID:      domain.RunID(runID),
			QuestionID: questionID,
			Outcome:    domain.QuestionAnswered,
			Answer:     "blue",
			Actor:      "channel:a2a:" + ctPrincipalOne,
			At:         100,
			Event: domain.RunEvent{
				RunID: domain.RunID(runID), Type: domain.EventUserQuestionAnswered,
				CreatedAt: 100, PayloadVersion: 2, Payload: []byte(`{"question_id":"` + questionID + `","answer":"blue","actor":"x"}`),
			},
		},
	}
}

func seedChannelScope(t *testing.T, b *Backend) {
	t.Helper()
	commit := channelTaskFixture("sess_ct_seed", "run_ct_seed", "msg_ct_seed", "remote-seed")
	if _, err := b.CommitChannelTask(context.Background(), commit); err != nil {
		t.Fatalf("seed scope: %v", err)
	}
}

// TestChannelTaskAnswerAtomicRace is the A2A-03.1 fault matrix: whatever
// races the shared transition — local, remote, expiry or a terminal —
// exactly one transition wins, and everything the winner commits survives
// or nothing does.
func TestChannelTaskAnswerAtomicRace(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	seedChannelScope(t, b)

	t.Run("remote answer commits question event and receipt together", func(t *testing.T) {
		commit := channelAnswerFixture(t, b, "run_ca_1", "que_ca_1")
		res, err := b.CommitChannelTaskAnswer(ctx, commit)
		if err != nil {
			t.Fatalf("CommitChannelTaskAnswer: %v", err)
		}
		if !res.NewlyCommitted || res.Receipt.Operation != "answer" || res.Receipt.QuestionID != "que_ca_1" {
			t.Fatalf("result = %+v", res)
		}
		q, err := b.GetQuestion(ctx, "que_ca_1")
		if err != nil || q.Status != domain.QuestionAnswered || q.Answer != "blue" || q.Actor != "channel:a2a:"+ctPrincipalOne {
			t.Fatalf("question = %+v, err=%v", q, err)
		}
		if len(res.Events) != 1 || res.Events[0].Type != domain.EventUserQuestionAnswered || res.Events[0].PayloadVersion != 2 {
			t.Fatalf("events = %+v", res.Events)
		}
		receipt, found, err := b.FindChannelTaskReceipt(ctx, commit.Scope, commit.MessageID)
		if err != nil || !found || receipt.AcceptedSeq != res.Events[0].Seq {
			t.Fatalf("receipt = %+v found=%v err=%v", receipt, found, err)
		}
	})

	t.Run("identical retry replays the original receipt without a second transition", func(t *testing.T) {
		commit := channelAnswerFixture(t, b, "run_ca_2", "que_ca_2")
		first, err := b.CommitChannelTaskAnswer(ctx, commit)
		if err != nil {
			t.Fatalf("first: %v", err)
		}
		second, err := b.CommitChannelTaskAnswer(ctx, commit)
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		if second.NewlyCommitted || second.Receipt.AcceptedSeq != first.Receipt.AcceptedSeq {
			t.Fatalf("retry = %+v, want original receipt", second)
		}
		var events int
		if err := b.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM run_events WHERE run_id = ? AND type = ?`,
			"run_ca_2", string(domain.EventUserQuestionAnswered)).Scan(&events); err != nil || events != 1 {
			t.Fatalf("answered events = %d, err=%v; want exactly 1", events, err)
		}
	})

	t.Run("different body under same message id is conflict", func(t *testing.T) {
		commit := channelAnswerFixture(t, b, "run_ca_3", "que_ca_3")
		if _, err := b.CommitChannelTaskAnswer(ctx, commit); err != nil {
			t.Fatalf("first: %v", err)
		}
		commit.InputHash = sha256.Sum256([]byte("different"))
		if _, err := b.CommitChannelTaskAnswer(ctx, commit); !errors.Is(err, storage.ErrConflict) {
			t.Fatalf("conflict err = %v", err)
		}
	})

	t.Run("local and remote settlement race produces exactly one winner", func(t *testing.T) {
		remote := channelAnswerFixture(t, b, "run_ca_4", "que_ca_4")
		local := storage.QuestionTransitionCommit{
			RunID:      "run_ca_4",
			QuestionID: "que_ca_4",
			Outcome:    domain.QuestionAnswered,
			Answer:     "local green",
			Actor:      "local_user",
			At:         100,
			Event: domain.RunEvent{RunID: "run_ca_4", Type: domain.EventUserQuestionAnswered,
				CreatedAt: 100, PayloadVersion: 2, Payload: []byte(`{"question_id":"que_ca_4","answer":"local green"}`)},
		}
		var wg sync.WaitGroup
		results := make(chan error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); _, err := b.CommitChannelTaskAnswer(ctx, remote); results <- err }()
		go func() { defer wg.Done(); _, err := b.CommitQuestionTransition(ctx, local); results <- err }()
		wg.Wait()
		close(results)
		wins, losses := 0, 0
		for err := range results {
			if err == nil {
				wins++
			} else if errors.Is(err, storage.ErrConflict) || errors.Is(err, storage.ErrNotFound) {
				losses++
			} else {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		if wins != 1 || losses != 1 {
			t.Fatalf("wins=%d losses=%d, want exactly one winner", wins, losses)
		}
		q, _ := b.GetQuestion(ctx, "que_ca_4")
		if q.Status != domain.QuestionAnswered {
			t.Fatalf("question status = %q", q.Status)
		}
	})

	t.Run("a losing message on a settled question cannot reach a later question", func(t *testing.T) {
		// fresh message id names the ALREADY-SETTLED question id: CAS
		// rejects it, no receipt commits, and a re-fetched pending
		// question on the same run is untouched.
		first := channelAnswerFixture(t, b, "run_ca_5", "que_ca_5")
		if _, err := b.CommitChannelTaskAnswer(ctx, first); err != nil {
			t.Fatalf("first: %v", err)
		}
		if err := b.CreateQuestion(ctx, domain.Question{
			ID: "que_ca_5b", RunID: "run_ca_5", ToolCallID: "call-2",
			Prompt: "next", Status: domain.QuestionPending, ExpiresAt: 9999999999999,
		}); err != nil {
			t.Fatalf("second question: %v", err)
		}
		loser := first
		loser.MessageID = "ans-loser"
		loser.InputHash = sha256.Sum256([]byte("ans-loser"))
		if _, err := b.CommitChannelTaskAnswer(ctx, loser); !errors.Is(err, storage.ErrConflict) {
			t.Fatalf("loser err = %v, want ErrConflict", err)
		}
		if _, found, err := b.FindChannelTaskReceipt(ctx, loser.Scope, loser.MessageID); err != nil || found {
			t.Fatalf("loser receipt found=%v err=%v", found, err)
		}
		q, _ := b.GetQuestion(ctx, "que_ca_5b")
		if q.Status != domain.QuestionPending {
			t.Fatalf("later question status = %q, want pending", q.Status)
		}
	})

	t.Run("run mismatch and terminal run both refuse without a receipt", func(t *testing.T) {
		commit := channelAnswerFixture(t, b, "run_ca_6", "que_ca_6")
		mismatch := commit
		mismatch.Transition.RunID = "run_other"
		mismatch.MessageID = "ans-mm"
		if _, err := b.CommitChannelTaskAnswer(ctx, mismatch); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("mismatch err = %v, want ErrNotFound", err)
		}
		if _, found, _ := b.FindChannelTaskReceipt(ctx, commit.Scope, "ans-mm"); found {
			t.Fatal("mismatch left a receipt")
		}
		if _, err := b.db.ExecContext(ctx,
			`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES (?, 99, ?, 101, 2, '{}')`,
			"run_ca_6", string(domain.EventRunCancelled)); err != nil {
			t.Fatalf("close run: %v", err)
		}
		commit.MessageID = "ans-closed"
		if _, err := b.CommitChannelTaskAnswer(ctx, commit); !errors.Is(err, storage.ErrRunClosed) {
			t.Fatalf("closed err = %v, want ErrRunClosed", err)
		}
		q, _ := b.GetQuestion(ctx, "que_ca_6")
		if q.Status != domain.QuestionPending {
			t.Fatalf("question on closed run status = %q", q.Status)
		}
	})

	t.Run("expired question refuses settlement", func(t *testing.T) {
		commit := channelAnswerFixture(t, b, "run_ca_7", "que_ca_7")
		if _, err := b.db.ExecContext(ctx, `UPDATE questions SET expires_at = 50 WHERE id = ?`, "que_ca_7"); err != nil {
			t.Fatalf("expire fixture: %v", err)
		}
		if _, err := b.CommitChannelTaskAnswer(ctx, commit); !errors.Is(err, storage.ErrConflict) {
			t.Fatalf("expired err = %v, want ErrConflict", err)
		}
		if _, found, _ := b.FindChannelTaskReceipt(ctx, commit.Scope, commit.MessageID); found {
			t.Fatal("expired answer left a receipt")
		}
	})

	t.Run("unknown scope is not-found with no disclosure", func(t *testing.T) {
		commit := channelAnswerFixture(t, b, "run_ca_8", "que_ca_8")
		commit.Scope = domain.ChannelTaskScope{InstanceKey: "other/host", PrincipalID: "ghost"}
		if _, err := b.CommitChannelTaskAnswer(ctx, commit); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("unknown scope err = %v, want ErrNotFound", err)
		}
	})
}

// TestQuestionTransitionRollback proves a failing transaction leaves no
// receipt, no event and no settled question behind.
func TestQuestionTransitionRollback(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	seedChannelScope(t, b)

	commit := channelAnswerFixture(t, b, "run_rb_1", "que_rb_1")
	// Fail the event insert deterministically after the question CAS:
	// a trigger aborting run_events inserts on this run rolls the whole
	// transaction back.
	if _, err := b.db.ExecContext(ctx, `CREATE TRIGGER fail_rb_1 BEFORE INSERT ON run_events
		WHEN NEW.run_id = 'run_rb_1' BEGIN SELECT RAISE(ABORT, 'forced'); END`); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if _, err := b.CommitChannelTaskAnswer(ctx, commit); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := b.db.ExecContext(ctx, `DROP TRIGGER fail_rb_1`); err != nil {
		t.Fatalf("drop trigger: %v", err)
	}
	q, _ := b.GetQuestion(ctx, "que_rb_1")
	if q.Status != domain.QuestionPending {
		t.Fatalf("rollback question status = %q", q.Status)
	}
	if _, found, _ := b.FindChannelTaskReceipt(ctx, commit.Scope, commit.MessageID); found {
		t.Fatal("rollback left a receipt")
	}
	var events int
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM run_events WHERE run_id = ?`, "run_rb_1").Scan(&events); err != nil || events != 0 {
		t.Fatalf("events = %d err=%v, want 0", events, err)
	}
}

// TestQuestionTransitionLocal exercises the shared store path the local
// Review/Face caller uses.
func TestQuestionTransitionLocal(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess_lt", CreatedAt: 1}); err != nil {
		t.Fatalf("session: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run_lt", SessionID: "sess_lt", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := b.CreateQuestion(ctx, domain.Question{
		ID: "que_lt", RunID: "run_lt", Status: domain.QuestionPending, ExpiresAt: 9999999999999,
	}); err != nil {
		t.Fatalf("question: %v", err)
	}
	res, err := b.CommitQuestionTransition(ctx, storage.QuestionTransitionCommit{
		RunID: "run_lt", QuestionID: "que_lt", Outcome: domain.QuestionAnswered,
		Answer: "yes", Actor: "local_user", At: 7,
		Event: domain.RunEvent{RunID: "run_lt", Type: domain.EventUserQuestionAnswered, CreatedAt: 7, PayloadVersion: 2, Payload: []byte(`{"question_id":"que_lt","answer":"yes","actor":"local_user"}`)},
	})
	if err != nil || !res.Changed || len(res.Events) != 1 || res.Events[0].Seq != 1 {
		t.Fatalf("result = %+v err=%v", res, err)
	}
	if _, err := b.CommitQuestionTransition(ctx, storage.QuestionTransitionCommit{
		RunID: "run_lt", QuestionID: "que_lt", Outcome: domain.QuestionCancelled,
		Actor: "local_user", At: 8,
		Event: domain.RunEvent{RunID: "run_lt", Type: domain.EventUserQuestionCancelled, CreatedAt: 8, PayloadVersion: 1, Payload: []byte(`{}`)},
	}); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("second transition err = %v, want ErrConflict", err)
	}
}
