package domain

import "fmt"

// RunStatus is the lifecycle state of a run (§3.1).
type RunStatus string

const (
	RunAccepted  RunStatus = "accepted"
	RunQueued    RunStatus = "queued"
	RunActive    RunStatus = "active"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

// validTransitions encodes the run state machine:
//
//	accepted ──> queued ──> active ──> completed
//	   │                        ├──> failed
//	   └────────────────────────┴──> cancelled
//
// Terminal states have no outgoing transitions; this is the domain-side
// pillar of the exactly-one-terminal invariant (D-008).
var validTransitions = map[RunStatus][]RunStatus{
	RunAccepted: {RunQueued, RunActive, RunCancelled},
	RunQueued:   {RunActive, RunCancelled},
	RunActive:   {RunCompleted, RunFailed, RunCancelled},
}

// RunKind distinguishes the user-facing root run from an independently
// supervised child run. Child runs use the same lifecycle state machine and
// Journal contracts as root runs.
type RunKind string

const (
	RunKindPrimary RunKind = "primary"
	RunKindChild   RunKind = "child"
)

func (k RunKind) Valid() bool {
	return k == RunKindPrimary || k == RunKindChild
}

// Terminal returns true for completed/failed/cancelled.
func (s RunStatus) Terminal() bool {
	switch s {
	case RunCompleted, RunFailed, RunCancelled:
		return true
	}
	return false
}

// Valid reports whether the status is part of the vocabulary.
func (s RunStatus) Valid() bool {
	switch s {
	case RunAccepted, RunQueued, RunActive, RunCompleted, RunFailed, RunCancelled:
		return true
	}
	return false
}

// CanTransitionTo reports whether the state machine allows s -> next.
func (s RunStatus) CanTransitionTo(next RunStatus) bool {
	for _, ok := range validTransitions[s] {
		if ok == next {
			return true
		}
	}
	return false
}

// TransitionTo validates and applies s -> next. Invalid transitions —
// including any move out of a terminal state — return an error and leave
// the receiver untouched.
func (s *RunStatus) TransitionTo(next RunStatus) error {
	if !s.CanTransitionTo(next) {
		return fmt.Errorf("invalid run transition %q -> %q", *s, next)
	}
	*s = next
	return nil
}

// Run is one agent execution against a session.
type Run struct {
	ID        RunID
	SessionID SessionID
	Status    RunStatus
	CreatedAt int64 // unix milli
	Kind      RunKind
	ParentID  RunID
	RootID    RunID
	Depth     int
}
