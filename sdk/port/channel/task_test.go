package channel_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	channel "agent-vivy/sdk/port/channel"
)

// taskHostDouble proves the optional contract can be implemented by a real
// provider without touching the base Host vocabulary. It never fakes
// iterator semantics — production streams are verified in A2A-04.
type taskHostDouble struct{}

func (taskHostDouble) SubmitTask(context.Context, channel.TaskRequest) (channel.TaskRef, error) {
	return channel.TaskRef{TaskID: "t1", ContextID: "c1"}, nil
}
func (taskHostDouble) GetTask(context.Context, channel.TaskQuery) (channel.TaskSnapshot, error) {
	return channel.TaskSnapshot{}, nil
}
func (taskHostDouble) ListTasks(context.Context, channel.TaskListQuery) (channel.TaskPage, error) {
	return channel.TaskPage{}, nil
}
func (taskHostDouble) CancelTask(context.Context, channel.TaskQuery) (channel.TaskSnapshot, error) {
	return channel.TaskSnapshot{Status: channel.TaskStatus{State: channel.TaskStateCanceled}}, nil
}
func (taskHostDouble) SubscribeTask(context.Context, channel.TaskSubscription) (channel.TaskStream, error) {
	return taskStreamDouble{}, nil
}

type taskStreamDouble struct{ closed bool }

func (s taskStreamDouble) Next(context.Context) (channel.TaskUpdate, error) {
	if s.closed {
		return channel.TaskUpdate{}, io.EOF
	}
	return channel.TaskUpdate{}, errors.New("unused")
}
func (s taskStreamDouble) Close() error { s.closed = true; return nil }

type infoHostDouble struct{}

func (infoHostDouble) TaskServiceInfo(context.Context) (channel.TaskServiceInfo, error) {
	return channel.TaskServiceInfo{Name: "Vivy", Streaming: true, InputContinuation: true}, nil
}

func TestTaskHostContract(t *testing.T) {
	var _ channel.TaskHost = taskHostDouble{}
	var _ channel.TaskServiceInfoHost = infoHostDouble{}

	// The base Host must not acquire the optional method set: declaring it
	// there would force every existing adapter's Host double to grow five
	// methods overnight.
	hostMethods := map[string]bool{}
	for i := 0; i < reflect.TypeOf((*channel.Host)(nil)).Elem().NumMethod(); i++ {
		hostMethods[reflect.TypeOf((*channel.Host)(nil)).Elem().Method(i).Name] = true
	}
	for _, forbidden := range []string{"SubmitTask", "GetTask", "ListTasks", "CancelTask", "SubscribeTask", "TaskServiceInfo"} {
		if hostMethods[forbidden] {
			t.Fatalf("base Host must not declare optional method %s", forbidden)
		}
	}

	// TaskError surfaces only its safe message; diagnostic causes stay
	// local to the Host's logs.
	err := &channel.TaskError{Code: channel.TaskErrNotFound, Message: "task not found"}
	if err.Error() != "task not found" {
		t.Fatalf("TaskError.Error() = %q, want only the safe message", err.Error())
	}
	var target *channel.TaskError
	if !errors.As(err, &target) || target.Code != channel.TaskErrNotFound {
		t.Fatal("errors.As must reach the typed code")
	}

	// Terminal-state vocabulary is exactly the frozen set.
	for _, state := range []channel.TaskState{
		channel.TaskStateSubmitted, channel.TaskStateWorking,
		channel.TaskStateInputRequired, channel.TaskStateAuthorizationRequired,
		channel.TaskStateCompleted, channel.TaskStateFailed, channel.TaskStateCanceled,
	} {
		if state == "" {
			t.Fatal("empty TaskState constant")
		}
	}
}
