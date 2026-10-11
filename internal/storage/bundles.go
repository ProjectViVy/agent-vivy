package storage

import (
	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

var (
	ErrBundleRevoked         = errors.New("storage: bundle revoked")
	ErrBundleCaptureConflict = errors.New("storage: bundle catalog capture conflict")
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
	Bindings           []ToolkitBinding               `json:"bindings"`
	Pending            *bundlecontract.Operation      `json:"pending,omitempty"`
}
type ToolkitBinding struct {
	InstallID string `json:"install_id"`
	Kind      string `json:"kind"`
	LocalID   string `json:"local_id"`
	Name      string `json:"name"`
	Reference string `json:"reference"`
}
type BindingCommit struct {
	OperationID, InputDigest, LeaseOwner, Subject string
	Expected                                      bundlecontract.CatalogIdentity
	InstallID, Kind, LocalID                      string
	Bindings                                      []ToolkitBinding
	EffectiveDigest                               string
	NowMS                                         int64
}
type InstallCommit struct {
	OperationID, InputDigest string
	Expected                 bundlecontract.CatalogIdentity
	Installation             BundleInstallation
	Release                  bundlecontract.ReleaseRef
	NowMS                    int64
}
type BeginOperation struct {
	ID, InputDigest, Kind string
	ExpectedEpoch         int64
	Prepared              *bundlecontract.PreparedOperation
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
	ExpectedEpoch, ExpectedConfigRevision, ExpectedRevision                                    int64
	Allowed                                                                                    bool
	NowMS                                                                                      int64
}
type BundleConsent struct {
	InstallID         string `json:"install_id"`
	Kind              string `json:"kind"`
	LocalID           string `json:"local_id"`
	DeclarationDigest string `json:"declaration_digest"`
	Subject           string `json:"subject"`
	Allowed           bool   `json:"allowed"`
	Revision          int64  `json:"revision"`
	UpdatedAt         int64  `json:"updated_at"`
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

// ValidateRunBundleSnapshot verifies the versioned immutable envelope and its
// canonical catalog payload before any backend persists or returns it.
func ValidateRunBundleSnapshot(s RunBundleSnapshot) (bundlecontract.RunSnapshotPayload, error) {
	if s.RunID == "" || s.SchemaVersion != 1 || len(s.Payload) == 0 {
		return bundlecontract.RunSnapshotPayload{}, errors.New("storage: invalid toolkit snapshot envelope")
	}
	h := sha256.Sum256(s.Payload)
	if hex.EncodeToString(h[:]) != s.PayloadSHA256 {
		return bundlecontract.RunSnapshotPayload{}, errors.New("storage: toolkit snapshot digest mismatch")
	}
	payload, err := bundlecontract.DecodeRunSnapshotPayload(s.Payload)
	if err != nil {
		return bundlecontract.RunSnapshotPayload{}, err
	}
	if !sameReleaseRefs(payload.Releases, s.Releases) {
		return bundlecontract.RunSnapshotPayload{}, errors.New("storage: toolkit snapshot release references mismatch")
	}
	return payload, nil
}

func sameReleaseRefs(a, b []bundlecontract.ReleaseRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type ArtifactReachability struct{ Digests []string }

type BundleStore interface {
	CheckBundleLease(context.Context, string) error
	ReadBundleState(context.Context) (BundleState, error)
	SaveBundleInstallation(context.Context, InstallCommit) (bundlecontract.Operation, error)
	BeginBundleOperation(context.Context, BeginOperation) (bundlecontract.Operation, error)
	GetBundleOperation(context.Context, string) (bundlecontract.Operation, error)
	CancelBundleOperation(context.Context, CancelOperation) (bundlecontract.Operation, error)
	ListBundleBlockers(context.Context, BlockerQuery) (BlockerPage, error)
	CommitBundleActivation(context.Context, ActivationCommit) (bundlecontract.Operation, error)
	CommitBundleBindings(context.Context, BindingCommit) (bundlecontract.Operation, error)
	CommitBundleConsent(context.Context, ConsentCommit) (bundlecontract.Operation, error)
	ReadBundleConsent(context.Context, string, bundlecontract.ComponentKind, string) (BundleConsent, bool, error)
	CommitBundleRevocation(context.Context, RevocationCommit) (bundlecontract.Operation, error)
	SaveRunBundleSnapshot(context.Context, RunBundleSnapshot) error
	LoadRunBundleSnapshot(context.Context, domain.RunID) (RunBundleSnapshot, error)
	ReadBundleReachability(context.Context) (ArtifactReachability, error)
	CheckBundleRevocations(context.Context, bundlecontract.CaptureCheck) error
}
