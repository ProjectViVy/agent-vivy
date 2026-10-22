package domain

// WorkflowRevision is the immutable host-validated descriptor admitted for
// one workflow Run. Descriptor bytes are canonical JSON; Journal events only
// carry its owner identity and bounded node projections.
type WorkflowRevision struct {
	RunID           RunID
	ParentRunID     RunID
	ParentSessionID SessionID
	RootRunID       RunID
	OperationKey    string
	DescriptorJSON  []byte
	AuthorityJSON   []byte
	SchemaVersion   int
	CreatedAt       int64
	// INOFY admission identity (schema_version 3). These columns are nullable
	// in storage; a revision admitted under discriminator 3 must carry every
	// field so a later recovery can rebind to the same immutable program.
	ProgramID       string
	CompilerVersion string
	EinoBuild       string
	// InputJSON is the canonical admitted run input (`{}` when the caller
	// supplied none); restart recovery re-executes the committed program with
	// exactly this payload.
	InputJSON       []byte
	EffectiveLimits []byte
	HostBindingID   string
	// DefinitionID/DefinitionRevision bind the admitted Run to its reusable
	// published definition (S11-F). Empty/0 = admitted directly from a raw
	// definition payload; revision 0 with a non-empty id = draft snapshot.
	DefinitionID       string
	DefinitionRevision uint64
}
