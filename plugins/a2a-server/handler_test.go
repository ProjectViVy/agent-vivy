package a2aserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"agent-vivy/sdk/port/channel"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// fakeTasks is the in-package TaskHost double: scripted snapshots and
// updates, every Submit recorded to prove validation happens pre-admission.
type fakeTasks struct {
	mu        sync.Mutex
	submits   []channel.TaskRequest
	submitRef channel.TaskRef
	submitErr error
	getSnap   channel.TaskSnapshot
	getErr    error
	updates   []channel.TaskUpdate
	streamErr error
}

func (f *fakeTasks) SubmitTask(_ context.Context, req channel.TaskRequest) (channel.TaskRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submits = append(f.submits, req)
	return f.submitRef, f.submitErr
}

func (f *fakeTasks) GetTask(context.Context, channel.TaskQuery) (channel.TaskSnapshot, error) {
	return f.getSnap, f.getErr
}

func (f *fakeTasks) ListTasks(context.Context, channel.TaskListQuery) (channel.TaskPage, error) {
	return channel.TaskPage{Tasks: []channel.TaskSnapshot{f.getSnap}, TotalSize: 1, PageSize: 50}, nil
}

func (f *fakeTasks) CancelTask(context.Context, channel.TaskQuery) (channel.TaskSnapshot, error) {
	return f.getSnap, f.getErr
}

func (f *fakeTasks) SubscribeTask(context.Context, channel.TaskSubscription) (channel.TaskStream, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return &fakeStream{updates: append([]channel.TaskUpdate{}, f.updates...)}, nil
}

func (f *fakeTasks) submitCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.submits)
}

type fakeStream struct {
	updates []channel.TaskUpdate
	i       int
}

func (s *fakeStream) Next(context.Context) (channel.TaskUpdate, error) {
	if s.i >= len(s.updates) {
		return channel.TaskUpdate{}, io.EOF
	}
	up := s.updates[s.i]
	s.i++
	return up, nil
}

func (s *fakeStream) Close() error { return nil }

type fakeInfo struct{ info channel.TaskServiceInfo }

func (f fakeInfo) TaskServiceInfo(context.Context) (channel.TaskServiceInfo, error) {
	return f.info, nil
}

func workingSnap() channel.TaskSnapshot {
	return channel.TaskSnapshot{
		Ref:    channel.TaskRef{TaskID: "task-1", ContextID: "ctx-1"},
		Status: channel.TaskStatus{State: channel.TaskStateWorking, UpdatedAt: time.Now()},
	}
}

func newTestServer(t *testing.T, f *fakeTasks) *httptest.Server {
	t.Helper()
	h := newRequestHandler(f, fakeInfo{info: channel.TaskServiceInfo{Name: "vivy", Version: "0.1.0", Streaming: true, PublicEndpoint: "http://x"}})
	h.settings = a2aSettings{}
	srv := httptest.NewServer(newHTTPHandler(h))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, srv *httptest.Server) *a2aclient.Client {
	t.Helper()
	client, err := a2aclient.NewFromCard(context.Background(), &a2a.AgentCard{
		Name:         "test",
		Capabilities: a2a.AgentCapabilities{Streaming: true},
		SupportedInterfaces: []*a2a.AgentInterface{{
			URL:             srv.URL + "/a2a",
			ProtocolBinding: a2a.TransportProtocolJSONRPC,
			ProtocolVersion: a2a.Version,
		}},
	}, a2aclient.WithJSONRPCTransport(nil))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return client
}

func userMsg(text string) *a2a.Message {
	return a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))
}

func TestA2ACustomHandlerOfficialClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("compile", func(t *testing.T) {
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
		} = (*requestHandler)(nil)
	})

	t.Run("send nonblocking submits once and returns durable receipt", func(t *testing.T) {
		f := &fakeTasks{
			submitRef: channel.TaskRef{TaskID: "task-1", ContextID: "ctx-1"},
			getSnap:   workingSnap(),
		}
		client := newTestClient(t, newTestServer(t, f))
		got, err := client.SendMessage(ctx, &a2a.SendMessageRequest{
			Config:  &a2a.SendMessageConfig{ReturnImmediately: true},
			Message: userMsg("ping"),
		})
		if err != nil {
			t.Fatalf("SendMessage: %v", err)
		}
		task, ok := got.(*a2a.Task)
		if !ok || task.ID != "task-1" || task.ContextID != "ctx-1" {
			t.Fatalf("result: %#v", got)
		}
		if f.submitCount() != 1 {
			t.Fatalf("submits = %d", f.submitCount())
		}
	})

	t.Run("invalid requests never submit", func(t *testing.T) {
		cases := map[string]*a2a.SendMessageRequest{
			"nil message":      {},
			"agent role":       {Message: a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("x"))},
			"empty parts":      {Message: a2a.NewMessage(a2a.MessageRoleUser)},
			"nontext part":     {Message: func() *a2a.Message { m := userMsg("a"); m.Parts = append(m.Parts, a2a.NewRawPart([]byte{1})); return m }()},
			"tenant set":       {Tenant: "t-foreign", Message: userMsg("x")},
			"extensions":       {Message: func() *a2a.Message { m := userMsg("x"); m.Extensions = []string{"urn:x"}; return m }()},
			"referenceTaskIds": {Message: func() *a2a.Message { m := userMsg("x"); m.ReferenceTasks = []a2a.TaskID{"t1"}; return m }()},
		}
		for name, req := range cases {
			f := &fakeTasks{submitRef: channel.TaskRef{TaskID: "t", ContextID: "c"}, getSnap: workingSnap()}
			client := newTestClient(t, newTestServer(t, f))
			_, err := client.SendMessage(ctx, req)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if f.submitCount() != 0 {
				t.Fatalf("%s: invalid request reached SubmitTask", name)
			}
		}
	})

	t.Run("get and cancel map through", func(t *testing.T) {
		f := &fakeTasks{getSnap: workingSnap()}
		client := newTestClient(t, newTestServer(t, f))
		got, err := client.GetTask(ctx, &a2a.GetTaskRequest{ID: "task-1"})
		if err != nil || got.Status.State != a2a.TaskStateWorking {
			t.Fatalf("get: %v %+v", err, got)
		}
		f.getSnap.Status.State = channel.TaskStateCanceled
		got, err = client.CancelTask(ctx, &a2a.CancelTaskRequest{ID: "task-1"})
		if err != nil || got.Status.State != a2a.TaskStateCanceled {
			t.Fatalf("cancel: %v %+v", err, got)
		}
	})

	t.Run("get surfaces typed not_found", func(t *testing.T) {
		f := &fakeTasks{getErr: &channel.TaskError{Code: channel.TaskErrNotFound, Message: "no task"}}
		client := newTestClient(t, newTestServer(t, f))
		_, err := client.GetTask(ctx, &a2a.GetTaskRequest{ID: "x"})
		if !errors.Is(err, a2a.ErrTaskNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("stream yields snapshot then updates to EOF", func(t *testing.T) {
		snap := workingSnap()
		completed := snap
		completed.Status.State = channel.TaskStateCompleted
		f := &fakeTasks{
			submitRef: channel.TaskRef{TaskID: "task-1", ContextID: "ctx-1"},
			updates: []channel.TaskUpdate{
				{Snapshot: &snap},
				{Status: &channel.TaskStatus{State: channel.TaskStateWorking}},
				{Artifact: &channel.TaskArtifact{ID: "a1", Parts: []channel.TaskTextPart{{Text: "chunk"}}}},
				{Status: &channel.TaskStatus{State: channel.TaskStateCompleted}},
			},
		}
		client := newTestClient(t, newTestServer(t, f))
		var events []a2a.Event
		for ev, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{Message: userMsg("go")}) {
			if err != nil {
				t.Fatalf("stream err: %v", err)
			}
			events = append(events, ev)
		}
		if len(events) != 4 {
			t.Fatalf("events = %d", len(events))
		}
		if _, ok := events[0].(*a2a.Task); !ok {
			t.Fatalf("first event must be Task, got %T", events[0])
		}
		if su, ok := events[1].(*a2a.TaskStatusUpdateEvent); !ok || su.Status.State != a2a.TaskStateWorking {
			t.Fatalf("second event = %T", events[1])
		}
		art, ok := events[2].(*a2a.TaskArtifactUpdateEvent)
		if !ok || art.Append || !art.LastChunk {
			t.Fatalf("artifact event flags: %#v", events[2])
		}
		if su, ok := events[3].(*a2a.TaskStatusUpdateEvent); !ok || su.Status.State != a2a.TaskStateCompleted {
			t.Fatalf("terminal event = %#v", events[3])
		}
	})

	t.Run("stream ends on interrupted states", func(t *testing.T) {
		for _, st := range []channel.TaskState{channel.TaskStateInputRequired, channel.TaskStateAuthorizationRequired} {
			snap := workingSnap()
			f := &fakeTasks{
				submitRef: channel.TaskRef{TaskID: "t", ContextID: "c"},
				updates: []channel.TaskUpdate{
					{Snapshot: &snap},
					{Status: &channel.TaskStatus{State: st}},
				},
			}
			client := newTestClient(t, newTestServer(t, f))
			var n int
			var serr error
			for _, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{Message: userMsg("go")}) {
				if err != nil {
					serr = err
					break
				}
				n++
			}
			if serr != nil || n != 2 {
				t.Fatalf("%s: events=%d err=%v", st, n, serr)
			}
		}
	})

	t.Run("push and extended card are unsupported", func(t *testing.T) {
		f := &fakeTasks{}
		client := newTestClient(t, newTestServer(t, f))
		if _, err := client.CreateTaskPushConfig(ctx, &a2a.PushConfig{TaskID: "t"}); !errors.Is(err, a2a.ErrPushNotificationNotSupported) {
			t.Fatalf("push: %v", err)
		}
		if _, err := client.GetExtendedAgentCard(ctx, &a2a.GetExtendedAgentCardRequest{}); !errors.Is(err, a2a.ErrExtendedCardNotConfigured) {
			t.Fatalf("extended card: %v", err)
		}
	})

	t.Run("card is public and truthful", func(t *testing.T) {
		f := &fakeTasks{}
		srv := newTestServer(t, f)
		resp, err := srv.Client().Get(srv.URL + "/.well-known/agent-card.json")
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		defer resp.Body.Close()
		var card a2a.AgentCard
		if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
			t.Fatalf("card decode: %v", err)
		}
		if card.Name != "vivy" || !card.Capabilities.Streaming {
			t.Fatalf("card: %+v", card)
		}
		if card.Capabilities.PushNotifications || card.Capabilities.ExtendedAgentCard {
			t.Fatalf("card advertises unsupported capabilities: %+v", card.Capabilities)
		}
		if _, ok := card.SecuritySchemes["bearer"]; !ok {
			t.Fatalf("card omits bearer scheme: %+v", card.SecuritySchemes)
		}
	})
}
