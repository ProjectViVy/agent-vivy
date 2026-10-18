package cognitivecontract

import (
	"agent-vivy/internal/domain"
	"context"

	"github.com/ProjectViVy/laputa/laputa/actmem"
)

// Recap values are the owner's C1 contract, without a second DTO scheme.
type (
	RecapBinding = actmem.RecapBinding
	RecapHead    = actmem.RecapHead
	RecapCommit  = actmem.RecapCommit
	SourceRange  = actmem.SourceRange
)

// RecapPort is bound to the trusted complete subject sharing domain. Read
// parameters never select identity; Commit.Binding may only assert the already
// bound identity. This is a host control-plane port, not a worker/model grant.
type RecapPort interface {
	ReadRecap(context.Context) (RecapHead, error)
	CommitRecap(context.Context, RecapCommit) (RecapHead, error)
	LookupRecapOperation(context.Context, string) (RecapHead, bool, error)
	ListRecapHistory(context.Context, uint64, int) ([]RecapHead, error)
	ReadRecapRevision(context.Context, uint64) (RecapHead, error)
}

// RecapProvider is optional: existing cognitive Bundle implementations do not
// need to implement Recap. Without the trusted factory resolver, calls fail
// unavailable and ordinary persona/chat preparation remains unchanged.
type RecapProvider interface{ Recap() RecapPort }

// RecapManagementPort is an optional private extension of the already attached
// runtime control cell. Authorization rechecks the trusted singleton and, for
// tools, the persisted run pins and actual selected tool policy. Empty tool/run
// identifies a human ActionHost request; it never authorizes a model call.
type RecapManagementPort interface {
	AuthorizeRecap(context.Context, domain.SessionID, domain.RunID, string) (RecapBinding, error)
}

// RecapSource is an authorized C2 original expansion. Native memory indexes
// and Recap text are not transcript authorities. References are retained
// destination-owned snapshots, not fresh reads from another session.
type RecapSource struct {
	InteractionID  string
	SourceRevision uint64
	SessionID      domain.SessionID
	RunID          domain.RunID
	SourceDigest   string
	Outcome        domain.RunStatus
	Messages       []domain.Message
	Disposition    map[string]string
	References     []domain.ContextReference
	Corrections    []RecapSourceCorrection
}
type RecapSourceCorrection struct {
	ViewSessionID  domain.SessionID `json:"view_session_id"`
	Reason         string           `json:"reason"`
	MessageIDs     []string         `json:"message_ids"`
	ViewMessageIDs []string         `json:"view_message_ids"`
}

// RecapSourcesPort is optional and bound through the existing private runtime
// cell. The reader revalidates the historical revision and current grants.
type RecapSourcesPort interface {
	ReadRecapSources(context.Context, domain.SessionID, domain.RunID, string, uint64) ([]RecapSource, error)
}

// RecapLoaded is an observation of a model request admitted in this process,
// never an inference from the newest owner revision.
type RecapLoaded struct {
	Status            string       `json:"status"`
	RunID             domain.RunID `json:"run_id,omitempty"`
	Revision          uint64       `json:"revision"`
	InvalidationEpoch uint64       `json:"invalidation_epoch"`
}
type RecapLoadedPort interface {
	ReadRecapLoaded(context.Context, domain.SessionID) (RecapLoaded, error)
}
