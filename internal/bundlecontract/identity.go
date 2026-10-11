package bundlecontract

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var localIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,23}$`)
var bundleIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}/[a-z0-9][a-z0-9-]{0,31}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ValidInstallID(id string) bool  { return uuidPattern.MatchString(id) }
func ValidLocalID(id string) bool    { return localIDPattern.MatchString(id) }
func ValidBundleID(id string) bool   { return bundleIDPattern.MatchString(id) }
func ValidDigest(digest string) bool { return validDigest(digest) }
func SkillID(installID, localID string) (string, error) {
	if !ValidInstallID(installID) || !localIDPattern.MatchString(localID) {
		return "", Invalid("component_identity")
	}
	return "b_" + strings.ReplaceAll(installID, "-", "") + "_" + localID, nil
}
func MCPInstanceID(installID, localID string) (string, error) { return SkillID(installID, localID) }
func MaskID(installID, localID string) (string, error) {
	if _, err := SkillID(installID, localID); err != nil {
		return "", err
	}
	return "bundle/" + installID + "/" + localID, nil
}

type ComponentKind string

const (
	Skill ComponentKind = "skill"
	Mask  ComponentKind = "mask"
	Rule  ComponentKind = "rule"
	Hook  ComponentKind = "hook"
	MCP   ComponentKind = "mcp"
)

type ComponentRef struct {
	InstallID string        `json:"install_id"`
	Kind      ComponentKind `json:"kind"`
	LocalID   string        `json:"local_id"`
}
type ReleaseRef struct {
	InstallID       string `json:"install_id"`
	BundleID        string `json:"bundle_id"`
	ReleaseDigest   string `json:"release_digest"`
	ReleaseSequence int64  `json:"release_sequence"`
}
type CatalogIdentity struct {
	Epoch           int64  `json:"epoch"`
	ConfigRevision  int64  `json:"config_revision"`
	EffectiveDigest string `json:"effective_digest"`
	GenerationID    string `json:"generation_id"`
}
type CaptureCheck struct {
	Catalog            CatalogIdentity `json:"catalog"`
	RevocationRevision int64           `json:"revocation_revision"`
	Executables        []ExecutableRef `json:"executables"`
	Consents           []ConsentRef    `json:"consents,omitempty"`
}

// ExecutableRef is the exact declaration a captured run is allowed to
// encounter. Runtime fences compare these identities against revocations
// newer than the run's captured revision; unrelated revocations do not stop
// the run.
type ExecutableRef struct {
	ComponentRef
	DeclarationDigest string `json:"declaration_digest"`
	ConsentDigest     string `json:"consent_digest,omitempty"`
	ConsentRevision   int64  `json:"consent_revision,omitempty"`
}

// ConsentRef is the exact current Hook consent row observed while preparing
// a run snapshot. Present=false captures the absence of consent so an
// arming race cannot silently change the run's guard chain after admission.
type ConsentRef struct {
	ComponentRef
	Present           bool   `json:"present"`
	DeclarationDigest string `json:"declaration_digest,omitempty"`
	Allowed           bool   `json:"allowed,omitempty"`
	Revision          int64  `json:"revision,omitempty"`
}

// ExecutableConsentDigest binds human consent to one installed release's
// executable declaration and the effective local configuration revision.
// Secret values never enter the digest.
func ExecutableConsentDigest(installID, releaseDigest, declarationDigest string, configRevision int64) (string, error) {
	if !ValidInstallID(installID) || !validDigest(releaseDigest) || !validDigest(declarationDigest) || configRevision < 0 {
		return "", Invalid("consent_identity")
	}
	canonical, err := CanonicalJSON(struct {
		InstallID         string `json:"install_id"`
		ReleaseDigest     string `json:"release_digest"`
		DeclarationDigest string `json:"declaration_digest"`
		ConfigRevision    int64  `json:"config_revision"`
	}{installID, releaseDigest, declarationDigest, configRevision})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

type SourceSpec struct {
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Ref      string `json:"ref,omitempty"`
}
type SourceLock struct {
	Kind              string `json:"kind"`
	Origin            string `json:"origin"`
	RequestedRef      string `json:"requested_ref,omitempty"`
	ResolvedCommit    string `json:"resolved_commit,omitempty"`
	AcquisitionDigest string `json:"acquisition_digest,omitempty"`
	ValidatorVersion  string `json:"validator_version"`
}
type InventoryEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
