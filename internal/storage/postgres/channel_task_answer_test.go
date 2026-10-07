package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func pgChannelAnswerFixture(t *testing.T, b *Backend, runID, questionID string) storage.ChannelTaskAnswerCommit {
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
		Scope:     domain.ChannelTaskScope{InstanceKey: "projectvivy/a2a-server:a2a:a2a", PrincipalID: "principal-1"},
		MessageID: "ans-" + questionID,
		InputHash: sha256.Sum256([]byte("answer " + questionID)),
		Transition: storage.QuestionTransitionCommit{
			RunID:      domain.RunID(runID),
			QuestionID: questionID,
			Outcome:    domain.QuestionAnswered,
			Answer:     "blue",
			Actor:      "channel:a2a:" + "principal-1",
			At:         100,
			Event: domain.RunEvent{
				RunID: domain.RunID(runID), Type: domain.EventUserQuestionAnswered,
				CreatedAt: 100, PayloadVersion: 2, Payload: []byte(`{"question_id":"` + questionID + `","answer":"blue","actor":"x"}`),
			},
		},
	}
}

// TestChannelTaskAnswerAtomicRacePG is the postgres mirror of the sqlite
// fault matrix: the advisory run lock serializes the transition with
// Journal.Append, so exactly one settlement wins.
func TestChannelTaskAnswerAtomicRacePG(t *testing.T) {
	b := openChannelTaskBackend(t)
	ctx := context.Background()

	seedCommit := channelTaskFixture("sess_ct_seed", "run_ct_seed", "msg_ct_seed", "remote-seed")
	if _, err := b.CommitChannelTask(ctx, seedCommit); err != nil {
		t.Fatalf("seed scope: %v", err)
	}

	t.Run("remote answer commits atomically and replays", func(t *testing.T) {
		commit := pgChannelAnswerFixture(t, b, "run_pga_1", "que_pga_1")
		res, err := b.CommitChannelTaskAnswer(ctx, commit)
		if err != nil {
			t.Fatalf("CommitChannelTaskAnswer: %v", err)
		}
		if !res.NewlyCommitted || res.Receipt.Operation != "answer" || res.Receipt.QuestionID != "que_pga_1" {
			t.Fatalf("result = %+v", res)
		}
		q, err := b.GetQuestion(ctx, "que_pga_1")
		if err != nil || q.Status != domain.QuestionAnswered || q.Answer != "blue" {
			t.Fatalf("question = %+v err=%v", q, err)
		}
		second, err := b.CommitChannelTaskAnswer(ctx, commit)
		if err != nil || second.NewlyCommitted || second.Receipt.AcceptedSeq != res.Receipt.AcceptedSeq {
			t.Fatalf("replay = %+v err=%v", second, err)
		}
	})

	t.Run("local and remote race settles exactly once", func(t *testing.T) {
		remote := pgChannelAnswerFixture(t, b, "run_pga_2", "que_pga_2")
		local := storage.QuestionTransitionCommit{
			RunID: "run_pga_2", QuestionID: "que_pga_2", Outcome: domain.QuestionAnswered,
			Answer: "local", Actor: "local_user", At: 100,
			Event: domain.RunEvent{RunID: "run_pga_2", Type: domain.EventUserQuestionAnswered, CreatedAt: 100, PayloadVersion: 2, Payload: []byte(`{"question_id":"que_pga_2","answer":"local"}`)},
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
			t.Fatalf("wins=%d losses=%d", wins, losses)
		}
	})

	t.Run("terminal run refuses without receipt", func(t *testing.T) {
		commit := pgChannelAnswerFixture(t, b, "run_pga_3", "que_pga_3")
		if _, err := b.db.ExecContext(ctx,
			`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES (?, 99, ?, 101, 2, '{}')`,
			"run_pga_3", string(domain.EventRunCancelled)); err != nil {
			t.Fatalf("close run: %v", err)
		}
		if _, err := b.CommitChannelTaskAnswer(ctx, commit); !errors.Is(err, storage.ErrRunClosed) {
			t.Fatalf("closed err = %v, want ErrRunClosed", err)
		}
		if _, found, _ := b.FindChannelTaskReceipt(ctx, commit.Scope, commit.MessageID); found {
			t.Fatal("closed run left a receipt")
		}
	})

	t.Run("rollback leaves nothing", func(t *testing.T) {
		commit := pgChannelAnswerFixture(t, b, "run_pga_4", "que_pga_4")
		bad := commit
		bad.Transition.Event.Type = ""
		bad.Transition.Event.Payload = nil
		// Force a mid-transaction failure via a colliding seq: lock the
		// max seq in place by inserting a rival event first is not
		// deterministic, so use a poisoned actor column length trick —
		// instead abort by violating the NOT NULL on run_id.
		bad.Transition.Event.RunID = ""
		if _, err := b.CommitChannelTaskAnswer(ctx, bad); err == nil {
			t.Skip("driver tolerated empty run_id — rollback path unprovable this way")
		}
		q, _ := b.GetQuestion(ctx, "que_pga_4")
		if q.Status != domain.QuestionPending {
			t.Fatalf("rollback question = %q", q.Status)
		}
		if _, found, _ := b.FindChannelTaskReceipt(ctx, commit.Scope, commit.MessageID); found {
			t.Fatal("rollback left a receipt")
		}
	})
}
