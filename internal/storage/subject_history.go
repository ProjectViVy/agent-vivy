package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"agent-vivy/internal/domain"
)

var ErrSubjectBinding = errors.New("storage: subject binding mismatch")
var ErrSubjectEvidenceIncomplete = errors.New("storage: subject evidence incomplete")

// ErrSubjectEvidenceUnresolvable is an ErrSubjectEvidenceIncomplete whose gap can
// never close: a terminal run whose tool effects are unresolved or whose tool
// arguments are not valid UTF-8. Only this error permits an excluded seal.
var ErrSubjectEvidenceUnresolvable = fmt.Errorf("%w: terminal run has unresolvable tool evidence", ErrSubjectEvidenceIncomplete)
var ErrSubjectInteractionConflict = errors.New("storage: subject interaction conflict")
var ErrRetainedSession = errors.New("storage: retained subject source cannot be deleted")

type SubjectHistoryBinding struct{ SubjectID, DomainID string }

type SubjectInteraction struct {
	Sequence     uint64
	ID           string
	SessionID    domain.SessionID
	RunID        domain.RunID
	TerminalSeq  domain.EventSeq
	SourceDigest string
	// Disposition is empty for an included interaction, so receipts hashed over
	// included rows keep their bytes. An excluded interaction has no originals
	// and the digest of an empty original list.
	Disposition string `json:",omitempty"`
}

const SubjectInteractionExcluded = "excluded"

// SubjectHistoryStore is the metadata-only C2 index. Callers must authorize
// the entire fixed domain before a read; these internal bindings are not tokens.
// List takes (after, capturedThrough, limit); zero through is an empty captured
// prefix. CaptureSubjectHistory atomically reads the watermark for later pages.
type SubjectHistoryStore interface {
	SealSubjectInteraction(context.Context, SubjectHistoryBinding, SubjectInteraction) (SubjectInteraction, error)
	// SealSubjectExclusion seals a run whose evidence is unresolvable as an
	// excluded interaction. Storage re-verifies the condition; a healthy run is
	// refused with ErrSubjectEvidenceIncomplete.
	SealSubjectExclusion(context.Context, SubjectHistoryBinding, SubjectInteraction) (SubjectInteraction, error)
	ListSubjectInteractions(context.Context, SubjectHistoryBinding, uint64, uint64, int) ([]SubjectInteraction, error)
	ReadSubjectInteraction(context.Context, SubjectHistoryBinding, string) ([]domain.Message, error)
	ArchiveRetainedSession(context.Context, domain.SessionID) error
}

// SubjectGrantSnapshot is supplied by a trusted Host after its existing Policy
// and Grant checks. Snapshot is the actual immutable authorization decision;
// Reference/Version identify its authority. Contribution alone does not grant
// resident LIFE context or complete-domain reads.
type SubjectGrantSnapshot struct {
	Reference       string
	Version         uint64
	Snapshot        json.RawMessage
	WholeDomainRead bool
}

// SubjectSourceBinding is immutable admission evidence, never accepted from
// JSON or a model. Empty ConversationID is an ordinary contribution-only source.
type SubjectSourceBinding struct {
	SubjectHistoryBinding
	SessionID      domain.SessionID
	SchemaVersion  int
	ConversationID string
	PolicyID       string
	ToolPolicyRef  string
	Scope          string
	Grant          SubjectGrantSnapshot
}

func (s SubjectSourceBinding) Validate() error {
	var grant map[string]json.RawMessage
	if json.Unmarshal(s.Grant.Snapshot, &grant) != nil || len(grant) == 0 {
		return ErrSubjectBinding
	}
	if s.SubjectID == "" || s.DomainID == "" || s.SessionID == "" || s.SchemaVersion != 1 || s.Scope != "personal" || s.PolicyID == "" || s.ToolPolicyRef == "" || s.Grant.Reference == "" || s.Grant.Version == 0 || !json.Valid(s.Grant.Snapshot) || string(s.Grant.Snapshot) == "null" {
		return ErrSubjectBinding
	}
	if (s.ConversationID != "" && (s.PolicyID != domain.SubjectResidentPolicy || !s.Grant.WholeDomainRead)) || (s.ConversationID == "" && (s.PolicyID == domain.SubjectResidentPolicy || s.Grant.WholeDomainRead)) {
		return ErrSubjectBinding
	}
	return nil
}

// SameSubjectSourceBinding compares the same JSON representation used for
// persisted source/run pins. RawMessage formatting and HTML escaping must not
// turn an unchanged Host decision into a changed grant after storage reload.
// This only encodes authorization metadata; original user evidence is untouched.
func SameSubjectSourceBinding(a, b SubjectSourceBinding) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	return err == nil && bytes.Equal(left, right)
}

type SubjectSourceStore interface {
	SubjectHistoryStore
	ReadSubjectRunEvidence(context.Context, SubjectHistoryBinding, domain.RunID) ([]domain.Message, SubjectInteraction, error)
	GetSubjectSource(context.Context, domain.SessionID) (SubjectSourceBinding, error)
	GetRunSubjectSource(context.Context, domain.RunID) (SubjectSourceBinding, error)
	CaptureSubjectHistory(context.Context, SubjectHistoryBinding) (uint64, error)
	CaptureSubjectContext(context.Context, SubjectHistoryBinding) (SubjectContextSnapshot, error)
	ListSubjectPendingRuns(context.Context, SubjectHistoryBinding) ([]SubjectPendingRun, error)
}

func SubjectInteractionID(run domain.RunID, terminal domain.EventSeq) string {
	return fmt.Sprintf("%s:%d", run, terminal)
}

// SubjectEvidenceDigest commits all exact source fields, including attachment
// bytes and file snapshots. It never stores a second transcript.
func SubjectEvidenceDigest(messages []domain.Message) (string, error) {
	// JSON cannot represent invalid UTF-8 without replacing bytes. Refuse such
	// evidence rather than committing a digest of silently changed originals.
	for _, m := range messages {
		if !utf8.Valid(m.ToolArgs) {
			return "", ErrSubjectEvidenceIncomplete
		}
		fields := []string{m.ID, string(m.SessionID), string(m.RunID), string(m.Role), m.Content, m.ToolCallID, m.ToolName, m.ToolError, m.Source, m.Channel, m.ChatID, m.ChannelMessageID, m.ContentOrigin}
		if m.CopySource != nil {
			fields = append(fields, string(m.CopySource.SessionID), m.CopySource.MessageID)
		}
		for _, a := range m.Attachments {
			fields = append(fields, a.Name, a.MimeType)
		}
		for _, f := range m.FileContexts {
			fields = append(fields, f.Path, f.Name)
		}
		for _, field := range fields {
			if !utf8.ValidString(field) {
				return "", ErrSubjectEvidenceIncomplete
			}
		}
	}

	raw, err := json.Marshal(messages)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// SubjectContextSnapshot is a complete metadata cut. Through remains the
// terminal-interaction ceiling; DispositionDigest is not a progress cursor.
type SubjectContextSnapshot struct {
	// Sources is the complete distinct domain membership from this same SQL cut.
	// It is current-authorization input, not a disposition acknowledgement.
	Sources           []SubjectSourceBinding
	Through           uint64
	Dispositions      []SubjectDisposition
	DispositionDigest string
}
type SubjectDisposition struct {
	SessionID domain.SessionID
	MarkerID  uint64
	Marker    SessionTruncation
	Targets   []SubjectDispositionTarget
}
type SubjectDispositionTarget struct {
	InteractionID   string
	Sequence        uint64
	SourceSessionID domain.SessionID
	RunID           domain.RunID
	SourceDigest    string
	MessageIDs      []string
	ViewMessageIDs  []string
}

// SubjectPendingRun is a terminal, unsealed coordinate, never body evidence.
type SubjectPendingRun struct {
	SessionID   domain.SessionID
	RunID       domain.RunID
	TerminalSeq domain.EventSeq
}
