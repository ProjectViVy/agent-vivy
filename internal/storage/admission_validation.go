package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
)

// ValidateRunAdmissionInput checks the fields that are independent of a
// backend transaction. The backend must still validate the captured
// selection and definition while holding its session/definition locks.
func ValidateRunAdmissionInput(in RunAdmission) error {
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
	if in.Started.RunID != in.Run.ID {
		return admissionInvalid("run.started and run ids differ")
	}
	if in.Started.Type != domain.EventRunStarted {
		return admissionInvalid("admission event must be run.started")
	}
	if in.Started.PayloadVersion <= 0 {
		return admissionInvalid("run.started payload version is invalid")
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
	if in.ExpectedMask == nil {
		if payload.Mask != nil {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("a mask snapshot requires a captured selection"))
		}
		return nil
	}
	return ValidatePromptMaskCapture(payload, *in.ExpectedMask)
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
	if in.MaskID == "" || mask.IsBuiltinID(in.MaskID) {
		if in.DefinitionRevision != 0 || in.DefinitionDigest != "" {
			return admissionInvalid("empty and built-in masks do not carry custom definition metadata")
		}
		return nil
	}
	if in.DefinitionRevision <= 0 || strings.TrimSpace(in.DefinitionDigest) == "" {
		return admissionInvalid("custom mask capture requires definition revision and digest")
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
	return payload, nil
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
	if !mask.IsBuiltinID(in.MaskID) && (payload.Mask.DefinitionRevision != in.DefinitionRevision || payload.Mask.Digest != in.DefinitionDigest) {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("prompt mask does not match captured definition"))
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
	if mask.IsBuiltinID(in.ID) {
		if in.DefinitionRevision != mask.BuiltinRevision || strings.TrimSpace(in.GenerationID) == "" || in.GenerationID != generationID {
			return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("built-in prompt mask generation is invalid"))
		}
	} else if in.GenerationID != "" {
		return mask.NewError(mask.CodeSnapshotCorrupt, errors.New("custom prompt mask carries a generation id"))
	}
	return nil
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
