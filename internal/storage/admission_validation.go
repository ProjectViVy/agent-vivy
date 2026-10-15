package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
)

// ValidateRunAdmissionInput checks the fields that are independent of a
// backend transaction. The backend must still validate the captured
// selection and definition while holding its session/definition locks.
func ValidateRunAdmissionInput(in RunAdmission) error {
	if in.Subject != nil {
		if err := in.Subject.Validate(); err != nil {
			return err
		}
		if in.Subject.SessionID != in.Run.SessionID || (in.Run.Kind != "" && in.Run.Kind != domain.RunKindPrimary) || in.Run.ParentID != "" || in.Run.Depth != 0 || in.Run.Purpose != "" {
			return ErrSubjectBinding
		}
	}
	if strings.TrimSpace(string(in.Run.ID)) == "" {
		return admissionInvalid("run id is required")
	}
	if strings.TrimSpace(string(in.Run.SessionID)) == "" {
		return admissionInvalid("run session id is required")
	}
	if in.Run.Status.Terminal() {
		return admissionInvalid("run admission cannot start a terminal run")
	}
	if in.Run.Status != "" && !in.Run.Status.Valid() {
		return admissionInvalid("run status is invalid")
	}
	if in.Run.Kind != "" && !in.Run.Kind.Valid() {
		return admissionInvalid("run kind is invalid")
	}
	if in.OmitMessage {
		if in.Run.Kind != domain.RunKindChild || in.Run.EffectiveChildMode() != domain.ChildModeOneShot {
			return admissionInvalid("only one-shot child admission may omit its transcript message")
		}
		if in.Message.ID != "" || in.Message.SessionID != "" || in.Message.RunID != "" || in.Message.Role != "" ||
			in.Message.CreatedAt != 0 || in.Message.Content != "" || len(in.Message.Attachments) != 0 ||
			len(in.Message.FileContexts) != 0 || in.Message.ToolCallID != "" || in.Message.ToolName != "" ||
			len(in.Message.ToolArgs) != 0 || in.Message.Source != "" || in.Message.Channel != "" ||
			in.Message.ChatID != "" || in.Message.ChannelMessageID != "" {
			return admissionInvalid("omitted admission message must be empty")
		}
	} else {
		if strings.TrimSpace(in.Message.ID) == "" {
			return admissionInvalid("message id is required")
		}
		if in.Message.SessionID != in.Run.SessionID {
			return admissionInvalid("message and run sessions differ")
		}
		if in.Message.RunID != "" && in.Message.RunID != in.Run.ID {
			return admissionInvalid("message and run ids differ")
		}
		if in.Message.Role != domain.RoleUser {
			return admissionInvalid("run admission message must be a user message")
		}
	}
	if in.Started.RunID != in.Run.ID {
		return admissionInvalid("run.started and run ids differ")
	}
	if in.Started.Type != domain.EventRunStarted {
		return admissionInvalid("admission event must be run.started")
	}
	if in.Started.PayloadVersion <= 0 {
		return admissionInvalid("run.started payload version is invalid")
	}
	for _, event := range in.Events {
		if event.RunID != in.Run.ID || event.PayloadVersion <= 0 || (event.Type != domain.EventTurnQueued && event.Type != domain.EventTurnDequeued) {
			return admissionInvalid("invalid queue admission event")
		}
	}
	if in.Edit != nil {
		if in.Edit.SessionID != in.Run.SessionID {
			return admissionInvalid("edit marker and run sessions differ")
		}
		if in.Edit.RunID != "" && in.Edit.RunID != in.Run.ID {
			return admissionInvalid("edit marker and run ids differ")
		}
		if in.Edit.Reason != TruncationEdit {
			return admissionInvalid("admission edit marker has the wrong reason")
		}
		if strings.TrimSpace(in.Edit.CutoffMessageID) == "" || strings.TrimSpace(in.Edit.TailMessageID) == "" {
			return admissionInvalid("admission edit marker requires cutoff and tail")
		}
	}
	if err := ValidateMaskCaptureCheck(in.ExpectedMask); err != nil {
		return err
	}
	if err := ValidateRunBundleAdmission(in.Run.ID, in.Started, in.Bundle, in.ExpectedBundle); err != nil {
		return err
	}
	if in.ExpectedMask != nil && in.ExpectedMask.SessionID != in.Run.SessionID {
		return admissionInvalid("mask capture and run sessions differ")
	}
	markerSchema, markerDigest, err := runStartedPromptMarker(in.Started.Payload)
	if err != nil {
		return mask.NewError(mask.CodeSnapshotCorrupt, err)
	}
	if in.Prompt == nil {
		if markerSchema != 0 || markerDigest != "" {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("run.started prompt marker requires a prompt snapshot"))
		}
		if in.ExpectedMask != nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("mask capture requires a prompt snapshot"))
		}
		return nil
	}
	if markerSchema != in.Prompt.SchemaVersion || markerDigest != in.Prompt.PayloadSHA256 {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("run.started prompt marker does not match prompt snapshot"))
	}
	payload, err := ValidateRunPromptSnapshot(*in.Prompt)
	if err != nil {
		return err
	}
	if in.Prompt.RunID != in.Run.ID {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt snapshot and run ids differ"))
	}
	if payload.Rules != nil && (in.Bundle == nil || in.ExpectedBundle == nil) {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("Toolkit Rules prompt requires a captured Toolkit catalog"))
	}
	if in.Bundle != nil {
		bundlePayload, err := ValidateRunBundleSnapshot(*in.Bundle)
		if err != nil {
			return admissionInvalid("Toolkit Rules validation requires a valid catalog snapshot")
		}
		var started struct {
			Face string `json:"face"`
		}
		if err := json.Unmarshal(in.Started.Payload, &started); err != nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("run.started face is invalid"))
		}
		if runSnapshotHasRulesForFace(bundlePayload, started.Face) && payload.Rules == nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("applicable Toolkit Rules are missing from the prompt snapshot"))
		}
		if payload.Rules != nil && (started.Face != payload.Rules.Face || !runSnapshotContainsRules(bundlePayload, *payload.Rules)) {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("Toolkit Rules are outside the captured Toolkit catalog or face scope"))
		}
	}
	if in.ExpectedMask == nil {
		if payload.Mask != nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("a mask snapshot requires a captured selection"))
		}
		return nil
	}
	if err := ValidatePromptMaskCapture(payload, *in.ExpectedMask); err != nil {
		return err
	}
	if payload.Mask != nil && mask.ResolveOrigin(payload.Mask.ID, payload.Mask.Origin) == mask.OriginBundle {
		if in.Bundle == nil || in.ExpectedBundle == nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("bundle Mask prompt requires a captured Toolkit catalog"))
		}
		bundlePayload, err := ValidateRunBundleSnapshot(*in.Bundle)
		if err != nil {
			return admissionInvalid("bundle Mask prompt has an invalid Toolkit snapshot")
		}
		if !runSnapshotContainsMask(bundlePayload, *payload.Mask) {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("bundle Mask provenance is outside the captured Toolkit catalog"))
		}
	}
	return nil
}

// ValidateBundleAdmission binds the durable snapshot row to the exact
// catalog/revocation CAS used by the admission transaction. Backends repeat
// the live comparison while holding their existing session write lock.
func ValidateRunBundleAdmission(runID domain.RunID, started domain.RunEvent, snapshot *RunBundleSnapshot, expected *bundlecontract.CaptureCheck) error {
	if snapshot == nil && expected == nil {
		schemaVersion, digest, err := runStartedBundleMarker(started.Payload)
		if err != nil || schemaVersion != 0 || digest != "" {
			return admissionInvalid("run.started has a toolkit marker without a snapshot")
		}
		return nil
	}
	if snapshot == nil || expected == nil {
		return admissionInvalid("toolkit admission requires both snapshot and expected capture")
	}
	payload, err := ValidateRunBundleSnapshot(*snapshot)
	if err != nil {
		return admissionInvalid(err.Error())
	}
	if snapshot.RunID != runID || payload.Catalog != expected.Catalog || payload.RevocationRevision != expected.RevocationRevision {
		return admissionInvalid("toolkit snapshot and expected capture differ")
	}
	derived := bundlecontract.CaptureCheckFor(payload)
	if !reflect.DeepEqual(derived, *expected) {
		return admissionInvalid("toolkit executable references do not match the snapshot")
	}
	schemaVersion, digest, err := runStartedBundleMarker(started.Payload)
	if err != nil || schemaVersion != snapshot.SchemaVersion || digest != snapshot.PayloadSHA256 {
		return admissionInvalid("run.started toolkit marker does not match the snapshot")
	}
	return nil
}

func runStartedBundleMarker(raw []byte) (schemaVersion int, digest string, err error) {
	if len(raw) == 0 {
		return 0, "", nil
	}
	var marker struct {
		BundleSchema int    `json:"bundle_schema"`
		BundleDigest string `json:"bundle_digest"`
	}
	if err := json.Unmarshal(raw, &marker); err != nil {
		return 0, "", err
	}
	if marker.BundleSchema < 0 || (marker.BundleSchema == 0 && marker.BundleDigest != "") ||
		(marker.BundleSchema > 0 && marker.BundleDigest == "") {
		return 0, "", errors.New("incomplete toolkit marker")
	}
	return marker.BundleSchema, marker.BundleDigest, nil
}

func ValidateCaptureCheck(in *bundlecontract.CaptureCheck) error {
	if in == nil {
		return nil
	}
	if in.Catalog.Epoch < 0 || in.Catalog.ConfigRevision < 0 || in.RevocationRevision < 0 ||
		(in.Catalog.Epoch == 0 && (in.Catalog.EffectiveDigest != "" || in.Catalog.GenerationID != "")) ||
		(in.Catalog.Epoch > 0 && !bundlecontract.ValidDigest(in.Catalog.EffectiveDigest)) {
		return admissionInvalid("invalid toolkit catalog capture")
	}
	for i, ref := range in.Executables {
		if !bundlecontract.ValidInstallID(ref.InstallID) || !bundlecontract.ValidLocalID(ref.LocalID) ||
			(ref.Kind != bundlecontract.Hook && ref.Kind != bundlecontract.MCP) || !bundlecontract.ValidDigest(ref.DeclarationDigest) {
			return admissionInvalid("invalid captured executable reference")
		}
		if i > 0 {
			previous := in.Executables[i-1]
			if executableRefKey(previous) >= executableRefKey(ref) {
				return admissionInvalid("captured executable references are not uniquely sorted")
			}
		}
		if (ref.Kind == bundlecontract.Hook || ref.Kind == bundlecontract.MCP) && ((ref.ConsentDigest == "") != (ref.ConsentRevision == 0)) {
			return admissionInvalid("captured executable consent reference is incomplete")
		}
		if ref.ConsentDigest != "" && (!bundlecontract.ValidDigest(ref.ConsentDigest) || ref.ConsentRevision <= 0) {
			return admissionInvalid("captured executable consent reference is invalid")
		}
	}
	for i, consent := range in.Consents {
		if !bundlecontract.ValidInstallID(consent.InstallID) || (consent.Kind != bundlecontract.Hook && consent.Kind != bundlecontract.MCP) || !bundlecontract.ValidLocalID(consent.LocalID) {
			return admissionInvalid("captured executable consent identity is invalid")
		}
		if consent.Present {
			if !bundlecontract.ValidDigest(consent.DeclarationDigest) || consent.Revision <= 0 {
				return admissionInvalid("captured executable consent row is invalid")
			}
		} else if consent.DeclarationDigest != "" || consent.Allowed || consent.Revision != 0 {
			return admissionInvalid("captured absent executable consent row is inconsistent")
		}
		if i > 0 {
			previous := in.Consents[i-1]
			if consentRefKey(previous) >= consentRefKey(consent) {
				return admissionInvalid("captured executable consent references are not uniquely sorted")
			}
		}
	}
	return nil
}

func executableRefKey(r bundlecontract.ExecutableRef) string {
	return r.InstallID + "\x00" + string(r.Kind) + "\x00" + r.LocalID + "\x00" + r.DeclarationDigest
}

func consentRefKey(r bundlecontract.ConsentRef) string {
	return r.InstallID + "\x00" + string(r.Kind) + "\x00" + r.LocalID
}

// runStartedPromptMarker extracts the optional prompt identity from the
// opaque run.started payload without importing Runtime's payload types. The
// storage boundary owns the invariant that a new-format admission persists
// its marker and snapshot together, while legacy admissions carry neither.
func runStartedPromptMarker(raw []byte) (schemaVersion int, digest string, err error) {
	if len(raw) == 0 {
		return 0, "", nil
	}
	var marker struct {
		PromptSchema int    `json:"prompt_schema"`
		PromptDigest string `json:"prompt_digest"`
	}
	if err := json.Unmarshal(raw, &marker); err != nil {
		return 0, "", fmt.Errorf("run.started payload is not valid JSON: %w", err)
	}
	if marker.PromptSchema < 0 || (marker.PromptSchema == 0 && strings.TrimSpace(marker.PromptDigest) != "") ||
		(marker.PromptSchema > 0 && strings.TrimSpace(marker.PromptDigest) == "") {
		return 0, "", errors.New("run.started prompt marker is incomplete")
	}
	return marker.PromptSchema, marker.PromptDigest, nil
}

// ValidateMaskCaptureCheck validates the shape of the optimistic capture
// token. Current selection and custom definition values are checked by the
// backend inside the admission transaction.
func ValidateMaskCaptureCheck(in *MaskCaptureCheck) error {
	if in == nil {
		return nil
	}
	if err := mask.ValidateSessionID(in.SessionID); err != nil {
		return admissionInvalid(err.Error())
	}
	if in.SelectionRevision < 0 {
		return admissionInvalid("selection revision must not be negative")
	}
	if err := mask.ValidateSelectionMaskID(in.MaskID); err != nil {
		return admissionInvalid(err.Error())
	}
	origin := mask.ResolveOrigin(in.MaskID, in.Origin)
	if in.MaskID == "" || origin == mask.OriginBuiltin {
		if in.DefinitionRevision != 0 || in.DefinitionDigest != "" || in.Bundle != nil || len(in.RecommendedSkills) > 0 {
			return admissionInvalid("empty and built-in masks do not carry custom definition metadata")
		}
		return nil
	}
	if in.DefinitionRevision <= 0 || strings.TrimSpace(in.DefinitionDigest) == "" {
		return admissionInvalid("non-built-in mask capture requires definition revision and digest")
	}
	switch origin {
	case mask.OriginBundle:
		if in.Bundle == nil || mask.ValidateBundleProvenance(in.MaskID, in.DefinitionRevision, *in.Bundle) != nil {
			return admissionInvalid("bundle Mask capture provenance is invalid")
		}
	case mask.OriginCustom:
		if mask.ValidateCustomID(in.MaskID) != nil {
			return admissionInvalid("custom Mask identity is invalid")
		}
		if in.Bundle != nil {
			sourceID, err := bundlecontract.MaskID(in.Bundle.Release.InstallID, in.Bundle.LocalID)
			if err != nil || mask.ValidateBundleProvenance(sourceID, in.Bundle.Release.ReleaseSequence, *in.Bundle) != nil {
				return admissionInvalid("custom Mask copy provenance is invalid")
			}
		} else if len(in.RecommendedSkills) > 0 {
			return admissionInvalid("custom Mask recommendations require copy provenance")
		}
	default:
		return admissionInvalid("mask capture origin is invalid")
	}
	if err := mask.ValidateRecommendations(in.RecommendedSkills, in.Bundle); err != nil {
		return admissionInvalid("mask recommendations are invalid")
	}
	return nil
}

// ValidateRunPromptSnapshot verifies the immutable envelope and decodes its
// canonical payload. Storage deliberately validates bytes and structure, but
// leaves Generation compatibility decisions to Runtime.
func ValidateRunPromptSnapshot(in RunPromptSnapshot) (RunPromptPayload, error) {
	if strings.TrimSpace(string(in.RunID)) == "" {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt snapshot run id is required"))
	}
	if in.SchemaVersion <= 0 || strings.TrimSpace(in.ComposerVersion) == "" || strings.TrimSpace(in.GenerationID) == "" {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt snapshot metadata is incomplete"))
	}
	if len(in.Payload) == 0 || !json.Valid(in.Payload) {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt snapshot payload is not valid JSON"))
	}
	if strings.TrimSpace(in.PayloadSHA256) == "" {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt snapshot digest is required"))
	}
	sum := sha256.Sum256(in.Payload)
	if in.PayloadSHA256 != hex.EncodeToString(sum[:]) {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt snapshot digest does not match payload"))
	}
	var payload RunPromptPayload
	if err := json.Unmarshal(in.Payload, &payload); err != nil {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, err)
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, fmt.Errorf("canonicalize prompt snapshot payload: %w", err))
	}
	if !bytes.Equal(canonical, in.Payload) {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt snapshot payload is not canonical JSON"))
	}
	if strings.TrimSpace(payload.Instruction) == "" {
		return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt snapshot instruction is empty"))
	}
	if payload.Mask != nil {
		if err := validatePromptMask(*payload.Mask, in.GenerationID); err != nil {
			return RunPromptPayload{}, err
		}
	}
	if payload.Rules != nil {
		if err := ValidateRunRulesSnapshot(payload.Rules); err != nil {
			return RunPromptPayload{}, mask.NewError(mask.CodeSnapshotCorrupt, err)
		}
	}
	return payload, nil
}

func runSnapshotContainsRules(snapshot bundlecontract.RunSnapshotPayload, rules RunRulesSnapshot) bool {
	expected := make(map[bundlecontract.ComponentRef]bundlecontract.CapturedComponent)
	for _, component := range snapshot.Components {
		if component.Ref.Kind != bundlecontract.Rule || !componentAppliesToFace(component.FaceScope, rules.Face) {
			continue
		}
		expected[component.Ref] = component
	}
	if len(expected) != len(rules.Blocks) {
		return false
	}
	for _, block := range rules.Blocks {
		component, ok := expected[block.Ref]
		if !ok || component.Release != block.Release || component.ContentDigest != block.ContentDigest {
			return false
		}
		delete(expected, block.Ref)
	}
	return len(expected) == 0
}

func runSnapshotHasRulesForFace(snapshot bundlecontract.RunSnapshotPayload, face string) bool {
	for _, component := range snapshot.Components {
		if component.Ref.Kind == bundlecontract.Rule && componentAppliesToFace(component.FaceScope, face) {
			return true
		}
	}
	return false
}

func componentAppliesToFace(faces []string, face string) bool {
	if len(faces) == 0 {
		return true
	}
	for _, candidate := range faces {
		if candidate == face {
			return true
		}
	}
	return false
}

// ValidatePromptMaskCapture ensures the persisted prompt bytes and the CAS
// token describe the same captured selection. This prevents a caller from
// checking one selection while persisting another mask's instruction.
func ValidatePromptMaskCapture(payload RunPromptPayload, in MaskCaptureCheck) error {
	if in.MaskID == "" {
		if payload.Mask != nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("unmasked capture carries a mask snapshot"))
		}
		return nil
	}
	if payload.Mask == nil {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("captured mask is missing from prompt snapshot"))
	}
	if payload.Mask.ID != in.MaskID || payload.Mask.SelectionRevision != in.SelectionRevision {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt mask does not match captured selection"))
	}
	if mask.ResolveOrigin(payload.Mask.ID, payload.Mask.Origin) != mask.ResolveOrigin(in.MaskID, in.Origin) {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt mask origin does not match captured selection"))
	}
	if !mask.IsBuiltinID(in.MaskID) && (payload.Mask.DefinitionRevision != in.DefinitionRevision || payload.Mask.Digest != in.DefinitionDigest) {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt mask does not match captured definition"))
	}
	if !sameMaskBundle(payload.Mask.Bundle, in.Bundle) {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt Mask provenance does not match captured definition"))
	}
	if !reflect.DeepEqual(payload.Mask.RecommendedSkills, in.RecommendedSkills) {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt Mask recommendations do not match captured definition"))
	}
	return nil
}

func validatePromptMask(in mask.Snapshot, generationID string) error {
	if err := mask.ValidateMaskID(in.ID); err != nil {
		return mask.NewError(mask.CodeSnapshotCorrupt, err)
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Body) == "" || strings.TrimSpace(in.Digest) == "" {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt mask is incomplete"))
	}
	if in.DefinitionRevision <= 0 || in.SelectionRevision < 0 {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt mask revisions are invalid"))
	}
	if in.Digest != mask.DefinitionDigest(in.ID, in.Name, in.Body) {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt mask digest does not match content"))
	}
	origin := mask.ResolveOrigin(in.ID, in.Origin)
	if origin == mask.OriginBuiltin {
		if in.DefinitionRevision != mask.BuiltinRevision || strings.TrimSpace(in.GenerationID) == "" || in.GenerationID != generationID {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("built-in prompt mask generation is invalid"))
		}
		if in.Bundle != nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("built-in prompt Mask carries bundle provenance"))
		}
	} else if in.GenerationID != "" {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("non-built-in prompt mask carries a generation id"))
	} else if origin == mask.OriginBundle {
		if in.Bundle == nil || mask.ValidateBundleProvenance(in.ID, in.DefinitionRevision, *in.Bundle) != nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("bundle prompt Mask provenance is invalid"))
		}
	} else if origin == mask.OriginCustom && in.Bundle != nil {
		sourceID, err := bundlecontract.MaskID(in.Bundle.Release.InstallID, in.Bundle.LocalID)
		if err != nil || mask.ValidateBundleProvenance(sourceID, in.Bundle.Release.ReleaseSequence, *in.Bundle) != nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("copied prompt Mask provenance is invalid"))
		}
	}
	if origin != mask.OriginBuiltin && origin != mask.OriginCustom && origin != mask.OriginBundle {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt Mask origin is invalid"))
	}
	if err := mask.ValidateRecommendations(in.RecommendedSkills, in.Bundle); err != nil {
		return mask.NewError(mask.CodeSnapshotCorrupt, err)
	}
	return nil
}

func sameMaskBundle(a, b *mask.BundleProvenance) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// MaskBundleProvenanceEqual compares immutable source metadata without
// exposing a mutable pointer contract to backend admission validators.
func MaskBundleProvenanceEqual(a, b *mask.BundleProvenance) bool {
	return sameMaskBundle(a, b)
}

func runSnapshotContainsMask(payload bundlecontract.RunSnapshotPayload, snapshot mask.Snapshot) bool {
	if snapshot.Bundle == nil {
		return false
	}
	for _, component := range payload.Components {
		if component.Ref.Kind != bundlecontract.Mask || component.ProjectedID != snapshot.ID || component.Release != snapshot.Bundle.Release || component.Ref.LocalID != snapshot.Bundle.LocalID {
			continue
		}
		return component.Release.ReleaseSequence == snapshot.DefinitionRevision &&
			snapshot.Digest == mask.DefinitionDigest(snapshot.ID, snapshot.Name, snapshot.Body)
	}
	return false
}

func admissionInvalid(message string) error {
	return mask.NewError(mask.CodeInvalidMask, errors.New(message))
}

// AdmissionConflict returns the stable error used when an admission retry
// names an existing run with different immutable input.
func AdmissionConflict() error {
	return mask.NewError(mask.CodeRevisionConflict, nil)
}

// AdmissionUnavailable wraps a backend failure without exposing driver text
// through the mask error vocabulary.
func AdmissionUnavailable(operation string, err error) error {
	if err == nil {
		err = errors.New("unknown storage failure")
	}
	return mask.NewError(mask.CodeUnavailable, fmt.Errorf("storage: %s: %w", operation, err))
}
