package storage

import (
	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"context"
)

type BundleInstallation struct {
	InstallID     string                    `json:"install_id"`
	BundleID      string                    `json:"bundle_id"`
	State         string                    `json:"state"`
	Source        bundlecontract.SourceLock `json:"source"`
	ReleaseDigest string                    `json:"release_digest"`
	Revision      int64                     `json:"revision"`
	Tombstone     bool                      `json:"tombstone"`
}
type BundleState struct {
	Catalog            bundlecontract.CatalogIdentity `json:"catalog"`
	RevocationRevision int64                          `json:"revocation_revision"`
	Installations      []BundleInstallation           `json:"installations"`
	Releases           []bundlecontract.ReleaseRef    `json:"releases"`
	Pending            *bundlecontract.Operation      `json:"pending,omitempty"`
}
type InstallCommit struct {
	OperationID, InputDigest string
	Installation             BundleInstallation
	Release                  bundlecontract.ReleaseRef
	NowMS                    int64
}
type BeginOperation struct {
	ID, InputDigest, Kind string
	ExpectedEpoch         int64
}
type CancelOperation struct {
	ID               string
	ExpectedRevision int64
}
type BlockerQuery struct {
	Cursor string
	Limit  int
}
type BlockerPage struct {
	Items      []bundlecontract.Blocker
	NextCursor string
}
type InstallationState struct {
	InstallID, State, ReleaseDigest string
	Revision                        int64
	Tombstone                       bool
}
type ActivationCommit struct {
	OperationID, InputDigest, LeaseOwner string
	Expected                             bundlecontract.CatalogIdentity
	EffectiveDigest, GenerationID        string
	Installations                        []InstallationState
	Releases                             []bundlecontract.ReleaseRef
	RemovedMasks                         []string
	NowMS                                int64
}
type ConsentCommit struct {
	OperationID, InputDigest, LeaseOwner, InstallID, Kind, LocalID, DeclarationDigest, Subject string
	ExpectedEpoch, ExpectedRevision                                                            int64
	Allowed                                                                                    bool
	NowMS                                                                                      int64
}
type RevocationCommit struct {
	OperationID, InputDigest, LeaseOwner, InstallID, Kind, LocalID, DeclarationDigest, Subject string
	ExpectedEpoch, ExpectedRevision                                                            int64
	NowMS                                                                                      int64
}
type RunBundleSnapshot struct {
	RunID         domain.RunID
	SchemaVersion int
	Payload       []byte
	PayloadSHA256 string
	Releases      []bundlecontract.ReleaseRef
}
type ArtifactReachability struct{ Digests []string }

type BundleStore interface {
	ReadBundleState(context.Context) (BundleState, error)
	SaveBundleInstallation(context.Context, InstallCommit) (bundlecontract.Operation, error)
	BeginBundleOperation(context.Context, BeginOperation) (bundlecontract.Operation, error)
	GetBundleOperation(context.Context, string) (bundlecontract.Operation, error)
	CancelBundleOperation(context.Context, CancelOperation) (bundlecontract.Operation, error)
	ListBundleBlockers(context.Context, BlockerQuery) (BlockerPage, error)
	CommitBundleActivation(context.Context, ActivationCommit) (bundlecontract.Operation, error)
	CommitBundleConsent(context.Context, ConsentCommit) (bundlecontract.Operation, error)
	CommitBundleRevocation(context.Context, RevocationCommit) (bundlecontract.Operation, error)
	SaveRunBundleSnapshot(context.Context, RunBundleSnapshot) error
	LoadRunBundleSnapshot(context.Context, domain.RunID) (RunBundleSnapshot, error)
	ReadBundleReachability(context.Context) (ArtifactReachability, error)
}
