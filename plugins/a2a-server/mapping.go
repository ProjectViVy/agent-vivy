package a2aserver

import (
	"errors"

	"agent-vivy/sdk/port/channel"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// mapTaskError is the sole Host-error -> protocol-error boundary
// (design §11): standard SDK constants for standard cases; busy/limit is
// a documented server-error classification, never an invented A2A error.
func mapTaskError(err error) error {
	var te *channel.TaskError
	if !errors.As(err, &te) {
		return a2a.NewError(a2a.ErrInternalError, "internal error")
	}
	switch te.Code {
	case channel.TaskErrInvalid, channel.TaskErrConflict, channel.TaskErrCursorInvalid:
		return a2a.NewError(a2a.ErrInvalidParams, te.Message)
	case channel.TaskErrNotFound:
		return a2a.NewError(a2a.ErrTaskNotFound, te.Message)
	case channel.TaskErrDenied:
		return a2a.NewError(a2a.ErrUnauthorized, te.Message)
	case channel.TaskErrUnsupported:
		return a2a.NewError(a2a.ErrUnsupportedOperation, te.Message)
	case channel.TaskErrNotCancelable:
		return a2a.NewError(a2a.ErrTaskNotCancelable, te.Message)
	case channel.TaskErrBusy, channel.TaskErrLimit:
		return a2a.NewError(a2a.ErrServerError, te.Message)
	default: // unavailable, corrupt, unknown
		return a2a.NewError(a2a.ErrInternalError, te.Message)
	}
}

// mapTaskSnapshot is the sole snapshot -> wire projection boundary.
// The Host projection is already safe: text parts only, no raw payloads,
// no metadata; Revision stays host-internal (not a wire extension).
func mapTaskSnapshot(s channel.TaskSnapshot) (*a2a.Task, error) {
	task := &a2a.Task{
		ID:        a2a.TaskID(s.Ref.TaskID),
		ContextID: s.Ref.ContextID,
		Status:    mapTaskStatus(s.Status),
	}
	for _, m := range s.History {
		task.History = append(task.History, mapTaskMessage(m))
	}
	for _, a := range s.Artifacts {
		task.Artifacts = append(task.Artifacts, mapTaskArtifact(a))
	}
	return task, nil
}

var taskStateMap = map[channel.TaskState]a2a.TaskState{
	channel.TaskStateSubmitted:             a2a.TaskStateSubmitted,
	channel.TaskStateWorking:               a2a.TaskStateWorking,
	channel.TaskStateInputRequired:         a2a.TaskStateInputRequired,
	channel.TaskStateAuthorizationRequired: a2a.TaskStateAuthRequired,
	channel.TaskStateCompleted:             a2a.TaskStateCompleted,
	channel.TaskStateFailed:                a2a.TaskStateFailed,
	channel.TaskStateCanceled:              a2a.TaskStateCanceled,
}

func mapTaskState(s channel.TaskState) (a2a.TaskState, error) {
	if st, ok := taskStateMap[s]; ok {
		return st, nil
	}
	return a2a.TaskStateUnspecified, errors.New("unknown task state")
}

func mapTaskStatus(s channel.TaskStatus) a2a.TaskStatus {
	st, _ := mapTaskState(s.State)
	out := a2a.TaskStatus{State: st}
	if !s.UpdatedAt.IsZero() {
		t := s.UpdatedAt
		out.Timestamp = &t
	}
	if s.Message != nil {
		out.Message = mapTaskMessage(*s.Message)
	}
	return out
}

func mapTaskMessage(m channel.TaskMessage) *a2a.Message {
	role := a2a.MessageRoleAgent
	if m.Role == "user" {
		role = a2a.MessageRoleUser
	}
	return &a2a.Message{ID: m.ID, Role: role, Parts: mapParts(m.Parts)}
}

func mapTaskArtifact(a channel.TaskArtifact) *a2a.Artifact {
	return &a2a.Artifact{ID: a2a.ArtifactID(a.ID), Name: a.Name, Parts: mapParts(a.Parts)}
}

func mapParts(parts []channel.TaskTextPart) a2a.ContentParts {
	out := make(a2a.ContentParts, 0, len(parts))
	for _, p := range parts {
		out = append(out, a2a.NewTextPart(p.Text))
	}
	return out
}
