package sqlite

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

const (
	ctScopeAInstance = "projectvivy/a2a-server:a2a:a2a"
	ctPrincipalOne   = "principal-1"
	ctPrincipalTwo   = "principal-2"
)

func channelTaskFixture(sessionID domain.SessionID, runID domain.RunID, messageID, remote string) storage.ChannelTaskCommit {
	return storage.ChannelTaskCommit{
		PrimaryRunCommit: storage.PrimaryRunCommit{
			Message: domain.Message{ID: messageID, SessionID: sessionID, RunID: runID, Role: domain.RoleUser, CreatedAt: 2, Content: "hello a2a"},
			Run:     domain.Run{ID: runID, SessionID: sessionID, Kind: domain.RunKindPrimary, Status: domain.RunActive, CreatedAt: 2},
			Started: domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)},
		},
		Scope:      domain.ChannelTaskScope{InstanceKey: ctScopeAInstance, PrincipalID: ctPrincipalOne},
		MessageID:  remote,
		InputHash:  sha256.Sum256([]byte("hello a2a")),
		NewSession: &domain.Session{ID: sessionID, CreatedAt: 2},
		Admitted: domain.RunEvent{
			RunID: runID, Type: domain.EventChannelTaskAdmitted, CreatedAt: 2, PayloadVersion: 1,
			Payload: []byte(`{"session_id":"` + string(sessionID) + `","run_id":"` + string(runID) + `","message_id":"` + remote + `"}`),
		},
	}
}

// TestChannelTaskAdmissionFaultMatrix pins the A2A-02.1 contract: one atomic
// transaction admits session+context+message+run+prompt+events+receipt or
// none at all, and the receipt — not the busy state — resolves retries.
func TestChannelTaskAdmissionFaultMatrix(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	t.Run("new admission commits every record together", func(t *testing.T) {
		commit := channelTaskFixture("sess_ct_a", "run_ct_a", "msg_ct_a", "remote-1")
		res, err := b.CommitChannelTask(ctx, commit)
		if err != nil {
			t.Fatalf("CommitChannelTask: %v", err)
		}
		if !res.NewlyCommitted {
			t.Fatal("first admission must report NewlyCommitted")
		}
		if len(res.Events) != 2 || res.Events[0].Type != domain.EventRunStarted || res.Events[1].Type != domain.EventChannelTaskAdmitted {
			t.Fatalf("events = %+v, want run.started then channel.task_admitted", res.Events)
		}
		if res.Events[1].Seq != res.Events[0].Seq+1 {
			t.Fatalf("admitted seq = %d, want immediately after started", res.Events[1].Seq)
		}
		if res.Receipt.MessageID != "remote-1" || res.Receipt.RunID != "run_ct_a" || res.Receipt.SessionID != "sess_ct_a" || res.Receipt.Operation != "submit" {
			t.Fatalf("receipt = %+v", res.Receipt)
		}
		if res.Receipt.AcceptedSeq != res.Events[1].Seq {
			t.Fatalf("accepted_seq = %d, want %d", res.Receipt.AcceptedSeq, res.Events[1].Seq)
		}
		if got, err := b.GetRun(ctx, "run_ct_a"); err != nil || got.SessionID != "sess_ct_a" {
			t.Fatalf("GetRun: %+v %v", got, err)
		}
		if msgs, err := b.ListMessages(ctx, "sess_ct_a"); err != nil || len(msgs) != 1 || msgs[0].Content != "hello a2a" {
			t.Fatalf("ListMessages: %+v %v", msgs, err)
		}
		receipt, found, err := b.FindChannelTaskReceipt(ctx, commit.Scope, "remote-1")
		if err != nil || !found || receipt.RunID != "run_ct_a" {
			t.Fatalf("FindChannelTaskReceipt = %+v,%v,%v", receipt, found, err)
		}
		owner, err := b.GetChannelTaskOwner(ctx, commit.Scope, "run_ct_a")
		if err != nil || owner != "sess_ct_a" {
			t.Fatalf("GetChannelTaskOwner = %q %v", owner, err)
		}
	})

	t.Run("identical retry returns original receipt", func(t *testing.T) {
		commit := channelTaskFixture("sess_ct_b", "run_ct_b", "msg_ct_b", "remote-2")
		if _, err := b.CommitChannelTask(ctx, commit); err != nil {
			t.Fatal(err)
		}
		// Same scope/message/hash but new generated IDs — the durable receipt
		// wins and the retried transaction commits nothing new.
		retry := channelTaskFixture("sess_ct_b2", "run_ct_b2", "msg_ct_b2", "remote-2")
		res, err := b.CommitChannelTask(ctx, retry)
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		if res.NewlyCommitted {
			t.Fatal("identical retry must not commit")
		}
		if res.Receipt.RunID != "run_ct_b" || res.Receipt.SessionID != "sess_ct_b" {
			t.Fatalf("receipt = %+v, want original identity", res.Receipt)
		}
		if _, err := b.GetRun(ctx, "run_ct_b2"); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("retry run must not exist: %v", err)
		}
		if _, err := b.GetSession(ctx, "sess_ct_b2"); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("retry session must not exist: %v", err)
		}
	})

	t.Run("same key different input hash is a conflict", func(t *testing.T) {
		if _, err := b.CommitChannelTask(ctx, channelTaskFixture("sess_ct_c", "run_ct_c", "msg_ct_c", "remote-3")); err != nil {
			t.Fatal(err)
		}
		changed := channelTaskFixture("sess_ct_c", "run_ct_c", "msg_ct_c", "remote-3")
		changed.InputHash = sha256.Sum256([]byte("different body"))
		changed.Message.Content = "different body"
		if _, err := b.CommitChannelTask(ctx, changed); !errors.Is(err, storage.ErrConflict) {
			t.Fatalf("changed hash = %v, want ErrConflict", err)
		}
	})

	t.Run("foreign scope sees not-found, never ownership detail", func(t *testing.T) {
		foreign := domain.ChannelTaskScope{InstanceKey: ctScopeAInstance, PrincipalID: ctPrincipalTwo}
		if _, found, err := b.FindChannelTaskReceipt(ctx, foreign, "remote-1"); err != nil || found {
			t.Fatalf("foreign receipt = found=%v err=%v", found, err)
		}
		if _, err := b.GetChannelTaskOwner(ctx, foreign, "run_ct_a"); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("foreign owner = %v, want ErrNotFound", err)
		}
		// A foreign scope cannot adopt an owned context.
		adopt := channelTaskFixture("sess_ct_a", "run_ct_x", "msg_ct_x", "remote-x")
		adopt.Scope = foreign
		adopt.NewSession = nil
		if _, err := b.CommitChannelTask(ctx, adopt); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("foreign adoption = %v, want ErrNotFound", err)
		}
	})

	t.Run("supplied context must already be owned", func(t *testing.T) {
		// Session exists natively but has no channel ownership row.
		if err := b.CreateSession(ctx, domain.Session{ID: "sess_native", CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
		commit := channelTaskFixture("sess_native", "run_ct_d", "msg_ct_d", "remote-4")
		commit.NewSession = nil
		if _, err := b.CommitChannelTask(ctx, commit); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("unowned context = %v, want ErrNotFound", err)
		}
		// A candidate session that already exists is a conflict, never an adoption.
		taken := channelTaskFixture("sess_native", "run_ct_d2", "msg_ct_d2", "remote-5")
		if _, err := b.CommitChannelTask(ctx, taken); !errors.Is(err, storage.ErrConflict) {
			t.Fatalf("taken candidate session = %v, want ErrConflict", err)
		}
	})

	t.Run("distinct message while context busy is a run conflict", func(t *testing.T) {
		if _, err := b.CommitChannelTask(ctx, channelTaskFixture("sess_ct_e", "run_ct_e", "msg_ct_e", "remote-6")); err != nil {
			t.Fatal(err)
		}
		// run_ct_e stays active: a second distinct message on the same owned
		// context collides with the one-primary-run gate.
		busy := channelTaskFixture("sess_ct_e", "run_ct_e2", "msg_ct_e2", "remote-7")
		busy.NewSession = nil
		if _, err := b.CommitChannelTask(ctx, busy); !errors.Is(err, storage.ErrWorkRunConflict) {
			t.Fatalf("busy context second submit = %v, want ErrWorkRunConflict", err)
		}
	})

	t.Run("failed commit leaves no core rows", func(t *testing.T) {
		// Force a late-stage failure: the admitted event names a different run,
		// so the receipt insert cannot reference a committed run.
		bad := channelTaskFixture("sess_ct_f", "run_ct_f", "msg_ct_f", "remote-8")
		bad.Admitted.RunID = "run_other"
		bad.Admitted.Payload = []byte(`{}`)
		if _, err := b.CommitChannelTask(ctx, bad); err == nil {
			t.Fatal("inconsistent admission must fail")
		}
		for _, probe := range []struct{ kind, id string }{
			{"session", "sess_ct_f"}, {"run", "run_ct_f"},
		} {
			var n int
			if err := b.db.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM "+probe.kind+"s WHERE id = ?", probe.id).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Fatalf("orphan %s row after failed commit", probe.kind)
			}
		}
		var n int
		if err := b.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM channel_task_receipts WHERE message_id = 'remote-8'").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatal("receipt survives a failed commit")
		}
		if err := b.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM channel_task_contexts WHERE session_id = 'sess_ct_f'").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatal("context ownership survives a failed commit")
		}
	})

	t.Run("list returns owned submits with a stable cursor", func(t *testing.T) {
		page, err := b.ListChannelTaskRuns(ctx, storage.ChannelTaskRunQuery{
			Scope: domain.ChannelTaskScope{InstanceKey: ctScopeAInstance, PrincipalID: ctPrincipalOne},
			Limit: 2,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Runs) == 0 || page.Runs[0].ID == "" {
			t.Fatalf("list = %+v", page)
		}
		for _, run := range page.Runs {
			if run.Kind != domain.RunKindPrimary {
				t.Fatalf("non-primary run listed: %+v", run)
			}
		}
		if page.HasMore {
			next, err := b.ListChannelTaskRuns(ctx, storage.ChannelTaskRunQuery{
				Scope:      domain.ChannelTaskScope{InstanceKey: ctScopeAInstance, PrincipalID: ctPrincipalOne},
				AfterRunID: page.NextRunID, Limit: 2,
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, run := range next.Runs {
				for _, prev := range page.Runs {
					if run.ID == prev.ID {
						t.Fatalf("cursor overlap: %s listed twice", run.ID)
					}
				}
			}
		}
		foreign, err := b.ListChannelTaskRuns(ctx, storage.ChannelTaskRunQuery{
			Scope: domain.ChannelTaskScope{InstanceKey: ctScopeAInstance, PrincipalID: ctPrincipalTwo}, Limit: 10,
		})
		if err != nil || len(foreign.Runs) != 0 {
			t.Fatalf("foreign list = %+v %v, want empty", foreign, err)
		}
	})
}

// TestChannelTaskAdmissionTwoHandles: a second backend handle racing the same
// receipt key commits exactly once.
func TestChannelTaskAdmissionTwoHandles(t *testing.T) {
	path := t.TempDir() + "/ct.db"
	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	ctx := context.Background()

	commit := channelTaskFixture("sess_ct_g", "run_ct_g", "msg_ct_g", "remote-9")
	res, err := first.CommitChannelTask(ctx, commit)
	if err != nil || !res.NewlyCommitted {
		t.Fatalf("first handle commit = %+v, %v", res, err)
	}
	retry := channelTaskFixture("sess_ct_g", "run_ct_g2", "msg_ct_g2", "remote-9")
	retry.NewSession = nil // winner owns it; the loser's candidate never landed
	res2, err := second.CommitChannelTask(ctx, retry)
	if err != nil {
		t.Fatalf("second handle retry: %v", err)
	}
	if res2.NewlyCommitted || res2.Receipt.RunID != "run_ct_g" {
		t.Fatalf("second handle = %+v, want winner receipt", res2)
	}
}

// TestChannelTaskOwnershipAndTombstone pins the .3 contract: scopes stay
// independent under the same remote message id, listing only sees owned
// primary submissions, deletion leaves tombstones that survive Session/Run
// removal, and a receipt whose run vanished without a tombstone is
// corruption, never resubmission.
func TestChannelTaskOwnershipAndTombstone(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	p1 := domain.ChannelTaskScope{InstanceKey: ctScopeAInstance, PrincipalID: ctPrincipalOne}
	p2 := domain.ChannelTaskScope{InstanceKey: ctScopeAInstance, PrincipalID: ctPrincipalTwo}

	// Same remote message id under two principals commits independently.
	a := channelTaskFixture("sess_own_a", "run_own_a", "msg_own_a", "shared-msg")
	if _, err := b.CommitChannelTask(ctx, a); err != nil {
		t.Fatal(err)
	}
	bc := channelTaskFixture("sess_own_b", "run_own_b", "msg_own_b", "shared-msg")
	bc.Scope = p2
	if _, err := b.CommitChannelTask(ctx, bc); err != nil {
		t.Fatal(err)
	}
	r1, found1, _ := b.FindChannelTaskReceipt(ctx, p1, "shared-msg")
	r2, found2, _ := b.FindChannelTaskReceipt(ctx, p2, "shared-msg")
	if !found1 || !found2 || r1.SessionID == r2.SessionID {
		t.Fatalf("scoped receipts = %+v / %+v", r1, r2)
	}

	// Local and child runs never appear in the owned-submission list.
	if err := b.CreateRun(ctx, domain.Run{
		ID: "run_local", SessionID: "sess_own_a", Status: domain.RunCompleted,
		Kind: domain.RunKindPrimary, CreatedAt: 5,
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{
		ID: "run_child", SessionID: "sess_own_a", Status: domain.RunCompleted,
		Kind: domain.RunKindChild, ParentID: "run_own_a", RootID: "run_own_a", CreatedAt: 6,
	}); err != nil {
		t.Fatal(err)
	}
	page, err := b.ListChannelTaskRuns(ctx, storage.ChannelTaskRunQuery{Scope: p1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range page.Runs {
		if run.ID != "run_own_a" {
			t.Fatalf("foreign/non-submit run listed: %s", run.ID)
		}
	}

	// Guessed ids under a foreign scope reveal nothing.
	if _, err := b.GetChannelTaskOwner(ctx, p2, "run_own_a"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("guessed run owner = %v, want ErrNotFound", err)
	}
	if _, err := b.GetChannelTaskOwner(ctx, p1, "run_nonexistent"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing run owner = %v, want ErrNotFound", err)
	}

	// Deletion tombstones the receipt and the context: the same remote
	// message can never recreate the deleted session.
	if err := b.DeleteSession(ctx, "sess_own_a"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetSession(ctx, "sess_own_a"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted session = %v, want ErrNotFound", err)
	}
	if _, found, err := b.FindChannelTaskReceipt(ctx, p1, "shared-msg"); err != nil || found {
		t.Fatalf("tombstoned receipt = found=%v err=%v", found, err)
	}
	if _, err := b.GetChannelTaskOwner(ctx, p1, "run_own_a"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("tombstoned owner = %v, want ErrNotFound", err)
	}
	// Resubmission mints a fresh candidate; the tombstoned session id stays dead.
	resub := channelTaskFixture("sess_own_a2", "run_own_a2", "msg_own_a2", "shared-msg")
	res, err := b.CommitChannelTask(ctx, resub)
	if err != nil {
		t.Fatalf("resubmit after delete: %v", err)
	}
	if !res.NewlyCommitted || res.Receipt.SessionID == "sess_own_a" {
		t.Fatalf("resubmit = %+v, want new session identity", res)
	}

	// A non-tombstoned receipt whose run row is missing is corruption.
	if _, err := b.CommitChannelTask(ctx, channelTaskFixture("sess_own_c", "run_own_c", "msg_own_c", "remote-c")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.ExecContext(ctx, `DELETE FROM runs WHERE id = 'run_own_c'`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ListChannelTaskRuns(ctx, storage.ChannelTaskRunQuery{Scope: p1, SessionID: "sess_own_c"}); err == nil {
		t.Fatal("missing run under live receipt must report corruption")
	}
}
