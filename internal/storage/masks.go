package storage

import (
	"context"
	"encoding/json"
	"errors"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
)

// DecodeMaskBundleMetadata reconstructs copy provenance stored beside a
// mutable custom Mask. Empty installID means this is an ordinary custom Mask.
func DecodeMaskBundleMetadata(installID, bundleID, localID, releaseDigest string, releaseSequence int64, recommendationsJSON []byte) (*mask.BundleProvenance, []mask.SkillRecommendation, error) {
	if installID == "" {
		if bundleID != "" || localID != "" || releaseDigest != "" || releaseSequence != 0 || len(recommendationsJSON) != 0 {
			return nil, nil, errors.New("incomplete Mask copy metadata")
		}
		return nil, nil, nil
	}
	var recommendations []mask.SkillRecommendation
	if err := json.Unmarshal(recommendationsJSON, &recommendations); err != nil || recommendations == nil {
		return nil, nil, errors.New("invalid copied Mask recommendation metadata")
	}
	provenance := &mask.BundleProvenance{Release: bundlecontract.ReleaseRef{
		InstallID: installID, BundleID: bundleID, ReleaseDigest: releaseDigest, ReleaseSequence: releaseSequence,
	}, LocalID: localID}
	sourceID, err := bundlecontract.MaskID(installID, localID)
	if err != nil || mask.ValidateBundleProvenance(sourceID, releaseSequence, *provenance) != nil || mask.ValidateRecommendations(recommendations, provenance) != nil {
		return nil, nil, errors.New("invalid copied Mask provenance")
	}
	return provenance, recommendations, nil
}

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
	Origin             mask.Origin
	Bundle             *mask.BundleProvenance
	RecommendedSkills  []mask.SkillRecommendation
}

type PersonaSnapshot struct {
	Source   string
	Revision string
	Digest   string
	Body     string
}

// RunRuleBlock stores the exact authored bytes and release identity for a
// literal Toolkit Rule admitted into a run. Rules are guidance only; they do
// not carry executable authority.
type RunRuleBlock struct {
	Ref           bundlecontract.ComponentRef `json:"ref"`
	Release       bundlecontract.ReleaseRef   `json:"release"`
	ContentDigest string                      `json:"content_digest"`
	Body          string                      `json:"body"`
}

// RunRulesSnapshot is immutable prompt evidence. Body is the deterministic
// agentsmd projection of Blocks and Digest covers the face, source refs, and
// exact bytes. It is optional in the prompt payload for legacy runs.
type RunRulesSnapshot struct {
	Face   string         `json:"face"`
	Blocks []RunRuleBlock `json:"blocks"`
	Body   string         `json:"body"`
	Digest string         `json:"digest"`
}

type RunPromptPayload struct {
	Persona PersonaSnapshot
	Mask    *mask.Snapshot
	// Frozen is the session-frozen FrozenCore v2 projection admitted
	// into this run's instruction; nil when no bound bundle prepared it.
	Frozen        *PersonaSnapshot
	Instruction   string
	FramingDigest string
	Rules         *RunRulesSnapshot `json:",omitempty"`
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
	// Events follow run.started in the same admission transaction.
	Events  []domain.RunEvent
	Message domain.Message
	// OmitMessage admits an internal child activation whose task is carried in
	// its immutable prompt snapshot rather than the parent Session transcript.
	// It is valid only for one-shot child Runs.
	OmitMessage    bool
	Run            domain.Run
	Started        domain.RunEvent
	Prompt         *RunPromptSnapshot
	ExpectedMask   *MaskCaptureCheck
	Bundle         *RunBundleSnapshot
	ExpectedBundle *bundlecontract.CaptureCheck
	Edit           *SessionTruncation
}

type RunAdmissionStore interface {
	CommitRunAdmission(context.Context, RunAdmission) (domain.RunEvent, error)
	LoadRunPrompt(context.Context, domain.RunID) (RunPromptSnapshot, error)
}
