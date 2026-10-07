package a2aserver

import (
	"errors"
	"testing"
	"time"

	"agent-vivy/sdk/port/channel"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestMapTaskErrorTable(t *testing.T) {
	cases := []struct {
		code channel.TaskErrorCode
		want error
	}{
		{channel.TaskErrInvalid, a2a.ErrInvalidParams},
		{channel.TaskErrConflict, a2a.ErrInvalidParams},
		{channel.TaskErrCursorInvalid, a2a.ErrInvalidParams},
		{channel.TaskErrNotFound, a2a.ErrTaskNotFound},
		{channel.TaskErrDenied, a2a.ErrUnauthorized},
		{channel.TaskErrUnsupported, a2a.ErrUnsupportedOperation},
		{channel.TaskErrNotCancelable, a2a.ErrTaskNotCancelable},
		{channel.TaskErrBusy, a2a.ErrServerError},
		{channel.TaskErrLimit, a2a.ErrServerError},
		{channel.TaskErrUnavailable, a2a.ErrInternalError},
		{channel.TaskErrCorrupt, a2a.ErrInternalError},
	}
	for _, tc := range cases {
		err := mapTaskError(&channel.TaskError{Code: tc.code, Message: "m"})
		if !errors.Is(err, tc.want) {
			t.Fatalf("%s -> %v, want %v", tc.code, err, tc.want)
		}
	}
	if err := mapTaskError(errors.New("raw")); !errors.Is(err, a2a.ErrInternalError) {
		t.Fatalf("untyped error leaked: %v", err)
	}
}

func TestMapTaskSnapshot(t *testing.T) {
	ts := time.Date(2026, 10, 7, 1, 0, 0, 0, time.UTC)
	snap := channel.TaskSnapshot{
		Ref: channel.TaskRef{TaskID: "t-1", ContextID: "ctx-1"},
		Status: channel.TaskStatus{
			State:     channel.TaskStateInputRequired,
			UpdatedAt: ts,
			Message:   &channel.TaskMessage{ID: "m-q", Role: "agent", Parts: []channel.TaskTextPart{{Text: "what?"}}},
		},
		History: []channel.TaskMessage{
			{ID: "m-1", Role: "user", Parts: []channel.TaskTextPart{{Text: "hi"}}},
			{ID: "m-2", Role: "agent", Parts: []channel.TaskTextPart{{Text: "hello"}}},
		},
		Artifacts: []channel.TaskArtifact{
			{ID: "art-1", Name: "out", Parts: []channel.TaskTextPart{{Text: "result"}}},
		},
		Revision: "h9:t-1",
	}
	task, err := mapTaskSnapshot(snap)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if task.ID != "t-1" || task.ContextID != "ctx-1" {
		t.Fatalf("ids: %v %v", task.ID, task.ContextID)
	}
	if task.Status.State != a2a.TaskStateInputRequired {
		t.Fatalf("state: %v", task.Status.State)
	}
	if task.Status.Timestamp == nil || !task.Status.Timestamp.Equal(ts) {
		t.Fatalf("timestamp: %v", task.Status.Timestamp)
	}
	if task.Status.Message == nil || task.Status.Message.Role != a2a.MessageRoleAgent {
		t.Fatalf("status message: %+v", task.Status.Message)
	}
	if len(task.History) != 2 || task.History[0].Role != a2a.MessageRoleUser || task.History[1].Role != a2a.MessageRoleAgent {
		t.Fatalf("history: %+v", task.History)
	}
	if len(task.Artifacts) != 1 || string(task.Artifacts[0].ID) != "art-1" {
		t.Fatalf("artifacts: %+v", task.Artifacts)
	}
	// Revision is host-internal — never a wire extension.
	if task.Metadata != nil {
		t.Fatalf("metadata must not carry internals: %+v", task.Metadata)
	}
}
