package bundlecontract

import (
	"regexp"
	"strings"
)

var localIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,23}$`)
var bundleIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}/[a-z0-9][a-z0-9-]{0,31}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ValidInstallID(id string) bool { return uuidPattern.MatchString(id) }
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
