package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestWorkExactInputRestartAndUnavailableHistory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "work-exact.db")
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	const sessionID domain.SessionID = "exact-work"
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	mutation := domain.WorkMutation{SessionID: sessionID, RequestID: "enter", RequestInput: `{"owner":"run-private","intent":"enter-plan"}`, Kind: domain.WorkEventPlanEntered}
	first, err := b.CommitWork(ctx, mutation)
	if err != nil {
		t.Fatal(err)
	}
	var payload []byte
	if err := b.db.QueryRowContext(ctx, "SELECT payload FROM session_work_events WHERE session_id = ?", sessionID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "run-private") || strings.Contains(string(payload), "RequestInput") {
		t.Fatalf("private receipt in event payload: %s", payload)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	replay, err := b.CommitWork(ctx, mutation)
	if err != nil || !replay.Replayed || replay.Event.Seq != first.Event.Seq {
		t.Fatalf("restart replay: %+v %v", replay, err)
	}
	changed := mutation
	changed.RequestInput = `{"owner":"run-foreign","intent":"enter-plan"}`
	if _, err := b.CommitWork(ctx, changed); !errors.Is(err, storage.ErrWorkRequestConflict) {
		t.Fatalf("changed owner replay: %v", err)
	}
	if _, err := b.db.ExecContext(ctx, "UPDATE session_work_events SET request_input = NULL WHERE session_id = ?", sessionID); err != nil {
		t.Fatal(err)
	}
	state, err := b.ReadWork(ctx, sessionID)
	if err != nil || !state.Plan.Active || state.Version != 1 {
		t.Fatalf("unbound history was lost: %+v %v", state, err)
	}
	if _, err := b.CommitWork(ctx, mutation); !errors.Is(err, storage.ErrWorkBindingUnavailable) {
		t.Fatalf("unknown input reused: %v", err)
	}
	events, _, err := b.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("unbound replay history: %v", err)
	}
	public, err := json.Marshal(events)
	if err != nil || strings.Contains(string(public), "run-private") {
		t.Fatalf("private replay leaked: %s %v", public, err)
	}
}

func TestChildExactInputReplayConflictAndUnavailableMailbox(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "child-exact.db")
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	parent := domain.SessionID("parent")
	parentRun := domain.RunID("parent-run")
	if err := b.CreateSession(ctx, domain.Session{ID: parent, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: parentRun, SessionID: parent, Status: domain.RunActive, RootID: parentRun, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	ceiling := domain.ChildAuthorityCeiling{PolicyProfile: domain.PolicyProfileDefault, PolicyDefinition: "policy-A", SandboxMode: domain.SandboxModeWorkspaceWrite, ApprovalPolicy: domain.ApprovalPolicyAsk, ToolNames: []string{"list_dir", "read_file"}}
	intended, err := domain.ChildRequestInput("create", "task-AA", []string{"read_file"})
	if err != nil {
		t.Fatal(err)
	}
	input := storage.ChildSessionAdmission{
		Session: domain.Session{ID: "child", CreatedAt: 3},
		Binding: domain.ChildSessionBinding{ChildSessionID: "child", OriginParentSessionID: parent, OriginParentRunID: parentRun, AuthorizerRunID: parentRun, InitialActivationRunID: "child-run", ActivationRunID: "child-run", OperationKey: "create", RequestInput: intended, AuthorityCeiling: ceiling, ActivationOperationKey: "create", ActivationRequestInput: intended, ActivationToolNames: []string{"read_file"}, State: domain.ChildSessionOpen},
		Admission: storage.RunAdmission{
			Message: domain.Message{ID: "child-message", SessionID: "child", RunID: "child-run", Role: domain.RoleUser, CreatedAt: 4, Content: "task-AA"},
			Run:     domain.Run{ID: "child-run", SessionID: "child", Status: domain.RunAccepted, CreatedAt: 4, Kind: domain.RunKindChild, ChildMode: domain.ChildModeContinuable, ParentID: parentRun, RootID: parentRun, Depth: 1},
			Started: domain.RunEvent{RunID: "child-run", Type: domain.EventRunStarted, PayloadVersion: 1, CreatedAt: 4, Payload: []byte(`{"provider":"fixture"}`)},
		},
	}
	first, err := b.CommitChildSessionAdmission(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*storage.ChildSessionAdmission){
		func(in *storage.ChildSessionAdmission) {
			in.Admission.Message.Content = "task-BB"
			value, _ := domain.ChildRequestInput("create", "task-BB", in.Binding.ActivationToolNames)
			in.Binding.RequestInput = value
			in.Binding.ActivationRequestInput = value
		},
		func(in *storage.ChildSessionAdmission) {
			in.Binding.ActivationToolNames = []string{"list_dir"}
			value, _ := domain.ChildRequestInput("create", "task-AA", in.Binding.ActivationToolNames)
			in.Binding.RequestInput = value
			in.Binding.ActivationRequestInput = value
		},
		func(in *storage.ChildSessionAdmission) { in.Binding.AuthorityCeiling.PolicyDefinition = "policy-B" },
		func(in *storage.ChildSessionAdmission) {
			in.Binding.OriginParentRunID = "other-run"
			in.Binding.AuthorizerRunID = "other-run"
			in.Admission.Run.ParentID = "other-run"
		},
	} {
		changed := input
		change(&changed)
		if _, err := b.CommitChildSessionAdmission(ctx, changed); !errors.Is(err, storage.ErrChildAdmissionConflict) {
			t.Fatalf("changed child retry: %v", err)
		}
	}
	message, created, err := b.EnqueueChildMessage(ctx, domain.ChildMailboxMessage{ID: "mail", ChildSessionID: "child", SenderSessionID: parent, RecipientSessionID: "child", IdempotencyKey: "mail-operation", Body: []byte("retained mail"), CreatedAt: 5})
	if err != nil || !created || message.Sequence != 1 {
		t.Fatalf("mail admission: %+v %v", message, err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	retry, err := b.CommitChildSessionAdmission(ctx, input)
	if err != nil || retry.Created || retry.Run.ID != first.Run.ID {
		t.Fatalf("child restart replay: %+v %v", retry, err)
	}
	if _, err := b.db.ExecContext(ctx, "UPDATE child_sessions SET request_input = NULL, activation_request_input = NULL WHERE child_session_id = 'child'"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CommitChildSessionAdmission(ctx, input); !errors.Is(err, storage.ErrChildBindingUnavailable) {
		t.Fatalf("unknown child input reused: %v", err)
	}
	binding, err := b.GetChildSessionBinding(ctx, "child")
	if err != nil || binding.State != domain.ChildSessionOpen || binding.NextMessageSequence != 2 {
		t.Fatalf("unbound child history: %+v %v", binding, err)
	}
	pending, err := b.ListPendingChildMessages(ctx, "child", "child", 0, 10)
	if err != nil || len(pending) != 1 || string(pending[0].Body) != "retained mail" {
		t.Fatalf("unbound mailbox lost: %+v %v", pending, err)
	}
}
