package cognitivecontract

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
)

// MainConversationBinding is the #58 consumer contract for #66's trusted
// singleton producer. SchemaVersion and Grant pin the actual host authorization
// decision; they are consumer additions pending producer agreement. No client
// field can create, convert, or select this policy.
type MainConversationBinding struct {
	SubjectID, DomainID, ConversationID string
	ActiveSessionID                     domain.SessionID
	PolicyID, ToolPolicyRef             string
	SchemaVersion                       int
	Grant                               storage.SubjectGrantSnapshot
}

type MainConversationResolver interface {
	ResolveMainConversation(context.Context) (MainConversationBinding, error)
}

// SubjectSourceResolver contributes explicitly host-authorized personal
// ordinary sources without giving them resident LIFE or whole-domain reads.
// Implementations use the existing Host Policy/Grant checks on every call;
// nil means no contribution. A persisted source cannot change scope or grants.
type SubjectSourceResolver func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error)
