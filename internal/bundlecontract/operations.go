package bundlecontract

// Operation is durable management evidence, independent from UI request state.
type Operation struct {
	ID             string             `json:"id"`
	Kind           string             `json:"kind"`
	State          string             `json:"state"`
	Revision       int64              `json:"revision"`
	ExpectedEpoch  int64              `json:"expected_epoch"`
	CommittedEpoch int64              `json:"committed_epoch"`
	ErrorCode      string             `json:"error_code,omitempty"`
	Retryable      bool               `json:"retryable"`
	Progress       string             `json:"progress,omitempty"`
	Blockers       []Blocker          `json:"blockers"`
	Prepared       *PreparedOperation `json:"prepared,omitempty"`
}

// PreparedOperation is the durable, redacted review receipt for a catalog
// candidate. It contains content identities and state transitions only; no
// source credentials, local secret values, or executable payloads are stored.
type PreparedOperation struct {
	InputDigest              string               `json:"input_digest"`
	Expected                 CatalogIdentity      `json:"expected_catalog"`
	CandidateDigest          string               `json:"candidate_digest"`
	Releases                 []ReleaseRef         `json:"releases"`
	NewReleases              []ReleaseRef         `json:"new_releases"`
	Installations            []InstallationChange `json:"installations"`
	Changes                  []ComponentChange    `json:"changes"`
	MutationKind             string               `json:"mutation_kind"`
	MutationInstallID        string               `json:"mutation_install_id"`
	MutationReleaseDigest    string               `json:"mutation_release_digest,omitempty"`
	MutationExpectedRevision int64                `json:"mutation_expected_revision,omitempty"`
}

type ComponentChange struct {
	Kind                 string `json:"kind"`
	LocalID              string `json:"local_id"`
	Change               string `json:"change"`
	BeforeDigest         string `json:"before_digest,omitempty"`
	AfterDigest          string `json:"after_digest,omitempty"`
	RequiresFreshConsent bool   `json:"requires_fresh_consent,omitempty"`
}

type InstallationChange struct {
	InstallID     string `json:"install_id"`
	State         string `json:"state"`
	ReleaseDigest string `json:"release_digest"`
	Revision      int64  `json:"revision"`
	Tombstone     bool   `json:"tombstone"`
}
type Blocker struct {
	RunID     string `json:"run_id"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}
