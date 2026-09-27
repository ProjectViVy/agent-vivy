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
}
