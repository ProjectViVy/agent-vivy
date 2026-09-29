package domain

// WorkflowRevision is the immutable host-validated descriptor admitted for
// one workflow Run. Descriptor bytes are canonical JSON; Journal events only
// carry its digest and bounded node projections.
type WorkflowRevision struct {
	RunID            RunID
	ParentRunID      RunID
	ParentSessionID  SessionID
	RootRunID        RunID
	OperationKey     string
	DescriptorDigest string
	AuthorityDigest  string
	DescriptorJSON   []byte
	AuthorityJSON    []byte
	SchemaVersion    int
	CreatedAt        int64
	// INOFY admission identity (schema_version 2). These columns are nullable
	// in storage; a revision admitted under discriminator 2 must carry every
	// field so a later recovery can rebind to the same immutable program.
	ProgramDigest   string
	CatalogDigest   string
	CompilerVersion string
	EinoBuild       string
	InputDigest     string
	EffectiveLimits []byte
	HostBindingID   string
}
