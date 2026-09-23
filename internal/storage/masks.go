package storage

import (
	"context"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
)

// MaskStore is the focused Core Storage extension for custom definitions,
// session selections, and atomic capture. It carries no tenant/scope input;
// the backend owns organism scope and transaction boundaries.
type MaskStore interface {
	ListCustomMasks(context.Context, mask.ListRequest) (mask.Page, error)
	GetCustomMask(context.Context, string) (mask.Definition, error)
	CreateCustomMask(context.Context, mask.CreateRequest) (mask.Definition, error)
	UpdateCustomMask(context.Context, mask.UpdateRequest) (mask.Definition, error)
	DeleteCustomMask(context.Context, mask.DeleteRequest) (mask.DeleteResult, error)
	ReadMaskCapture(context.Context, domain.SessionID) (mask.Capture, error)
	SetMaskSelection(context.Context, mask.SetSelectionRequest) (mask.Selection, error)
}

// MaskCaptureCheck is the admission CAS input. Definition revision and digest
// apply to custom definitions; built-ins and the empty state use zero values.
type MaskCaptureCheck struct {
	SessionID          domain.SessionID
	SelectionRevision  int64
	MaskID             string
	DefinitionRevision int64
	DefinitionDigest   string
}

type PersonaSnapshot struct {
	Source   string
	Revision string
	Digest   string
	Body     string
}

type RunPromptPayload struct {
	Persona       PersonaSnapshot
	Mask          *mask.Snapshot
	Instruction   string
	FramingDigest string
}

type RunPromptSnapshot struct {
	RunID           domain.RunID
	SchemaVersion   int
	ComposerVersion string
	GenerationID    string
	Payload         []byte
	PayloadSHA256   string
}

type RunAdmission struct {
	Message      domain.Message
	Run          domain.Run
	Started      domain.RunEvent
	Prompt       *RunPromptSnapshot
	ExpectedMask *MaskCaptureCheck
	Edit         *SessionTruncation
}

type RunAdmissionStore interface {
	CommitRunAdmission(context.Context, RunAdmission) (domain.RunEvent, error)
	LoadRunPrompt(context.Context, domain.RunID) (RunPromptSnapshot, error)
}
