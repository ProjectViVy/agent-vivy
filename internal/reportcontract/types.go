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
	CodeCancelled             Code = "cancelled"
	CodeDestinationDeleted    Code = "destination_deleted"
	CodeStorageUnavailable    Code = "storage_unavailable"
	CodeOutcomeUnknown        Code = "outcome_unknown"
	CodeRecoveryRequired      Code = "recovery_required"
)

// Error is the typed report error the sealed Service returns.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// --- R1: bounded generation contracts -----------------------------------------

// Window is the resolved report period: a half-open UTC instant pair plus
// the local-calendar identity and timezone it was computed under.
type Window struct {
	Period  Period `json:"period"`
	ID      string `json:"id"`
	StartMs int64  `json:"start_ms"`
	EndMs   int64  `json:"end_ms"`
	AsOfMs  int64  `json:"as_of_ms"`
	// Completed records whether EndMs <= AsOfMs; manual current windows
	// keep AsOfMs inside the period and are never reported as completed.
	Completed bool `json:"completed"`
}

// ReportSettings is the durable per-(scope,period) configuration projected
// from the single CronJob row that owns it. Disabled rows still carry the
// manual-generation defaults; R3 owns writes and scheduling.
type ReportSettings struct {
	Scope        string `json:"scope"`
	Period       Period `json:"period"`
	Timezone     string `json:"timezone"`
	SectionID    string `json:"section_id"`
	Provider     string `json:"provider,omitempty"`
	ModelID      string `json:"model_id,omitempty"`
	Enabled      bool   `json:"enabled"`
	Revision     int64  `json:"revision"`
	ScheduleExpr string `json:"schedule_expr,omitempty"`
}

// SourceRef identifies one exact authorized evidence item — a session
// message, a reused generated bundle, or a pinned notebook revision — with
// its content digest. Comments and human edits never appear as sources.
type SourceRef struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// FeedbackSnapshot freezes one comment version admitted as user context.
type FeedbackSnapshot struct {
	CommentID string `json:"comment_id"`
	EntryID   string `json:"entry_id"`
	Version   int64  `json:"version"`
	Body      string `json:"body"`
	Digest    string `json:"digest"`
}

// Fact is one collected source fact: which day it belongs to, which sources
// it cites, and the bounded excerpt text.
type Fact struct {
	Date      string      `json:"date"`
	SessionID string      `json:"session_id,omitempty"`
	Text      string      `json:"text"`
	Sources   []SourceRef `json:"sources"`
}

// FactBundle is the frozen collect-node output: facts, coverage disclosure,
// admitted feedback and human-edit context. It is immutable provenance —
// never execution state.
type FactBundle struct {
	Window         Window             `json:"window"`
	SeriesID       string             `json:"series_id"`
	ConfigRevision int64              `json:"config_revision"`
	Facts          []Fact             `json:"facts"`
	Included       int                `json:"included"`
	Excluded       int                `json:"excluded"`
	Truncated      int                `json:"truncated"`
	MissingDates   []string           `json:"missing_dates,omitempty"`
	Feedback       []FeedbackSnapshot `json:"feedback,omitempty"`
	HumanEdits     []FeedbackSnapshot `json:"human_edits,omitempty"`
	Omitted        int                `json:"omitted,omitempty"`
}

// NarrativeMode is the model-outcome classification feeding the renderer.
type NarrativeMode string

const (
	NarrativeModel    NarrativeMode = "model"
	NarrativeEmpty    NarrativeMode = "empty"
	NarrativeFallback NarrativeMode = "fallback"
	NarrativePartial  NarrativeMode = "partial"
)

// Narrative is the strict bounded structured output of the narrate node.
type Narrative struct {
	Mode     NarrativeMode `json:"mode"`
	Reason   string        `json:"reason,omitempty"`
	Sections []struct {
		Heading string `json:"heading"`
		Claims  []struct {
			Text      string   `json:"text"`
			SourceIDs []string `json:"source_ids"`
		} `json:"claims"`
	} `json:"sections"`
}

// GenerationProvenance is the immutable record persisted with the
// publication transaction; it never carries scheduler/execution state.
type GenerationProvenance struct {
	RunID          string `json:"run_id"`
	Scope          string `json:"scope"`
	SeriesID       string `json:"series_id"`
	WindowID       string `json:"window_id"`
	ConfigRevision int64  `json:"config_revision"`
	Timezone       string `json:"timezone"`
	StartMs        int64  `json:"start_ms"`
	EndMs          int64  `json:"end_ms"`
	AsOfMs         int64  `json:"as_of_ms"`
	InputDigest    string `json:"input_digest"`
	FactsDigest    string `json:"facts_digest"`
	Provider       string `json:"provider,omitempty"`
	ModelID        string `json:"model_id,omitempty"`
	OutcomeMode    string `json:"outcome_mode"`
	OutcomeReason  string `json:"outcome_reason,omitempty"`
	EntryID        string `json:"entry_id"`
	RevisionID     string `json:"revision_id"`
}

// ReportResult is the persisted run-level projection a get action returns.
type ReportResult struct {
	RunID      string                `json:"run_id"`
	Status     string                `json:"status"`
	Generation *GenerationProvenance `json:"generation,omitempty"`
}
