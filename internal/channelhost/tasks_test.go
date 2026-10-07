package channelhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/port/channel"
)

var taskCtx = context.Background()

func taskScope() domain.ChannelTaskScope {
	p := TaskPrincipal{ModuleID: "a2a", ProviderID: "a2a-server", InstanceID: "inst-1", PrincipalID: "alice", AuthorizationRevision: 1}
	return p.scope()
}

func taskPrincipalCtx(rev int64) context.Context {
	return WithTaskPrincipal(taskCtx, TaskPrincipal{
		ModuleID: "a2a", ProviderID: "a2a-server", InstanceID: "inst-1",
		PrincipalID: "alice", AuthorizationRevision: rev,
	})
}

// taskFixture wires a taskHost over a real sqlite backend. submit feeds
// channel-task admissions; events appended through appendTaskEvent are the
// only journal content — the host reads committed events exclusively.
type taskFixture struct {
	b        *sqlite.Backend
	host     channel.TaskHost
	submits  []domain.ChannelTaskInput
	cancels  []domain.RunID
	cancelOK bool
	revision int64
}

func newTaskFixture(t *testing.T) *taskFixture {
	t.Helper()
	f := &taskFixture{b: openBackend(t), cancelOK: true, revision: 1}
	f.host = NewTaskHost(TaskDeps{
		Store:    f.b,
		Journal:  f.b,
		Messages: f.b,
		Submit: func(ctx context.Context, in domain.ChannelTaskInput) (domain.ChannelTaskReceipt, error) {
			f.submits = append(f.submits, in)
			return f.admitTask(ctx, in)
		},
		Cancel: func(runID domain.RunID) bool {
			f.cancels = append(f.cancels, runID)
			return f.cancelOK
		},
		Subscribe: func(runID domain.RunID) (<-chan domain.RunEvent, func()) {
			ch := make(chan domain.RunEvent)
			return ch, func() { close(ch) }
		},
		Authorize: func(ctx context.Context, p TaskPrincipal) bool {
			return p.PrincipalID == "alice" && p.AuthorizationRevision == f.revision
		},
		SafePrompts: true,
		ServiceInfo: channel.TaskServiceInfo{
			Name: "vivy", Streaming: true, InputContinuation: true,
			Skills: []channel.TaskSkill{{ID: "s1", Name: "one"}},
		},
	})
	if f.host == nil {
		t.Fatal("NewTaskHost returned nil for a complete dep pack")
	}
	return f
}

// admitTask performs the scoped receipt + primary-run admission the
// runtime's SubmitChannelTask produces, minus the model run.
func (f *taskFixture) admitTask(ctx context.Context, in domain.ChannelTaskInput) (domain.ChannelTaskReceipt, error) {
	scope := in.Scope
	if scope.InstanceKey == "" {
		scope = taskScope()
	}
	runID := in.RunID
	if runID == "" {
		runID = domain.RunID("run_" + in.MessageID)
	}
	sessionID := in.SessionID
	newSession := &domain.Session{ID: domain.SessionID("sess_" + in.MessageID), CreatedAt: 1}
	if sessionID == "" {
		sessionID = newSession.ID
	}
	msg := domain.Message{
		ID: "msg_" + in.MessageID, SessionID: sessionID, RunID: runID,
		Role: domain.RoleUser, CreatedAt: 1, Content: strings.Join(in.Parts, "\n\n"),
		ChannelMessageID: in.MessageID,
	}
	run := domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 1}
	started := domain.RunEvent{
		RunID: runID, Type: domain.EventRunStarted, CreatedAt: 1, PayloadVersion: 1,
		Payload: []byte(`{"no_context":false}`),
	}
	admitted := domain.RunEvent{
		RunID: runID, Type: domain.EventChannelTaskAdmitted, CreatedAt: 1, PayloadVersion: 1,
		Payload: []byte(`{"session_id":"` + string(sessionID) + `","run_id":"` + string(runID) + `","message_id":"` + in.MessageID + `"}`),
	}
	res, err := f.b.CommitChannelTask(ctx, storage.ChannelTaskCommit{
		Scope: scope, MessageID: in.MessageID, InputHash: [32]byte{1},
		PrimaryRunCommit: storage.PrimaryRunCommit{Message: msg, Run: run, Started: started},
		NewSession:       newSession,
		Admitted:         admitted,
	})
	if err != nil {
		return domain.ChannelTaskReceipt{}, err
	}
	return res.Receipt, nil
}

func (f *taskFixture) appendTaskEvent(t *testing.T, runID domain.RunID, typ domain.EventType, version int, payload string, ats ...int64) {
	t.Helper()
	at := int64(0)
	if len(ats) > 0 {
		at = ats[0]
	}
	if at == 0 {
		at = time.Now().UnixMilli()
	}
	_, err := f.b.Append(taskCtx, storage.Commit{
		RunID: runID,
		Events: []domain.RunEvent{{
			RunID: runID, Type: typ, CreatedAt: at, PayloadVersion: version, Payload: []byte(payload),
		}},
	})
	if err != nil {
		t.Fatalf("append %s: %v", typ, err)
	}
}

func taskErrCode(t *testing.T, err error) channel.TaskErrorCode {
	t.Helper()
	var te *channel.TaskError
	if err == nil || !errors.As(err, &te) {
		t.Fatalf("err = %v, want TaskError", err)
	}
	return te.Code
}

func TestTaskOperations(t *testing.T) {
	f := newTaskFixture(t)
	ctx := taskPrincipalCtx(1)

	t.Run("scoped submission commits then replays", func(t *testing.T) {
		ref, err := f.host.SubmitTask(ctx, channel.TaskRequest{
			MessageID: "m1", Parts: []channel.TaskTextPart{{Text: "hello"}},
		})
		if err != nil {
			t.Fatalf("submit: %v", err)
		}
		if ref.TaskID == "" || ref.ContextID == "" || ref.Replayed {
			t.Fatalf("ref = %+v", ref)
		}
		ref2, err := f.host.SubmitTask(ctx, channel.TaskRequest{
			MessageID: "m1", Parts: []channel.TaskTextPart{{Text: "hello"}},
		})
		if err != nil || ref2.TaskID != ref.TaskID {
			t.Fatalf("replay ref = %+v err=%v", ref2, err)
		}
	})

	t.Run("foreign task read denied; scoped read projects", func(t *testing.T) {
		ref, err := f.host.SubmitTask(ctx, channel.TaskRequest{
			MessageID: "m2", Parts: []channel.TaskTextPart{{Text: "hi"}},
		})
		if err != nil {
			t.Fatalf("submit: %v", err)
		}
		f.appendTaskEvent(t, domain.RunID(ref.TaskID), domain.EventModelDelta, 1, `{"delta":"secret tool args and paths"}`)

		// Another scope must not see it.
		other := WithTaskPrincipal(taskCtx, TaskPrincipal{
			ModuleID: "a2a", ProviderID: "a2a-server", InstanceID: "inst-2",
			PrincipalID: "alice", AuthorizationRevision: 1,
		})
		if _, err := f.host.GetTask(other, channel.TaskQuery{TaskID: ref.TaskID}); err == nil {
			t.Fatal("foreign scope read succeeded")
		}
		snap, err := f.host.GetTask(ctx, channel.TaskQuery{TaskID: ref.TaskID})
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if snap.Status.State != channel.TaskStateWorking {
			t.Fatalf("state = %s", snap.Status.State)
		}
	})

	t.Run("revoked authorization denies every operation", func(t *testing.T) {
		f.revision = 2 // config bumped; the rev-1 binding must stop working
		ref, err := f.host.SubmitTask(ctx, channel.TaskRequest{
			MessageID: "m3", Parts: []channel.TaskTextPart{{Text: "x"}},
		})
		if taskErrCode(t, err) != channel.TaskErrDenied {
			t.Fatalf("submit err = %v", err)
		}
		_ = ref
		// A fresh rev-2 binding works again.
		if _, err := f.host.SubmitTask(taskPrincipalCtx(2), channel.TaskRequest{
			MessageID: "m4", Parts: []channel.TaskTextPart{{Text: "x"}},
		}); err != nil {
			t.Fatalf("rev-2 submit: %v", err)
		}
	})
}

func TestTaskStateMap(t *testing.T) {
	f := newTaskFixture(t)
	ctx := taskPrincipalCtx(1)

	newTask := func(msgID string) string {
		ref, err := f.host.SubmitTask(ctx, channel.TaskRequest{
			MessageID: msgID, Parts: []channel.TaskTextPart{{Text: "go"}},
		})
		if err != nil {
			t.Fatalf("submit %s: %v", msgID, err)
		}
		return ref.TaskID
	}
	get := func(id string) channel.TaskSnapshot {
		snap, err := f.host.GetTask(ctx, channel.TaskQuery{TaskID: id})
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		return snap
	}

	t.Run("pending question projects INPUT_REQUIRED with safe prompt", func(t *testing.T) {
		id := newTask("q1")
		f.appendTaskEvent(t, domain.RunID(id), domain.EventUserQuestionRequired, 1,
			`{"question_id":"qu1","tool_call_id":"c1","prompt":"Pick a color?","expires_at":0,"resume_target":"t"}`)
		snap := get(id)
		if snap.Status.State != channel.TaskStateInputRequired {
			t.Fatalf("state = %s", snap.Status.State)
		}
		if snap.Status.Message == nil || snap.Status.Message.Parts[0].Text != "Pick a color?" {
			t.Fatalf("prompt = %+v", snap.Status.Message)
		}
	})

	t.Run("answered precedence clears input state", func(t *testing.T) {
		id := newTask("q2")
		f.appendTaskEvent(t, domain.RunID(id), domain.EventUserQuestionRequired, 1,
			`{"question_id":"qu2","tool_call_id":"c1","prompt":"?","expires_at":0,"resume_target":"t"}`)
		f.appendTaskEvent(t, domain.RunID(id), domain.EventUserQuestionAnswered, 2,
			`{"question_id":"qu2","answer":"blue","actor":"channel:a2a:alice"}`)
		if snap := get(id); snap.Status.State != channel.TaskStateWorking {
			t.Fatalf("state = %s", snap.Status.State)
		}
	})

	t.Run("pending approval projects AUTHORIZATION_REQUIRED without prompt", func(t *testing.T) {
		id := newTask("q3")
		f.appendTaskEvent(t, domain.RunID(id), domain.EventToolApprovalRequired, 1,
			`{"approval_id":"ap1","tool_call_id":"c1","tool_name":"shell.exec","args":{"cmd":"rm -rf /"},"expires_at":0,"face":"code"}`)
		snap := get(id)
		if snap.Status.State != channel.TaskStateAuthorizationRequired {
			t.Fatalf("state = %s", snap.Status.State)
		}
		if snap.Status.Message != nil && strings.Contains(snap.Status.Message.Parts[0].Text, "rm -rf") {
			t.Fatalf("approval args leaked: %+v", snap.Status.Message)
		}
		f.appendTaskEvent(t, domain.RunID(id), domain.EventToolApprovalDecided, 1,
			`{"approval_id":"ap1","decision":"approved"}`)
		if snap := get(id); snap.Status.State != channel.TaskStateWorking {
			t.Fatalf("decided state = %s", snap.Status.State)
		}
	})

	t.Run("terminal wins; history is user + agent text only", func(t *testing.T) {
		id := newTask("q4")
		f.appendTaskEvent(t, domain.RunID(id), domain.EventToolRequested, 1,
			`{"tool_call_id":"c1","tool_name":"shell.exec","args":{"cmd":"ls /secret/path"}}`)
		f.appendTaskEvent(t, domain.RunID(id), domain.EventToolFinished, 1,
			`{"tool_call_id":"c1","tool_name":"shell.exec","result":"secret-output","error":""}`)
		f.appendTaskEvent(t, domain.RunID(id), domain.EventModelDelta, 1, `{"delta":"all done"}`, 0)
		f.appendTaskEvent(t, domain.RunID(id), domain.EventModelCompleted, 2, modelCompletedV2("all done"), 0)
		f.appendTaskEvent(t, domain.RunID(id), domain.EventRunCompleted, 2, `{"outcome":"completed"}`, 0)
		snap := get(id)
		if snap.Status.State != channel.TaskStateCompleted {
			t.Fatalf("state = %s", snap.Status.State)
		}
		blob := ""
		for _, m := range snap.History {
			for _, p := range m.Parts {
				blob += p.Text
			}
		}
		if strings.Contains(blob, "secret-output") || strings.Contains(blob, "shell.exec") || strings.Contains(blob, "/secret/path") {
			t.Fatalf("unsafe content leaked into history: %q", blob)
		}
		if !strings.Contains(blob, "go") || !strings.Contains(blob, "all done") {
			t.Fatalf("history missing user/agent text: %q", blob)
		}
	})
}

func modelCompletedV2(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf(`{"byte_len":%d,"content_sha256":"%s"}`, len([]byte(text)), hex.EncodeToString(sum[:]))
}

func TestTaskListAndCancel(t *testing.T) {
	f := newTaskFixture(t)
	ctx := taskPrincipalCtx(1)

	newTask := func(msgID string) string {
		ref, err := f.host.SubmitTask(ctx, channel.TaskRequest{
			MessageID: msgID, Parts: []channel.TaskTextPart{{Text: "work"}},
		})
		if err != nil {
			t.Fatalf("submit %s: %v", msgID, err)
		}
		return ref.TaskID
	}

	t.Run("exact totals and paging over the authorized scope", func(t *testing.T) {
		ids := []string{newTask("l1"), newTask("l2"), newTask("l3")}
		// l3 completes; the rest stay working.
		f.appendTaskEvent(t, domain.RunID(ids[2]), domain.EventRunCompleted, 1, `{"outcome":"completed"}`, 100)

		page, err := f.host.ListTasks(ctx, channel.TaskListQuery{})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if page.TotalSize != 3 || len(page.Tasks) != 3 {
			t.Fatalf("page = %+v", page)
		}
		// Sorted by committed status time descending, RunID tie-breaker.
		for i := 1; i < len(page.Tasks); i++ {
			a, b := page.Tasks[i-1], page.Tasks[i]
			if a.Status.UpdatedAt.Before(b.Status.UpdatedAt) ||
				(a.Status.UpdatedAt.Equal(b.Status.UpdatedAt) && a.Ref.TaskID < b.Ref.TaskID) {
				t.Fatalf("not sorted: %v then %v", a.Ref.TaskID, b.Ref.TaskID)
			}
		}
		page2, err := f.host.ListTasks(ctx, channel.TaskListQuery{PageSize: 1})
		if err != nil || len(page2.Tasks) != 1 || page2.NextPageToken == "" {
			t.Fatalf("paged = %+v err=%v", page2, err)
		}
		next, err := f.host.ListTasks(ctx, channel.TaskListQuery{PageSize: 1, PageToken: page2.NextPageToken})
		if err != nil || len(next.Tasks) != 1 || next.Tasks[0].Ref.TaskID == page2.Tasks[0].Ref.TaskID {
			t.Fatalf("next page = %+v err=%v", next, err)
		}
		// Filtered lists see only matching state.
		done, err := f.host.ListTasks(ctx, channel.TaskListQuery{State: channel.TaskStateCompleted})
		if err != nil || done.TotalSize != 1 || done.Tasks[0].Status.State != channel.TaskStateCompleted {
			t.Fatalf("state filter = %+v err=%v", done, err)
		}
	})

	t.Run("tokens reject tamper, foreign scope and expiry", func(t *testing.T) {
		newTask("l4")
		page, err := f.host.ListTasks(ctx, channel.TaskListQuery{PageSize: 1})
		if err != nil || page.NextPageToken == "" {
			t.Fatalf("page: %v", err)
		}
		// Tampered signature.
		bad := page.NextPageToken[:len(page.NextPageToken)-2] + "zz"
		if _, err := f.host.ListTasks(ctx, channel.TaskListQuery{PageSize: 1, PageToken: bad}); taskErrCode(t, err) != channel.TaskErrCursorInvalid {
			t.Fatalf("tamper err = %v", err)
		}
		// Foreign scope: token was minted for this listing's binding.
		if _, err := f.host.ListTasks(ctx, channel.TaskListQuery{PageSize: 1, PageToken: page.NextPageToken, ContextID: "sess_other"}); taskErrCode(t, err) != channel.TaskErrCursorInvalid {
			t.Fatalf("foreign filter err = %v", err)
		}
		// Expired.
		nowSeconds = func() int64 { return unixSeconds() + taskListTokenTTL + 1 }
		defer func() { nowSeconds = unixSeconds }()
		if _, err := f.host.ListTasks(ctx, channel.TaskListQuery{PageSize: 1, PageToken: page.NextPageToken}); taskErrCode(t, err) != channel.TaskErrCursorInvalid {
			t.Fatalf("expired err = %v", err)
		}
	})

	t.Run("lazy cancel accepted on nonterminal; terminal reports committed state", func(t *testing.T) {
		id := newTask("c1")
		snap, err := f.host.CancelTask(ctx, channel.TaskQuery{TaskID: id})
		if err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if snap.Status.State == channel.TaskStateCanceled {
			t.Fatal("cancel reported intent, not committed state")
		}
		if len(f.cancels) != 1 || f.cancels[0] != domain.RunID(id) {
			t.Fatalf("cancel calls = %v", f.cancels)
		}
		// The committed cancellation lands as a journal event; a later
		// cancel is idempotent and reports the terminal projection.
		f.appendTaskEvent(t, domain.RunID(id), domain.EventRunCancelled, 1, `{"reason":"user"}`, 0)
		snap, err = f.host.CancelTask(ctx, channel.TaskQuery{TaskID: id})
		if err != nil || snap.Status.State != channel.TaskStateCanceled {
			t.Fatalf("terminal cancel = %+v err=%v", snap, err)
		}
		if len(f.cancels) != 1 {
			t.Fatalf("terminal cancel re-signalled: %v", f.cancels)
		}
		// A task the in-memory runner cannot signal is not_cancelable.
		f.cancelOK = false
		id2 := newTask("c2")
		if _, err := f.host.CancelTask(ctx, channel.TaskQuery{TaskID: id2}); taskErrCode(t, err) != channel.TaskErrNotCancelable {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("context_id mismatch and ordinary-answer conflicts", func(t *testing.T) {
		id := newTask("c3")
		if _, err := f.host.GetTask(ctx, channel.TaskQuery{TaskID: id, ContextID: "sess_other"}); taskErrCode(t, err) != channel.TaskErrConflict {
			t.Fatalf("ctx err = %v", err)
		}
		// Ordinary message onto a task with no pending input conflicts.
		if _, err := f.host.SubmitTask(ctx, channel.TaskRequest{
			TaskID: id, MessageID: "ans1", Parts: []channel.TaskTextPart{{Text: "answer"}},
		}); taskErrCode(t, err) != channel.TaskErrConflict {
			t.Fatalf("answer err = %v", err)
		}
	})

	t.Run("service info exposes flags only", func(t *testing.T) {
		h := NewTaskServiceInfoHost(TaskDeps{Authorize: func(ctx context.Context, p TaskPrincipal) bool { return true },
			ServiceInfo: channel.TaskServiceInfo{Name: "vivy", Streaming: true, InputContinuation: true}})
		info, err := h.TaskServiceInfo(ctx)
		if err != nil {
			t.Fatalf("info: %v", err)
		}
		if !info.Streaming || !info.InputContinuation || info.Name != "vivy" {
			t.Fatalf("info = %+v", info)
		}
	})
}
