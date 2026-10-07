// The A2A server adapter: official a2asrv transport mounted on the
// channel port's optional TaskHost surface. This file holds only
// request/response plumbing — wire mapping lives in mapping.go, card
// construction in card.go.
package a2aserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"time"

	"agent-vivy/sdk/port/channel"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// blockingSendDeadline is the adapter's blocking-message cap (design
// §10.1: 60s). A timeout is a safe server error, never a synthesized
// working Task — the caller retries with returnImmediately.
const blockingSendDeadline = 60 * time.Second

// requestHandler implements the pinned a2asrv.RequestHandler (v2.6.0):
// all 11 methods, consuming the Host's task surface.
type requestHandler struct {
	tasks    channel.TaskHost
	info     channel.TaskServiceInfoHost
	settings a2aSettings
}

func newRequestHandler(tasks channel.TaskHost, info channel.TaskServiceInfoHost) *requestHandler {
	return &requestHandler{tasks: tasks, info: info}
}

var _ a2asrv.RequestHandler = (*requestHandler)(nil)

// newHTTPHandler mounts the official JSON-RPC transport plus the public
// agent card route on one handler. Authentication is the Host listener's
// job — this handler trusts the already-stripped request.
func newHTTPHandler(h *requestHandler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /.well-known/agent-card.json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		card, err := h.card(r.Context())
		if err != nil {
			http.Error(w, "card unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, card)
	}))
	mux.Handle("/a2a", a2asrv.NewJSONRPCHandler(&a2asrv.InterceptedHandler{
		Handler:      h,
		Interceptors: []a2asrv.CallInterceptor{guardInterceptor{}},
	}))
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encode failed", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

// guardInterceptor is the protocol admission seam: exactly one
// A2A-Version=1.0 service parameter and no tenant binding — the endpoint
// is already pinned to one authenticated principal, so a client-sent
// tenant is unsupported, not ignored.
type guardInterceptor struct{}

func (guardInterceptor) Before(ctx context.Context, callCtx *a2asrv.CallContext, req *a2asrv.Request) (context.Context, any, error) {
	if sp := callCtx.ServiceParams(); sp != nil {
		if v, ok := sp.Get(a2a.SvcParamVersion); ok && (len(v) != 1 || v[0] != "1.0") {
			return ctx, nil, a2a.NewError(a2a.ErrVersionNotSupported, "server requires A2A-Version: 1.0")
		}
	}
	return ctx, nil, nil
}

func (guardInterceptor) After(context.Context, *a2asrv.CallContext, *a2asrv.Response) error {
	return nil
}

// checkSendMessage enforces the caller-message contract before admission:
// user role, non-empty text-only parts, and no tenant/extension/reference
// selectors or metadata-driven privilege — invalid requests never reach
// SubmitTask.
func checkSendMessage(req *a2a.SendMessageRequest) error {
	if req == nil || req.Message == nil {
		return a2a.NewError(a2a.ErrInvalidParams, "message is required")
	}
	if req.Tenant != "" {
		return a2a.NewError(a2a.ErrInvalidParams, "tenant is not supported by this endpoint")
	}
	m := req.Message
	if m.Role != a2a.MessageRoleUser {
		return a2a.NewError(a2a.ErrInvalidParams, "only user-role messages are accepted")
	}
	if len(m.Parts) == 0 {
		return a2a.NewError(a2a.ErrInvalidParams, "message has no parts")
	}
	if len(m.Extensions) > 0 || len(m.ReferenceTasks) > 0 {
		return a2a.NewError(a2a.ErrInvalidParams, "message extensions and referenceTaskIds are not supported")
	}
	for _, p := range m.Parts {
		if p == nil {
			return a2a.NewError(a2a.ErrInvalidParams, "nil part")
		}
		if _, ok := p.Content.(a2a.Text); !ok {
			return a2a.NewError(a2a.ErrUnsupportedContentType, "only text parts accepted")
		}
	}
	if req.Config != nil && req.Config.PushConfig != nil {
		return a2a.NewError(a2a.ErrPushNotificationNotSupported, "push notifications not supported")
	}
	return nil
}

func (h *requestHandler) taskRequest(m *a2a.Message) channel.TaskRequest {
	parts := make([]channel.TaskTextPart, 0, len(m.Parts))
	for _, p := range m.Parts {
		parts = append(parts, channel.TaskTextPart{Text: string(p.Content.(a2a.Text))})
	}
	return channel.TaskRequest{
		MessageID: m.ID,
		ContextID: m.ContextID,
		TaskID:    string(m.TaskID),
		Parts:     parts,
	}
}

func (h *requestHandler) GetTask(ctx context.Context, req *a2a.GetTaskRequest) (*a2a.Task, error) {
	if req == nil || req.ID == "" {
		return nil, a2a.NewError(a2a.ErrInvalidParams, "id is required")
	}
	snap, err := h.tasks.GetTask(ctx, channel.TaskQuery{TaskID: string(req.ID), HistoryLimit: req.HistoryLength})
	if err != nil {
		return nil, mapTaskError(err)
	}
	task, err := mapTaskSnapshot(snap)
	if err != nil {
		return nil, a2a.NewError(a2a.ErrInternalError, "projection failed")
	}
	return task, nil
}

func (h *requestHandler) ListTasks(ctx context.Context, req *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	q := channel.TaskListQuery{
		ContextID:    req.ContextID,
		PageSize:     req.PageSize,
		PageToken:    req.PageToken,
		HistoryLimit: req.HistoryLength,
	}
	if req.Status != "" {
		st := a2a.TaskState(req.Status)
		found := false
		for cs, as := range taskStateMap {
			if as == st {
				q.State = cs
				found = true
			}
		}
		if !found {
			return nil, a2a.NewError(a2a.ErrInvalidParams, "unsupported status filter")
		}
	}
	if req.StatusTimestampAfter != nil {
		q.UpdatedAfter = req.StatusTimestampAfter
	}
	page, err := h.tasks.ListTasks(ctx, q)
	if err != nil {
		return nil, mapTaskError(err)
	}
	resp := &a2a.ListTasksResponse{
		PageSize:      page.PageSize,
		NextPageToken: page.NextPageToken,
		TotalSize:     page.TotalSize,
	}
	for _, s := range page.Tasks {
		task, err := mapTaskSnapshot(s)
		if err != nil {
			return nil, a2a.NewError(a2a.ErrInternalError, "projection failed")
		}
		resp.Tasks = append(resp.Tasks, task)
	}
	return resp, nil
}

func (h *requestHandler) CancelTask(ctx context.Context, req *a2a.CancelTaskRequest) (*a2a.Task, error) {
	if req == nil || req.ID == "" {
		return nil, a2a.NewError(a2a.ErrInvalidParams, "id is required")
	}
	snap, err := h.tasks.CancelTask(ctx, channel.TaskQuery{TaskID: string(req.ID)})
	if err != nil {
		return nil, mapTaskError(err)
	}
	task, err := mapTaskSnapshot(snap)
	if err != nil {
		return nil, a2a.NewError(a2a.ErrInternalError, "projection failed")
	}
	return task, nil
}

func (h *requestHandler) SendMessage(ctx context.Context, req *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	if err := checkSendMessage(req); err != nil {
		return nil, err
	}
	ref, err := h.tasks.SubmitTask(ctx, h.taskRequest(req.Message))
	if err != nil {
		return nil, mapTaskError(err)
	}
	// After commit: a nonblocking send returns the durable snapshot.
	if req.Config != nil && req.Config.ReturnImmediately {
		return h.snapshotResult(ctx, ref, req.Config.HistoryLength)
	}
	// Blocking waits for terminal/interrupted truth under the 60s cap.
	bctx, cancel := context.WithTimeout(ctx, blockingSendDeadline)
	defer cancel()
	snap, err := h.waitTask(bctx, ref)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			// The adapter deadline fired while the request is still live:
			// a safe timeout error, never a fake working result; the task
			// is NOT canceled (design §10.1).
			return nil, a2a.NewError(a2a.ErrServerError, "blocking wait timed out; retry with returnImmediately")
		}
		return nil, mapTaskError(err)
	}
	return applyHistoryLimit(snap, req)
}

func (h *requestHandler) snapshotResult(ctx context.Context, ref channel.TaskRef, history *int) (a2a.SendMessageResult, error) {
	snap, err := h.tasks.GetTask(ctx, channel.TaskQuery{TaskID: ref.TaskID, HistoryLimit: history})
	if err != nil {
		return nil, mapTaskError(err)
	}
	return mapTaskSnapshot(snap)
}

// waitTask subscribes and returns the first terminal or interrupted
// snapshot. Under option A the stream re-sends the latest state on
// reconnect; the first completed event settles this wait.
func (h *requestHandler) waitTask(ctx context.Context, ref channel.TaskRef) (*a2a.Task, error) {
	stream, err := h.tasks.SubscribeTask(ctx, channel.TaskSubscription{TaskID: ref.TaskID})
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	for {
		up, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if up.Snapshot != nil && up.Snapshot.Status.State != "" &&
			(up.Snapshot.Status.State == channel.TaskStateCompleted ||
				up.Snapshot.Status.State == channel.TaskStateFailed ||
				up.Snapshot.Status.State == channel.TaskStateCanceled ||
				up.Snapshot.Status.State == channel.TaskStateInputRequired ||
				up.Snapshot.Status.State == channel.TaskStateAuthorizationRequired) {
			return mapTaskSnapshot(*up.Snapshot)
		}
		if up.Status != nil {
			if st := up.Status.State; st == channel.TaskStateCompleted ||
				st == channel.TaskStateFailed || st == channel.TaskStateCanceled ||
				st == channel.TaskStateInputRequired || st == channel.TaskStateAuthorizationRequired {
				snap, err := h.tasks.GetTask(ctx, channel.TaskQuery{TaskID: ref.TaskID})
				if err != nil {
					return nil, err
				}
				return mapTaskSnapshot(snap)
			}
		}
	}
	// EOF without a settling state: report the latest truth.
	snap, err := h.tasks.GetTask(ctx, channel.TaskQuery{TaskID: ref.TaskID})
	if err != nil {
		return nil, err
	}
	return mapTaskSnapshot(snap)
}

func applyHistoryLimit(task *a2a.Task, req *a2a.SendMessageRequest) (*a2a.Task, error) {
	if req.Config != nil && req.Config.HistoryLength != nil && len(task.History) > *req.Config.HistoryLength {
		task.History = task.History[len(task.History)-*req.Config.HistoryLength:]
	}
	return task, nil
}

// streamUpdates pumps one task stream onto the wire: snapshot or tail
// events from the Host's catch-up, status -> TaskStatusUpdateEvent with
// Final on terminal/interrupted, artifacts as full replacements
// (Append=false, LastChunk=true). io.EOF ends the stream cleanly.
// streamUpdates pumps one task stream onto the wire: snapshot or tail
// events from the Host's catch-up, status -> TaskStatusUpdateEvent with
// Final on terminal/interrupted, artifacts as full replacements
// (Append=false, LastChunk=true). The stream is bound to one task; the
// context ID is learned from its first snapshot. io.EOF ends cleanly.
func (h *requestHandler) streamUpdates(ctx context.Context, stream channel.TaskStream, taskID string, yield func(a2a.Event, error) bool) {
	defer stream.Close()
	contextID := ""
	for {
		up, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			yield(nil, mapTaskError(err))
			return
		}
		var ev a2a.Event
		switch {
		case up.Snapshot != nil:
			task, err := mapTaskSnapshot(*up.Snapshot)
			if err != nil {
				yield(nil, a2a.NewError(a2a.ErrInternalError, "projection failed"))
				return
			}
			contextID = task.ContextID
			ev = task
		case up.Status != nil:
			ev = &a2a.TaskStatusUpdateEvent{
				TaskID:    a2a.TaskID(taskID),
				ContextID: contextID,
				Status:    mapTaskStatus(*up.Status),
			}
		case up.Artifact != nil:
			ev = &a2a.TaskArtifactUpdateEvent{
				TaskID:    a2a.TaskID(taskID),
				ContextID: contextID,
				Artifact:  mapTaskArtifact(*up.Artifact),
				Append:    false,
				LastChunk: true,
			}
		default:
			continue
		}
		if !yield(ev, nil) {
			return
		}
		if st, ok := statusEventState(up); ok && isSettlingState(st) {
			return
		}
	}
}

func isSettlingState(s channel.TaskState) bool {
	switch s {
	case channel.TaskStateCompleted, channel.TaskStateFailed, channel.TaskStateCanceled,
		channel.TaskStateInputRequired, channel.TaskStateAuthorizationRequired:
		return true
	}
	return false
}

func statusEventState(up channel.TaskUpdate) (channel.TaskState, bool) {
	switch {
	case up.Snapshot != nil:
		return up.Snapshot.Status.State, true
	case up.Status != nil:
		return up.Status.State, true
	}
	return "", false
}

func (h *requestHandler) SendStreamingMessage(ctx context.Context, req *a2a.SendMessageRequest) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		if err := checkSendMessage(req); err != nil {
			yield(nil, err)
			return
		}
		ref, err := h.tasks.SubmitTask(ctx, h.taskRequest(req.Message))
		if err != nil {
			yield(nil, mapTaskError(err))
			return
		}
		h.streamTaskUpdates(ctx, ref.TaskID, yield)
	}
}

func (h *requestHandler) SubscribeToTask(ctx context.Context, req *a2a.SubscribeToTaskRequest) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		if req == nil || req.ID == "" {
			yield(nil, a2a.NewError(a2a.ErrInvalidParams, "id is required"))
			return
		}
		h.streamTaskUpdates(ctx, string(req.ID), yield)
	}
}

// streamTaskUpdates subscribes, yields the committed snapshot first when
// the stream supplies one, then ordered updates until EOF.
func (h *requestHandler) streamTaskUpdates(ctx context.Context, taskID string, yield func(a2a.Event, error) bool) {
	stream, err := h.tasks.SubscribeTask(ctx, channel.TaskSubscription{TaskID: taskID})
	if err != nil {
		yield(nil, mapTaskError(err))
		return
	}
	h.streamUpdates(ctx, stream, taskID, yield)
}

func (h *requestHandler) GetTaskPushConfig(context.Context, *a2a.GetTaskPushConfigRequest) (*a2a.PushConfig, error) {
	return nil, a2a.NewError(a2a.ErrPushNotificationNotSupported, "push notifications not supported")
}

func (h *requestHandler) ListTaskPushConfigs(context.Context, *a2a.ListTaskPushConfigRequest) (*a2a.ListTaskPushConfigResponse, error) {
	return nil, a2a.NewError(a2a.ErrPushNotificationNotSupported, "push notifications not supported")
}

func (h *requestHandler) CreateTaskPushConfig(context.Context, *a2a.PushConfig) (*a2a.PushConfig, error) {
	return nil, a2a.NewError(a2a.ErrPushNotificationNotSupported, "push notifications not supported")
}

func (h *requestHandler) DeleteTaskPushConfig(context.Context, *a2a.DeleteTaskPushConfigRequest) error {
	return a2a.NewError(a2a.ErrPushNotificationNotSupported, "push notifications not supported")
}

func (h *requestHandler) GetExtendedAgentCard(context.Context, *a2a.GetExtendedAgentCardRequest) (*a2a.AgentCard, error) {
	return nil, a2a.NewError(a2a.ErrExtendedCardNotConfigured, "no extended card")
}
