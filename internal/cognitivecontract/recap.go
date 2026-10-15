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
