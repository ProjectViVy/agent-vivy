// Package reportcontract owns the domain-facing types of the bounded report
// capability: the trusted root workflow admission (R0) and, in later
// stories, publication receipts, settings and scoped actions. No Eino,
// INOFY, storage SQL, or runtime imports cross this package.
package reportcontract

import (
	nb "agent-vivy/internal/notebookcontract"
)

// Period is one report cadence. Periods are input policies of the single
// sealed report program, not separate engines.
type Period string

const (
	PeriodDaily   Period = "daily"
	PeriodWeekly  Period = "weekly"
	PeriodMonthly Period = "monthly"
)

func (p Period) Valid() bool {
	switch p {
	case PeriodDaily, PeriodWeekly, PeriodMonthly:
		return true
	}
	return false
}

// WindowSelector names the report window inside a period: the in-progress
// current window or the last completed one.
type WindowSelector string

const (
	WindowCurrent   WindowSelector = "current"
	WindowCompleted WindowSelector = "completed"
)

func (w WindowSelector) Valid() bool {
	switch w {
	case WindowCurrent, WindowCompleted:
		return true
	}
	return false
}

// TargetRef pins an optional notebook destination the report writes to.
type TargetRef struct {
	SectionID string `json:"section_id,omitempty"`
	EntryID   string `json:"entry_id,omitempty"`
}

// ReportRequest is the caller's stable semantic request. Its canonical form
// is the request digest: a retry rejoins the admitted Run; the same
// operation key with a changed request is an idempotency conflict.
type ReportRequest struct {
	Period       Period         `json:"period"`
	Window       WindowSelector `json:"window"`
	OperationKey string         `json:"operation_key"`
	Target       *TargetRef     `json:"target,omitempty"`
}

// AdmissionContext carries host-resolved provenance. Callers never supply
// scope, actor, or origin — the trusted facade binds them.
type AdmissionContext struct {
	Scope  nb.ScopeID
	Actor  nb.Actor
	Origin nb.Origin
}

// ReportAdmission is the bounded result of one StartReport call.
type ReportAdmission struct {
	RunID    string
	Created  bool
	Rejoined bool
	// Busy reports a distinct operation key declining against a
	// non-terminal Run that already owns the same target.
	Busy bool
}

// Code is the stable domain error code of the bounded result envelope.
type Code string

const (
	CodeInvalidRequest        Code = "invalid_request"
	CodeNotFound              Code = "not_found"
	CodeIdempotencyConflict   Code = "idempotency_conflict"
	CodeCapabilityUnavailable Code = "capability_unavailable"
	CodeBusy                  Code = "busy"
	CodeUnavailable           Code = "unavailable"
)

// Error is the typed report error the sealed Service returns.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }
