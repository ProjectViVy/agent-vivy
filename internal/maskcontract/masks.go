// Package maskcontract contains the kernel-owned values shared by the mask
// service, Runtime, and Core Storage. It deliberately has no provider, SQL,
// RPC, or prompt execution dependencies.
package maskcontract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/controlaction"
)

const (
	MaxBodyBytes        = 16 << 10
	MaxNameBytes        = 128
	MaxDescriptionBytes = 1024
	MaxIDBytes          = 128

	DefaultListLimit = 50
	MaxListLimit     = 100

	BuiltinProgrammerID = "builtin/programmer"
	BuiltinResearcherID = "builtin/researcher"
	BuiltinWriterID     = "builtin/writer"
	BuiltinRevision     = int64(1)
)

// Error codes are the only mask failures that may cross the ActionHost/RPC
// boundary. The Error type intentionally does not expose its cause text.
const (
	CodeInvalidMask         = "invalid_mask"
	CodeNotFound            = "not_found"
	CodeRevisionConflict    = "revision_conflict"
	CodeMaskInUse           = "mask_in_use"
	CodeMaskUnavailable     = "mask_unavailable"
	CodeReadOnlyComponent   = "read_only_component"
	CodeSnapshotMissing     = "snapshot_missing"
	CodeSnapshotCorrupt     = "snapshot_corrupt"
	CodePromptTooLarge      = "prompt_too_large"
	CodeIncompatiblePrompt  = "incompatible_prompt_version"
	CodeAuthorizationDenied = "authorization_denied"
	CodeCancelled           = "cancelled"
	CodeUnavailable         = "unavailable"
)

var allowedErrorCodes = map[string]struct{}{
	CodeInvalidMask:         {},
	CodeNotFound:            {},
	CodeRevisionConflict:    {},
	CodeMaskInUse:           {},
	CodeMaskUnavailable:     {},
	CodeReadOnlyComponent:   {},
	CodeSnapshotMissing:     {},
	CodeSnapshotCorrupt:     {},
	CodePromptTooLarge:      {},
	CodeIncompatiblePrompt:  {},
	CodeAuthorizationDenied: {},
	CodeCancelled:           {},
	CodeUnavailable:         {},
}

// Error carries safe, structured mask failure metadata. The underlying cause
// is retained for local diagnostics and errors.Is/errors.As, but is never
// included in Error's string representation.
type Error struct {
	Code            string
	CurrentRevision int64
	ReferenceCount  int
	cause           error
}

func (e *Error) Error() string {
	if e == nil {
		return "mask: <nil>"
	}
	code := e.Code
	if !IsErrorCode(code) {
		code = CodeInvalidMask
	}
	message := "mask: " + code
	if e.CurrentRevision > 0 {
		message += fmt.Sprintf(" (current_revision=%d)", e.CurrentRevision)
	}
	if e.ReferenceCount > 0 {
		message += fmt.Sprintf(" (reference_count=%d)", e.ReferenceCount)
	}
	return message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// NewError creates a safe typed error. Unknown codes are reduced to
// invalid_mask so an accidental internal string cannot become a new wire
// vocabulary.
func NewError(code string, cause error) *Error {
	if !IsErrorCode(code) {
		code = CodeInvalidMask
	}
	return &Error{Code: code, cause: cause}
}

// NewRevisionError creates a safe error with the current definition or
// selection revision used by a conflict response.
func NewRevisionError(code string, revision int64, cause error) *Error {
	err := NewError(code, cause)
	if revision > 0 {
		err.CurrentRevision = revision
	}
	return err
}

// NewReferenceError creates a safe error with the number of sessions that
// reference a definition.
func NewReferenceError(code string, references int, cause error) *Error {
	err := NewError(code, cause)
	if references > 0 {
		err.ReferenceCount = references
	}
	return err
}

// IsErrorCode reports whether code belongs to the allowlisted mask vocabulary.
func IsErrorCode(code string) bool {
	_, ok := allowedErrorCodes[code]
	return ok
}

// Selection is the durable per-session mask choice. An empty MaskID is the
// explicit unmasked state; Revision remains meaningful for CAS.
type Selection struct {
	SessionID domain.SessionID `json:"session_id"`
	MaskID    string           `json:"mask_id"`
	Revision  int64            `json:"revision"`
}

type Origin string

const (
	OriginBuiltin Origin = "builtin"
	OriginCustom  Origin = "custom"
	OriginBundle  Origin = "bundle"
)

// BundleProvenance identifies the exact immutable Toolkit Mask definition.
// The release sequence is the definition revision; the content digest remains
// DefinitionDigest's existing ID/name/body digest.
type BundleProvenance struct {
	Release bundlecontract.ReleaseRef `json:"release"`
	LocalID string                    `json:"local_id"`
}

// SkillRecommendation is discovery metadata only. Available describes the
// target's independent Host state; it never grants Skill activation.
type SkillRecommendation struct {
	Ref            bundlecontract.ComponentRef `json:"ref"`
	SkillID        string                      `json:"skill_id"`
	Name           string                      `json:"name"`
	Available      bool                        `json:"available"`
	InactiveReason string                      `json:"inactive_reason,omitempty"`
}

// Definition is one catalog entry. Built-ins are immutable and have a
// GenerationID; custom rows use a server-generated custom/<uuid> ID.
type Definition struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	Description       string                `json:"description"`
	Body              string                `json:"body"`
	Revision          int64                 `json:"revision"`
	Digest            string                `json:"digest"`
	BuiltIn           bool                  `json:"built_in"`
	GenerationID      string                `json:"generation_id"`
	Origin            Origin                `json:"origin,omitempty"`
	Bundle            *BundleProvenance     `json:"bundle,omitempty"`
	RecommendedSkills []SkillRecommendation `json:"recommended_skills,omitempty"`
}

// Snapshot is the immutable definition captured for one admitted run.
type Snapshot struct {
	ID                 string                `json:"id"`
	Name               string                `json:"name"`
	Body               string                `json:"body"`
	Digest             string                `json:"digest"`
	DefinitionRevision int64                 `json:"definition_revision"`
	SelectionRevision  int64                 `json:"selection_revision"`
	GenerationID       string                `json:"generation_id"`
	Origin             Origin                `json:"origin,omitempty"`
	Bundle             *BundleProvenance     `json:"bundle,omitempty"`
	RecommendedSkills  []SkillRecommendation `json:"recommended_skills,omitempty"`
}

// ViewResolver captures bundle Masks from the same admitted Toolkit View as
// the run. Existing callers keep using Resolver for legacy/custom Masks.
type ViewResolver interface {
	CaptureFromView(context.Context, domain.SessionID, bundlecontract.View) (Capture, error)
}

// Capture is the service's owned-copy read of a session selection and its
// selected definition. A nil Mask means explicitly unmasked.
type Capture struct {
	Selection Selection `json:"selection"`
	Mask      *Snapshot `json:"mask"`
}

// Resolver is the narrow runtime-facing service seam.
type Resolver interface {
	Capture(context.Context, domain.SessionID) (Capture, error)
}

type ListRequest struct {
	AfterID string `json:"after_id"`
	Limit   int    `json:"limit"`
}

type Metadata struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	Description       string                `json:"description"`
	Digest            string                `json:"digest"`
	GenerationID      string                `json:"generation_id"`
	Revision          int64                 `json:"revision"`
	BuiltIn           bool                  `json:"built_in"`
	Origin            Origin                `json:"origin,omitempty"`
	Bundle            *BundleProvenance     `json:"bundle,omitempty"`
	RecommendedSkills []SkillRecommendation `json:"recommended_skills,omitempty"`
	Available         bool                  `json:"available"`
	InactiveReason    string                `json:"inactive_reason,omitempty"`
}

type Page struct {
	Items        []Metadata `json:"items"`
	NextAfterID  string     `json:"next_after_id"`
	CatalogEpoch int64      `json:"catalog_epoch,omitempty"`
}

type GetRequest struct {
	ID string `json:"id"`
}

type CreateRequest struct {
	OperationID string `json:"operation_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	// Bundle and RecommendedSkills are internal copy metadata. Public create
	// actions cannot set them directly; CopyMask populates them after resolving
	// the source from a pinned current Toolkit View.
	Bundle            *BundleProvenance     `json:"-"`
	RecommendedSkills []SkillRecommendation `json:"-"`
}

type CopyMaskRequest struct {
	OperationID          string `json:"operation_id"`
	SourceID             string `json:"source_id"`
	ExpectedCatalogEpoch int64  `json:"expected_catalog_epoch"`
}

type UpdateRequest struct {
	ID               string `json:"id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Body             string `json:"body"`
}

type DeleteRequest struct {
	ID               string `json:"id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type DeleteResult struct {
	ID string `json:"id"`
}

type SelectionRequest struct {
	SessionID domain.SessionID `json:"session_id"`
}

type SetSelectionRequest struct {
	SessionID            domain.SessionID `json:"session_id"`
	MaskID               string           `json:"mask_id"`
	ExpectedRevision     int64            `json:"expected_revision"`
	ExpectedCatalogEpoch int64            `json:"expected_catalog_epoch,omitempty"`
}

type SelectionView struct {
	Selection      Selection `json:"selection"`
	Available      bool      `json:"available"`
	InactiveReason string    `json:"inactive_reason"`
}

// Manager is the authenticated control-plane surface. Runtime should use
// Resolver, while ActionHost receives this interface through a scoped facade.
type Manager interface {
	ListMasks(context.Context, ListRequest) (Page, error)
	GetMask(context.Context, GetRequest) (Definition, error)
	CreateMask(context.Context, CreateRequest) (Definition, error)
	CopyMask(context.Context, CopyMaskRequest) (Definition, error)
	UpdateMask(context.Context, UpdateRequest) (Definition, error)
	DeleteMask(context.Context, DeleteRequest) (DeleteResult, error)
	GetMaskSelection(context.Context, SelectionRequest) (SelectionView, error)
	SetMaskSelection(context.Context, SetSelectionRequest) (SelectionView, error)
}

// Service is both the runtime resolver and the control-plane manager. Prompt
// assets are immutable provider data and are returned separately from capture.
type Service interface {
	Resolver
	Manager
	PromptAssets() (frame string, digest string)
}

// ActionHost is the extra private facade made available only to the sealed T1
// mask action owner.
type ActionHost interface {
	controlaction.Host
	Manager
}

// NormalizeCreate validates and canonicalizes one custom definition request.
// The body is only line-ending-normalized; Markdown/template-looking text is
// otherwise preserved literally.
func NormalizeCreate(in CreateRequest) (CreateRequest, error) {
	out := in
	if err := validateUUID(in.OperationID); err != nil {
		return CreateRequest{}, validationError(err)
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return CreateRequest{}, validationError(err)
	}
	description, err := normalizeDescription(in.Description)
	if err != nil {
		return CreateRequest{}, validationError(err)
	}
	body, err := normalizeBody(in.Body)
	if err != nil {
		return CreateRequest{}, validationError(err)
	}
	out.Name = name
	out.Description = description
	out.Body = body
	if in.Bundle != nil {
		sourceID, sourceErr := bundlecontract.MaskID(in.Bundle.Release.InstallID, in.Bundle.LocalID)
		if sourceErr != nil || ValidateBundleProvenance(sourceID, in.Bundle.Release.ReleaseSequence, *in.Bundle) != nil {
			return CreateRequest{}, validationError(errors.New("copy provenance is invalid"))
		}
		out.Bundle = cloneBundleProvenance(in.Bundle)
	}
	if len(in.RecommendedSkills) > 0 && in.Bundle == nil {
		return CreateRequest{}, validationError(errors.New("recommendations require copy provenance"))
	}
	if err := ValidateRecommendations(in.RecommendedSkills, in.Bundle); err != nil {
		return CreateRequest{}, validationError(err)
	}
	out.RecommendedSkills = cloneRecommendations(in.RecommendedSkills)
	return out, nil
}

func NormalizeCopy(in CopyMaskRequest) (CopyMaskRequest, error) {
	if err := validateUUID(in.OperationID); err != nil {
		return CopyMaskRequest{}, validationError(err)
	}
	if !IsBundleID(in.SourceID) || in.ExpectedCatalogEpoch <= 0 {
		return CopyMaskRequest{}, validationError(errors.New("copy requires a bundle Mask ID and positive expected catalog epoch"))
	}
	return in, nil
}

// NormalizeUpdate validates and canonicalizes a custom definition update.
// Built-in IDs are reserved and cannot be updated through this request.
func NormalizeUpdate(in UpdateRequest) (UpdateRequest, error) {
	if err := ValidateCustomID(in.ID); err != nil {
		return UpdateRequest{}, validationError(err)
	}
	if in.ExpectedRevision <= 0 {
		return UpdateRequest{}, validationError(errors.New("expected revision must be positive"))
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return UpdateRequest{}, validationError(err)
	}
	description, err := normalizeDescription(in.Description)
	if err != nil {
		return UpdateRequest{}, validationError(err)
	}
	body, err := normalizeBody(in.Body)
	if err != nil {
		return UpdateRequest{}, validationError(err)
	}
	out := in
	out.Name = name
	out.Description = description
	out.Body = body
	return out, nil
}

// NormalizeDelete validates a custom delete request.
func NormalizeDelete(in DeleteRequest) (DeleteRequest, error) {
	if err := ValidateCustomID(in.ID); err != nil {
		return DeleteRequest{}, validationError(err)
	}
	if in.ExpectedRevision <= 0 {
		return DeleteRequest{}, validationError(errors.New("expected revision must be positive"))
	}
	return in, nil
}

// NormalizeList validates cursor pagination and applies MASK-C1's default.
func NormalizeList(in ListRequest) (ListRequest, error) {
	if in.AfterID != "" {
		if err := ValidateMaskID(in.AfterID); err != nil {
			return ListRequest{}, validationError(err)
		}
	}
	if in.Limit < 0 || in.Limit > MaxListLimit {
		return ListRequest{}, validationError(errors.New("limit must be between 1 and 100, or zero for the default"))
	}
	out := in
	if out.Limit == 0 {
		out.Limit = DefaultListLimit
	}
	return out, nil
}

// NormalizeSelection validates a session selection mutation. ExpectedRevision
// may be zero for an uninitialized selection.
func NormalizeSelection(in SetSelectionRequest) (SetSelectionRequest, error) {
	if err := ValidateSessionID(in.SessionID); err != nil {
		return SetSelectionRequest{}, validationError(err)
	}
	if err := ValidateSelectionMaskID(in.MaskID); err != nil {
		return SetSelectionRequest{}, validationError(err)
	}
	if in.ExpectedRevision < 0 {
		return SetSelectionRequest{}, validationError(errors.New("expected selection revision must not be negative"))
	}
	if in.ExpectedCatalogEpoch < 0 || (IsBundleID(in.MaskID) && in.ExpectedCatalogEpoch <= 0) {
		return SetSelectionRequest{}, validationError(errors.New("bundle selection requires a positive expected catalog epoch"))
	}
	return in, nil
}

// CreateRequestDigest returns the canonical SHA-256 identity of the original
// request content. OperationID is intentionally excluded; description remains
// part of create idempotency even though it is display-only in a definition
// digest.
func CreateRequestDigest(in CreateRequest) string {
	if in.Bundle != nil || len(in.RecommendedSkills) > 0 {
		request := struct {
			Name              string                `json:"name"`
			Description       string                `json:"description"`
			Body              string                `json:"body"`
			Bundle            *BundleProvenance     `json:"bundle"`
			RecommendedSkills []SkillRecommendation `json:"recommended_skills"`
		}{strings.TrimSpace(in.Name), in.Description, normalizeLineEndings(in.Body), cloneBundleProvenance(in.Bundle), cloneRecommendations(in.RecommendedSkills)}
		return digestJSON(request)
	}
	request := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Body        string `json:"body"`
	}{
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		Body:        normalizeLineEndings(in.Body),
	}
	return digestJSON(request)
}

func cloneBundleProvenance(in *BundleProvenance) *BundleProvenance {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneRecommendations(in []SkillRecommendation) []SkillRecommendation {
	if in == nil {
		return nil
	}
	return append([]SkillRecommendation{}, in...)
}

// DefinitionDigest returns the canonical SHA-256 identity of the semantic
// definition fields. Description and revision are deliberately excluded.
func DefinitionDigest(id, name, body string) string {
	definition := struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Body string `json:"body"`
	}{
		ID:   id,
		Name: strings.TrimSpace(name),
		Body: normalizeLineEndings(body),
	}
	return digestJSON(definition)
}

func digestJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		// The structs passed here contain only strings, so this is unreachable.
		// Keep a deterministic value rather than returning an untyped error from
		// a helper whose contract is a string.
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// ValidateMaskID accepts a built-in, custom, or exact Toolkit Mask ID.
func ValidateMaskID(id string) error {
	if len(id) == 0 || len(id) > MaxIDBytes || !isASCII(id) {
		return errors.New("mask id must be non-empty ASCII of at most 128 bytes")
	}
	if IsBuiltinID(id) {
		return nil
	}
	if IsBundleID(id) {
		return nil
	}
	return ValidateCustomID(id)
}

// IsBundleID recognizes the canonical bundle/<install-uuid>/<local-id> form.
func IsBundleID(id string) bool {
	parts := strings.Split(id, "/")
	if len(parts) != 3 || parts[0] != "bundle" || !bundlecontract.ValidInstallID(parts[1]) || !bundlecontract.ValidLocalID(parts[2]) {
		return false
	}
	want, err := bundlecontract.MaskID(parts[1], parts[2])
	return err == nil && want == id
}

// ResolveOrigin derives the legacy origin from the ID only when the persisted
// record predates explicit origin metadata.
func ResolveOrigin(id string, origin Origin) Origin {
	if origin != "" {
		return origin
	}
	if IsBuiltinID(id) {
		return OriginBuiltin
	}
	if IsBundleID(id) {
		return OriginBundle
	}
	return OriginCustom
}

// ValidateCustomID rejects built-in IDs and accepts only custom/<uuid>.
func ValidateCustomID(id string) error {
	if len(id) == 0 || len(id) > MaxIDBytes || !isASCII(id) {
		return errors.New("custom mask id must be non-empty ASCII of at most 128 bytes")
	}
	if IsBuiltinID(id) {
		return errors.New("built-in mask id is reserved")
	}
	if !strings.HasPrefix(id, "custom/") || !validateUUIDText(strings.TrimPrefix(id, "custom/")) {
		return errors.New("custom mask id must be custom/<uuid>")
	}
	return nil
}

// ValidateSelectionMaskID accepts the empty unmasked state in addition to a
// built-in or custom ID.
func ValidateSelectionMaskID(id string) error {
	if id == "" {
		return nil
	}
	return ValidateMaskID(id)
}

func ValidateSessionID(id domain.SessionID) error {
	if len(id) == 0 || len(id) > MaxIDBytes || !isASCII(string(id)) {
		return errors.New("session id must be non-empty ASCII of at most 128 bytes")
	}
	return nil
}

func ValidateSelection(selection Selection) error {
	if err := ValidateSessionID(selection.SessionID); err != nil {
		return err
	}
	if err := ValidateSelectionMaskID(selection.MaskID); err != nil {
		return err
	}
	if selection.Revision < 0 {
		return errors.New("selection revision must not be negative")
	}
	return nil
}

// ValidateDefinition checks stored/embedded definition invariants, including
// explicit origin metadata and the legacy ID/name/body digest.
func ValidateDefinition(definition Definition) error {
	if err := ValidateMaskID(definition.ID); err != nil {
		return err
	}
	origin := definition.Origin
	if origin == "" {
		if definition.BuiltIn {
			origin = OriginBuiltin
		} else {
			origin = OriginCustom
		}
	}
	if definition.BuiltIn != (origin == OriginBuiltin) || definition.BuiltIn != IsBuiltinID(definition.ID) {
		return errors.New("built-in flag does not match reserved id")
	}
	if origin != OriginBuiltin && origin != OriginCustom && origin != OriginBundle {
		return errors.New("mask origin is invalid")
	}
	if definition.Revision <= 0 {
		return errors.New("definition revision must be positive")
	}
	switch origin {
	case OriginBuiltin:
		if definition.Revision != BuiltinRevision || definition.GenerationID == "" || definition.Bundle != nil {
			return errors.New("built-in mask origin metadata is invalid")
		}
	case OriginCustom:
		if ValidateCustomID(definition.ID) != nil || definition.GenerationID != "" {
			return errors.New("custom mask origin metadata is invalid")
		}
		if definition.Bundle != nil {
			sourceID, err := bundlecontract.MaskID(definition.Bundle.Release.InstallID, definition.Bundle.LocalID)
			if err != nil || ValidateBundleProvenance(sourceID, definition.Bundle.Release.ReleaseSequence, *definition.Bundle) != nil {
				return errors.New("custom mask copy provenance is invalid")
			}
		}
	case OriginBundle:
		if !IsBundleID(definition.ID) || definition.BuiltIn || definition.GenerationID != "" || definition.Bundle == nil {
			return errors.New("bundle mask origin metadata is invalid")
		}
		if err := ValidateBundleProvenance(definition.ID, definition.Revision, *definition.Bundle); err != nil {
			return err
		}
	}
	if len(definition.RecommendedSkills) > 0 && definition.Bundle == nil {
		return errors.New("recommendations require bundle provenance")
	}
	if err := ValidateRecommendations(definition.RecommendedSkills, definition.Bundle); err != nil {
		return err
	}
	name, err := normalizeName(definition.Name)
	if err != nil {
		return err
	}
	if name != definition.Name {
		return errors.New("definition name is not normalized")
	}
	description, err := normalizeDescription(definition.Description)
	if err != nil {
		return err
	}
	if description != definition.Description {
		return errors.New("definition description is not normalized")
	}
	body, err := normalizeBody(definition.Body)
	if err != nil {
		return err
	}
	if body != definition.Body {
		return errors.New("definition body is not normalized")
	}
	if definition.Digest != DefinitionDigest(definition.ID, definition.Name, definition.Body) {
		return errors.New("definition digest does not match content")
	}
	return nil
}

func ValidateBundleProvenance(maskID string, revision int64, provenance BundleProvenance) error {
	if !bundlecontract.ValidInstallID(provenance.Release.InstallID) || !bundlecontract.ValidDigest(provenance.Release.ReleaseDigest) ||
		!bundlecontract.ValidLocalID(provenance.LocalID) || provenance.Release.ReleaseSequence <= 0 || !bundlecontract.ValidBundleID(provenance.Release.BundleID) {
		return errors.New("bundle mask provenance is invalid")
	}
	expected, err := bundlecontract.MaskID(provenance.Release.InstallID, provenance.LocalID)
	if err != nil || expected != maskID || revision != provenance.Release.ReleaseSequence {
		return errors.New("bundle mask provenance does not match identity and revision")
	}
	return nil
}

func ValidateRecommendations(items []SkillRecommendation, provenance *BundleProvenance) error {
	seen := map[string]bool{}
	for _, item := range items {
		if item.Ref.Kind != bundlecontract.Skill || !bundlecontract.ValidInstallID(item.Ref.InstallID) || !bundlecontract.ValidLocalID(item.Ref.LocalID) {
			return errors.New("mask Skill recommendation reference is invalid")
		}
		if provenance != nil && item.Ref.InstallID != provenance.Release.InstallID {
			return errors.New("mask Skill recommendation belongs to another bundle")
		}
		id, err := bundlecontract.SkillID(item.Ref.InstallID, item.Ref.LocalID)
		if err != nil || item.SkillID != id || seen[id] {
			return errors.New("mask Skill recommendation identity is invalid or duplicated")
		}
		name, err := normalizeName(item.Name)
		if err != nil || name != item.Name {
			return errors.New("mask Skill recommendation name is invalid")
		}
		if (item.Available && item.InactiveReason != "") || (!item.Available && item.InactiveReason != "skill_unavailable") {
			return errors.New("mask Skill recommendation availability is invalid")
		}
		seen[id] = true
	}
	return nil
}

func IsBuiltinID(id string) bool {
	switch id {
	case BuiltinProgrammerID, BuiltinResearcherID, BuiltinWriterID:
		return true
	default:
		return false
	}
}

func BuiltinIDs() []string {
	return []string{BuiltinProgrammerID, BuiltinResearcherID, BuiltinWriterID}
}

func normalizeName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("name must not be blank")
	}
	if err := validateMetadata(value, MaxNameBytes, "name"); err != nil {
		return "", err
	}
	return value, nil
}

func normalizeDescription(value string) (string, error) {
	if err := validateMetadata(value, MaxDescriptionBytes, "description"); err != nil {
		return "", err
	}
	return value, nil
}

func normalizeBody(value string) (string, error) {
	value = normalizeLineEndings(value)
	if !utf8.ValidString(value) {
		return "", errors.New("body must be valid UTF-8")
	}
	if strings.TrimSpace(value) == "" {
		return "", errors.New("body must not be blank")
	}
	if len(value) > MaxBodyBytes {
		return "", fmt.Errorf("body exceeds %d bytes", MaxBodyBytes)
	}
	for _, r := range value {
		if (r < 0x20 && r != '\t' && r != '\n') || r == 0x7f {
			return "", errors.New("body contains a disallowed control character")
		}
	}
	return value, nil
}

func validateMetadata(value string, max int, field string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", field)
	}
	if len(value) > max {
		return fmt.Errorf("%s exceeds %d bytes", field, max)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%s contains a disallowed control character", field)
		}
	}
	return nil
}

func normalizeLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func validateUUID(value string) error {
	if !validateUUIDText(value) {
		return errors.New("operation id must be a canonical UUID")
	}
	return nil
}

func validateUUIDText(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		switch index {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if !isHex(char) {
				return false
			}
		}
	}
	return true
}

func isHex(value rune) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func validationError(cause error) error {
	return NewError(CodeInvalidMask, cause)
}
