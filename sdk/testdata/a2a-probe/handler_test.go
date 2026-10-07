package probe

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// TestA2ACustomHandlerOfficialClient is the acceptance fixture of record for
// issue #2's SDK seam: a custom a2asrv.RequestHandler mounted through the
// official NewJSONRPCHandler and driven by the official a2aclient.Client.
func TestA2ACustomHandlerOfficialClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("compile", func(t *testing.T) {
		// Static proof: the test handler satisfies all 11 RequestHandler methods.
		var _ interface {
			GetTask(context.Context, *a2a.GetTaskRequest) (*a2a.Task, error)
			ListTasks(context.Context, *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error)
			CancelTask(context.Context, *a2a.CancelTaskRequest) (*a2a.Task, error)
			SendMessage(context.Context, *a2a.SendMessageRequest) (a2a.SendMessageResult, error)
			SubscribeToTask(context.Context, *a2a.SubscribeToTaskRequest) iter.Seq2[a2a.Event, error]
			SendStreamingMessage(context.Context, *a2a.SendMessageRequest) iter.Seq2[a2a.Event, error]
			GetTaskPushConfig(context.Context, *a2a.GetTaskPushConfigRequest) (*a2a.PushConfig, error)
			ListTaskPushConfigs(context.Context, *a2a.ListTaskPushConfigRequest) (*a2a.ListTaskPushConfigResponse, error)
			CreateTaskPushConfig(context.Context, *a2a.PushConfig) (*a2a.PushConfig, error)
			DeleteTaskPushConfig(context.Context, *a2a.DeleteTaskPushConfigRequest) error
			GetExtendedAgentCard(context.Context, *a2a.GetExtendedAgentCardRequest) (*a2a.AgentCard, error)
		} = (*probeHandler)(nil)
	})

	t.Run("send", func(t *testing.T) {
		h := &probeHandler{
			sendResult: &a2a.Task{
				ID:        "task-probe-1",
				ContextID: "ctx-probe-1",
				Status:    a2a.TaskStatus{State: a2a.TaskStateSubmitted},
			},
		}
		client := newProbeClient(t, newProbeServer(t, h), nil)
		got, err := client.SendMessage(ctx, &a2a.SendMessageRequest{
			Config:  &a2a.SendMessageConfig{ReturnImmediately: true},
			Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("ping")),
		})
		if err != nil {
			t.Fatalf("SendMessage: %v", err)
		}
		task, ok := got.(*a2a.Task)
		if !ok {
			t.Fatalf("expected *a2a.Task, got %T", got)
		}
		if task.ID != "task-probe-1" || task.ContextID != "ctx-probe-1" {
			t.Fatalf("handler IDs not preserved: id=%q contextId=%q", task.ID, task.ContextID)
		}
		obs := h.last()
		if obs.returnImmediately != true {
			t.Fatalf("returnImmediately flag did not reach handler: %+v", obs)
		}
	})

	t.Run("stream", func(t *testing.T) {
		task := &a2a.Task{ID: "task-s", ContextID: "ctx-s", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}
		h := &probeHandler{streamScript: []streamItem{
			{event: task},
			{event: &a2a.TaskStatusUpdateEvent{TaskID: task.ID, ContextID: task.ContextID, Status: a2a.TaskStatus{State: a2a.TaskStateWorking}}},
			{event: a2a.NewArtifactEvent(task, a2a.NewTextPart("chunk-1"))},
			{event: &a2a.TaskStatusUpdateEvent{TaskID: task.ID, ContextID: task.ContextID, Status: a2a.TaskStatus{State: a2a.TaskStateCompleted}}},
		}}
		client := newProbeClient(t, newProbeServer(t, h), nil)
		var events []a2a.Event
		for ev, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
			Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("go")),
		}) {
			if err != nil {
				t.Fatalf("stream error: %v", err)
			}
			events = append(events, ev)
		}
		if len(events) != 4 {
			t.Fatalf("expected 4 events, got %d", len(events))
		}
		if _, ok := events[0].(*a2a.Task); !ok {
			t.Fatalf("first event must be a Task, got %T", events[0])
		}
		su, ok := events[1].(*a2a.TaskStatusUpdateEvent)
		if !ok || su.Status.State != a2a.TaskStateWorking {
			t.Fatalf("second event must be working status, got %T", events[1])
		}
		if _, ok := events[2].(*a2a.TaskArtifactUpdateEvent); !ok {
			t.Fatalf("third event must be artifact update, got %T", events[2])
		}
		su, ok = events[3].(*a2a.TaskStatusUpdateEvent)
		if !ok || su.Status.State != a2a.TaskStateCompleted {
			t.Fatalf("fourth event must be completed status, got %T", events[3])
		}
	})

	t.Run("interrupt", func(t *testing.T) {
		for _, state := range []a2a.TaskState{a2a.TaskStateInputRequired, a2a.TaskStateAuthRequired} {
			t.Run(string(state), func(t *testing.T) {
				task := &a2a.Task{ID: "task-i", ContextID: "ctx-i", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}
				h := &probeHandler{streamScript: []streamItem{
					{event: task},
					{event: &a2a.TaskStatusUpdateEvent{TaskID: task.ID, ContextID: task.ContextID, Status: a2a.TaskStatus{State: state}}},
				}}
				client := newProbeClient(t, newProbeServer(t, h), nil)
				sctx, scancel := context.WithTimeout(ctx, 5*time.Second)
				defer scancel()
				var events []a2a.Event
				var streamErr error
				for ev, err := range client.SendStreamingMessage(sctx, &a2a.SendMessageRequest{
					Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("go")),
				}) {
					if err != nil {
						streamErr = err
						break
					}
					events = append(events, ev)
				}
				if streamErr != nil {
					t.Fatalf("interrupt stream errored: %v", streamErr)
				}
				if len(events) != 2 {
					t.Fatalf("expected Task + status update then stream close, got %d events", len(events))
				}
				su, ok := events[1].(*a2a.TaskStatusUpdateEvent)
				if !ok || su.Status.State != state {
					t.Fatalf("expected %s update, got %T", state, events[1])
				}
			})
		}
	})

	t.Run("terminal", func(t *testing.T) {
		h := &probeHandler{
			storedTask: &a2a.Task{
				ID:        "task-t",
				ContextID: "ctx-t",
				Status:    a2a.TaskStatus{State: a2a.TaskStateCompleted},
			},
			subscribeErr: a2a.ErrUnsupportedOperation,
		}
		client := newProbeClient(t, newProbeServer(t, h), nil)
		for _, err := range client.SubscribeToTask(ctx, &a2a.SubscribeToTaskRequest{ID: "task-t"}) {
			if err == nil {
				t.Fatal("SubscribeToTask must surface the unsupported error")
			}
			if !errors.Is(err, a2a.ErrUnsupportedOperation) {
				t.Fatalf("expected ErrUnsupportedOperation, got %v", err)
			}
		}
		got, err := client.GetTask(ctx, &a2a.GetTaskRequest{ID: "task-t"})
		if err != nil {
			t.Fatalf("GetTask: %v", err)
		}
		if got.Status.State != a2a.TaskStateCompleted || !got.Status.State.Terminal() {
			t.Fatalf("terminal truth lost: %v", got.Status.State)
		}
	})

	t.Run("validation", func(t *testing.T) {
		newHandler := func() *probeHandler {
			return &probeHandler{sendResult: &a2a.Task{ID: "task-v", ContextID: "ctx-v", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}}
		}
		msg := func() *a2a.SendMessageRequest {
			return &a2a.SendMessageRequest{Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hi"))}
		}
		newGuarded := func(t *testing.T, inject map[string][]string, requiredExts ...string) *a2aclient.Client {
			return newAuthedProbeClient(t, newGuardedServer(t, newHandler(), requiredExts...), "probe-good-token", inject)
		}

		t.Run("version", func(t *testing.T) {
			client := newGuarded(t, map[string][]string{a2a.SvcParamVersion: {"9.9"}})
			if _, err := client.SendMessage(ctx, msg()); !errors.Is(err, a2a.ErrVersionNotSupported) {
				t.Fatalf("expected ErrVersionNotSupported, got %v", err)
			}
		})
		t.Run("content", func(t *testing.T) {
			client := newGuarded(t, nil)
			bad := msg()
			bad.Message.Parts = append(bad.Message.Parts, a2a.NewRawPart([]byte{0x00, 0x01}))
			if _, err := client.SendMessage(ctx, bad); !errors.Is(err, a2a.ErrUnsupportedContentType) {
				t.Fatalf("expected ErrUnsupportedContentType, got %v", err)
			}
		})
		t.Run("tenant", func(t *testing.T) {
			h := newHandler()
			h.boundTenant = "tenant-local"
			client := newAuthedProbeClient(t, newGuardedServer(t, h), "probe-good-token", nil)
			tctx := a2a.AttachTenant(ctx, "tenant-foreign")
			if _, err := client.SendMessage(tctx, msg()); !errors.Is(err, a2a.ErrUnauthorized) {
				t.Fatalf("expected ErrUnauthorized, got %v", err)
			}
		})
		t.Run("extension", func(t *testing.T) {
			// The probe server requires an extension; the stock official
			// client sends no A2A-Extensions parameter and must be rejected.
			client := newGuarded(t, nil, probeRequiredExtension)
			if _, err := client.SendMessage(ctx, msg()); !errors.Is(err, a2a.ErrExtensionSupportRequired) {
				t.Fatalf("expected ErrExtensionSupportRequired, got %v", err)
			}
			// An interceptor that declares the extension unblocks the call.
			okClient := newGuarded(t, map[string][]string{a2a.SvcParamExtensions: {probeRequiredExtension}}, probeRequiredExtension)
			if _, err := okClient.SendMessage(ctx, msg()); err != nil {
				t.Fatalf("declared extension still rejected: %v", err)
			}
		})
	})

	t.Run("middleware", func(t *testing.T) {
		h := &probeHandler{sendResult: &a2a.Task{ID: "task-m", ContextID: "ctx-m", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}}
		srv := newGuardedServer(t, h)
		authed := newAuthedProbeClient(t, srv, "probe-good-token", nil)
		if _, err := authed.SendMessage(ctx, &a2a.SendMessageRequest{
			Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hi")),
		}); err != nil {
			t.Fatalf("authed SendMessage: %v", err)
		}
		obs := h.last()
		if obs.marker != "probe-principal-1" {
			t.Fatalf("private context marker did not reach handler: %+v", obs)
		}
		if len(obs.authHeaders) != 0 {
			t.Fatalf("stripped bearer reached handler: %v", obs.authHeaders)
		}
		if got := obs.versionParams; len(got) != 1 || got[0] != "1.0" {
			t.Fatalf("client did not declare version 1.0: %v", got)
		}

		unauthed := newAuthedProbeClient(t, srv, "probe-bad-token", nil)
		if _, err := unauthed.SendMessage(ctx, &a2a.SendMessageRequest{
			Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hi")),
		}); err == nil {
			t.Fatal("unauthenticated request reached the handler path")
		}
		if h.callCount() != 1 {
			t.Fatalf("rejected request still invoked handler: %d calls", h.callCount())
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("preHeader", func(t *testing.T) {
			h := &probeHandler{getErr: a2a.NewError(a2a.ErrTaskNotFound, "no such task")}
			client := newProbeClient(t, newProbeServer(t, h), nil)
			_, err := client.GetTask(ctx, &a2a.GetTaskRequest{ID: "missing"})
			if !errors.Is(err, a2a.ErrTaskNotFound) {
				t.Fatalf("expected ErrTaskNotFound, got %v", err)
			}
		})
		t.Run("postHeader", func(t *testing.T) {
			task := &a2a.Task{ID: "task-e", ContextID: "ctx-e", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}
			h := &probeHandler{streamScript: []streamItem{
				{event: task},
				{err: a2a.NewError(a2a.ErrInternalError, "mid-stream failure")},
			}}
			client := newProbeClient(t, newProbeServer(t, h), nil)
			var events int
			var streamErr error
			for ev, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
				Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("go")),
			}) {
				if err != nil {
					streamErr = err
					break
				}
				_ = ev
				events++
			}
			if events != 1 || !errors.Is(streamErr, a2a.ErrInternalError) {
				t.Fatalf("post-header error not decoded: events=%d err=%v", events, streamErr)
			}
		})
	})

	t.Run("limits", func(t *testing.T) {
		// SSE client frame bound: an event over MaxSSETokenSize (10MB) must
		// surface as an error, never a truncated success.
		big := strings.Repeat("x", 11*1024*1024)
		task := &a2a.Task{ID: "task-l", ContextID: "ctx-l", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}
		h := &probeHandler{streamScript: []streamItem{
			{event: task},
			{event: a2a.NewArtifactEvent(task, a2a.NewTextPart(big))},
		}}
		client := newProbeClient(t, newProbeServer(t, h), nil)
		var events int
		var streamErr error
		for ev, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
			Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("go")),
		}) {
			if err != nil {
				streamErr = err
				break
			}
			_ = ev
			events++
		}
		if streamErr == nil {
			t.Fatal("oversized SSE frame did not error")
		}
		if events != 1 {
			t.Fatalf("oversized frame partially delivered: %d events", events)
		}
		if !strings.Contains(streamErr.Error(), "SSE") && !strings.Contains(streamErr.Error(), "token") {
			t.Logf("frame error surfaced as: %v", streamErr)
		}
	})
}
