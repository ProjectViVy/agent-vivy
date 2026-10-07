package channelhost

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/channel"
)

// streamFixture is taskFixture with a real event bus so subscribe-before-
// snapshot, dedup, and catch-up all exercise the production shape.
type streamFixture struct {
	*taskFixture
	bus *events.Bus
}

func newStreamFixture(t *testing.T) *streamFixture {
	t.Helper()
	f := newTaskFixture(t)
	fx := &streamFixture{taskFixture: f, bus: events.NewBus(64)}
	f.host = NewTaskHost(TaskDeps{
		Store:    f.b,
		Journal:  f.b,
		Messages: f.b,
		Submit: func(ctx context.Context, in domain.ChannelTaskInput) (domain.ChannelTaskReceipt, error) {
			f.submits = append(f.submits, in)
			return f.admitTask(ctx, in)
		},
		Cancel:    f.depsCancel(),
		Subscribe: fx.bus.Subscribe,
		Authorize: func(ctx context.Context, p TaskPrincipal) bool {
			return p.PrincipalID == "alice" && p.AuthorizationRevision == 1
		},
		SafePrompts: true,
	})
	if f.host == nil {
		t.Fatal("NewTaskHost nil")
	}
	return fx
}

func (f *taskFixture) depsCancel() func(domain.RunID) bool {
	return func(runID domain.RunID) bool {
		f.cancels = append(f.cancels, runID)
		return f.cancelOK
	}
}

// commitAndPublish appends the event to the journal and publishes it on the
// bus — the live-path shape (journal is truth, bus is wakeup).
func (f *streamFixture) commitAndPublish(t *testing.T, runID domain.RunID, typ domain.EventType, version int, payload string) {
	t.Helper()
	ev := domain.RunEvent{RunID: runID, Type: typ, CreatedAt: time.Now().UnixMilli(), PayloadVersion: version, Payload: []byte(payload)}
	if _, err := f.b.Append(taskCtx, storage.Commit{RunID: runID, Events: []domain.RunEvent{ev}}); err != nil {
		t.Fatalf("append %s: %v", typ, err)
	}
	it, err := f.b.Replay(taskCtx, runID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	var last domain.RunEvent
	for it.Next() {
		last = it.Value().Event
	}
	it.Close()
	f.bus.Publish(last)
}

func (f *streamFixture) newTask(t *testing.T, msgID string) string {
	t.Helper()
	ref, err := f.host.SubmitTask(taskPrincipalCtx(1), channel.TaskRequest{
		MessageID: msgID, Parts: []channel.TaskTextPart{{Text: "go"}},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	return ref.TaskID
}

func nextUpdate(t *testing.T, stream channel.TaskStream, timeout time.Duration) (channel.TaskUpdate, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(taskCtx, timeout)
	defer cancel()
	return stream.Next(ctx)
}

func TestTaskStreamReplayLive(t *testing.T) {
	f := newStreamFixture(t)
	ctx := taskPrincipalCtx(1)

	t.Run("snapshot first then live until terminal", func(t *testing.T) {
		id := f.newTask(t, "s1")
		stream, err := f.host.SubscribeTask(ctx, channel.TaskSubscription{TaskID: id})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer stream.Close()

		u, err := nextUpdate(t, stream, 3*time.Second)
		if err != nil || u.Snapshot == nil {
			t.Fatalf("first = %+v err=%v", u, err)
		}
		if u.Snapshot.Status.State != channel.TaskStateWorking || u.Cursor == "" {
			t.Fatalf("snapshot = %+v", u.Snapshot)
		}
		// Cursor is bound to this run.
		if cur, err := decodeTaskCursor(u.Cursor, domain.RunID(id), 1<<30); err != nil || string(cur.RunID) != id {
			t.Fatalf("cursor decode: %+v err=%v", cur, err)
		}

		f.commitAndPublish(t, domain.RunID(id), domain.EventModelDelta, 1, `{"delta":"hi"}`)
		f.commitAndPublish(t, domain.RunID(id), domain.EventModelCompleted, 2, modelCompletedV2("hi"))
		f.commitAndPublish(t, domain.RunID(id), domain.EventRunCompleted, 2, `{"outcome":"done"}`)

		var got []channel.TaskUpdate
		for {
			u, err := nextUpdate(t, stream, 3*time.Second)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("next: %v", err)
			}
			got = append(got, u)
		}
		var states []channel.TaskState
		var artifacts int
		for _, u := range got {
			if u.Status != nil {
				states = append(states, u.Status.State)
			}
			if u.Artifact != nil {
				artifacts++
				if u.Artifact.Parts[0].Text != "hi" {
					t.Fatalf("artifact = %+v", u.Artifact)
				}
			}
		}
		if artifacts != 1 {
			t.Fatalf("artifacts = %d, want 1", artifacts)
		}
		if len(states) == 0 || states[len(states)-1] != channel.TaskStateCompleted {
			t.Fatalf("states = %v", states)
		}
	})

	t.Run("input-required delivers then EOF; run stays open", func(t *testing.T) {
		id := f.newTask(t, "s2")
		stream, err := f.host.SubscribeTask(ctx, channel.TaskSubscription{TaskID: id})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer stream.Close()
		if _, err := nextUpdate(t, stream, 3*time.Second); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		f.commitAndPublish(t, domain.RunID(id), domain.EventUserQuestionRequired, 1,
			`{"question_id":"qu9","tool_call_id":"c1","prompt":"Pick?","expires_at":0,"resume_target":"t"}`)
		var sawInput bool
		for {
			u, err := nextUpdate(t, stream, 3*time.Second)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("next: %v", err)
			}
			if u.Status != nil && u.Status.State == channel.TaskStateInputRequired {
				sawInput = true
				if u.Status.Message == nil || u.Status.Message.Parts[0].Text != "Pick?" {
					t.Fatalf("prompt = %+v", u.Status.Message)
				}
			}
		}
		if !sawInput {
			t.Fatal("no input_required delivered")
		}
	})

	t.Run("cursor replays only the tail after it", func(t *testing.T) {
		id := f.newTask(t, "s3")
		f.commitAndPublish(t, domain.RunID(id), domain.EventModelDelta, 1, `{"delta":"a"}`)
		f.commitAndPublish(t, domain.RunID(id), domain.EventModelCompleted, 2, modelCompletedV2("a"))
		max, err := func() (domain.EventSeq, error) {
			var _ domain.EventSeq
			page, err := f.b.ReadJournalPage(taskCtx, storage.JournalPageQuery{RunID: domain.RunID(id), MaxEvents: 1, MaxBytes: storage.JournalPageMaxBytes})
			return page.ThroughSeq, err
		}()
		if err != nil {
			t.Fatalf("max: %v", err)
		}
		// Commit more, subscribe from a cursor pinned before them.
		f.commitAndPublish(t, domain.RunID(id), domain.EventRunCompleted, 2, `{"outcome":"done"}`)
		cur, _ := encodeTaskCursor(taskCursor{V: 1, RunID: domain.RunID(id), Seq: max})
		stream, err := f.host.SubscribeTask(ctx, channel.TaskSubscription{TaskID: id, After: cur})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer stream.Close()
		var first channel.TaskUpdate
		u, err := nextUpdate(t, stream, 3*time.Second)
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		first = u
		if first.Snapshot != nil {
			t.Fatal("cursor subscription emitted a snapshot")
		}
		if first.Status == nil || first.Status.State != channel.TaskStateCompleted {
			t.Fatalf("first tail update = %+v", first)
		}
	})

	t.Run("malformed, foreign and too-new cursors reject", func(t *testing.T) {
		id := f.newTask(t, "s4")
		for _, tok := range []string{"!!!", base64.RawURLEncoding.EncodeToString([]byte(`{"v":9}`))} {
			if _, err := f.host.SubscribeTask(ctx, channel.TaskSubscription{TaskID: id, After: tok}); taskErrCode(t, err) != channel.TaskErrCursorInvalid {
				t.Fatalf("malformed %q err=%v", tok, err)
			}
		}
		foreign, _ := encodeTaskCursor(taskCursor{V: 1, RunID: "run_other", Seq: 0})
		if _, err := f.host.SubscribeTask(ctx, channel.TaskSubscription{TaskID: id, After: foreign}); taskErrCode(t, err) != channel.TaskErrCursorInvalid {
			t.Fatalf("foreign err=%v", err)
		}
		toonew, _ := encodeTaskCursor(taskCursor{V: 1, RunID: domain.RunID(id), Seq: 1 << 30})
		if _, err := f.host.SubscribeTask(ctx, channel.TaskSubscription{TaskID: id, After: toonew}); taskErrCode(t, err) != channel.TaskErrCursorInvalid {
			t.Fatalf("too-new err=%v", err)
		}
	})

	t.Run("missed publish is recovered by the fallback tick", func(t *testing.T) {
		id := f.newTask(t, "s5")
		stream, err := f.host.SubscribeTask(ctx, channel.TaskSubscription{TaskID: id})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer stream.Close()
		if _, err := nextUpdate(t, stream, 3*time.Second); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		// Journal-committed but never published: only the tick can find it.
		f.appendTaskEvent(t, domain.RunID(id), domain.EventRunCompleted, 2, `{"outcome":"silent"}`, 0)
		deadline := time.After(4 * time.Second)
		for {
			u, err := nextUpdate(t, stream, 4*time.Second)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("next: %v", err)
			}
			if u.Status != nil && u.Status.State == channel.TaskStateCompleted {
				return
			}
			select {
			case <-deadline:
				t.Fatal("tick never delivered the silent commit")
			default:
			}
		}
		t.Fatal("EOF before terminal status")
	})
}
