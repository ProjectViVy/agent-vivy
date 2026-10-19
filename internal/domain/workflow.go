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
	// RootPurpose marks trusted workflows admitted outside the child lane
	// (e.g. report/v1). Empty means a child workflow admitted under a parent
	// Run; the parent lineage columns stay authoritative for it.
	RootPurpose string
	// AdmissionNamespace is the collision domain of OperationKey: the parent
	// Run ID for child workflows, the hidden control Session ID for trusted
	// report roots. Legacy rows carry ParentRunID.
	AdmissionNamespace string
	// RequestDigest is the canonical digest of the caller's stable semantic
	// request (period + window + target). Replaying an operation key with a
	// different request is a revision conflict.
	RequestDigest string
	// TargetKey scopes active-run exclusivity: a distinct operation key
	// targeting the same window while that Run is non-terminal returns the
	// existing Run as busy.
	TargetKey string
	// INOFY admission identity (schema_version 2). These columns are nullable
	// in storage; a revision admitted under discriminator 2 must carry every
	// field so a later recovery can rebind to the same immutable program.
	ProgramDigest   string
	CatalogDigest   string
	CompilerVersion string
	EinoBuild       string
	InputDigest     string
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
