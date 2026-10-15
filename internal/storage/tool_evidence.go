package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"agent-vivy/internal/domain"
)

const ToolEvidenceSchema = "vivy.tool-evidence/v1"

// ToolEvidenceReference lives in the existing operation receipt and Journal.
// Only a pinned subject operation may interpret it; ordinary tool text remains
// opaque even when it happens to look like this JSON.
type ToolEvidenceReference struct {
	Schema           string       `json:"schema"`
	ID               string       `json:"id"`
	SHA256           string       `json:"sha256"`
	ByteCount        int          `json:"byte_count"`
	RunID            domain.RunID `json:"run_id"`
	OperationID      string       `json:"operation_id"`
	Projection       string       `json:"projection"`
	ResultSHA256     string       `json:"result_sha256"`
	ResultByteCount  int          `json:"result_byte_count"`
	FailureSHA256    string       `json:"failure_sha256"`
	FailureByteCount int          `json:"failure_byte_count"`
}

// ToolEvidenceOutcome is the persisted native tool.finished failure unit.
// An explicit all-empty unit attests no classified failure; absence is legacy
// data and is never promoted to such an attestation during replay.
type ToolEvidenceOutcome struct {
	Outcome    string `json:"outcome"`
	Reason     string `json:"reason"`
	Effects    string `json:"effects"`
	Diagnostic string `json:"diagnostic"`
}

func (o ToolEvidenceOutcome) Validate() error {
	if o == (ToolEvidenceOutcome{}) {
		return nil
	}
	if !utf8.ValidString(o.Diagnostic) || len(o.Diagnostic) > 2048 {
		return ErrSubjectEvidenceIncomplete
	}
	switch o.Outcome {
	case "refused":
		if o.Effects == "not_executed" && (o.Reason == "policy_denied" || o.Reason == "user_denied" || o.Reason == "invalid_arguments") {
			return nil
		}
	case "recoverable":
		if (o.Reason == "not_found" && o.Effects == "none") || ((o.Reason == "invalid_arguments" || o.Reason == "command_failed" || o.Reason == "remote_tool_error") && o.Effects == "unknown") {
			return nil
		}
	}
	return ErrSubjectEvidenceIncomplete
}
func decodeToolEvidenceOutcome(raw json.RawMessage) (*ToolEvidenceOutcome, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrSubjectEvidenceIncomplete
	}
	fields := map[string]bool{}
	values := map[string]string{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, ErrSubjectEvidenceIncomplete
		}
		name, ok := key.(string)
		if !ok || fields[name] {
			return nil, ErrSubjectEvidenceIncomplete
		}
		switch name {
		case "outcome", "reason", "effects", "diagnostic":
		default:
			return nil, ErrSubjectEvidenceIncomplete
		}
		fields[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrSubjectEvidenceIncomplete
		}
		var text string
		if json.Unmarshal(value, &text) != nil {
			return nil, ErrSubjectEvidenceIncomplete
		}
		values[name] = text
	}
	if _, err = decoder.Token(); err != nil || len(fields) != 4 {
		return nil, ErrSubjectEvidenceIncomplete
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, ErrSubjectEvidenceIncomplete
	}
	outcome := &ToolEvidenceOutcome{Outcome: values["outcome"], Reason: values["reason"], Effects: values["effects"], Diagnostic: values["diagnostic"]}
	return outcome, outcome.Validate()
}

type ToolEvidence struct {
	Outcome         *ToolEvidenceOutcome `json:"outcome,omitempty"`
	Result          string               `json:"result"`
	Failure         string               `json:"failure"`
	ModelProjection string               `json:"model_projection,omitempty"`
	ModelFailure    string               `json:"model_failure,omitempty"`
}

func ToolEvidenceArgumentsID(op domain.ToolOperation) string {
	return "subject-tool/" + toolEvidenceKey(op) + "/arguments/" + op.RequestDigest
}
func toolEvidenceKey(op domain.ToolOperation) string {
	sum := sha256.Sum256([]byte(string(op.RunID) + "\x00" + op.OperationID))
	return hex.EncodeToString(sum[:])
}
func ToolEvidenceResultID(op domain.ToolOperation, hash string) string {
	return "subject-tool/" + toolEvidenceKey(op) + "/result/" + hash
}
func NewToolEvidenceReference(op domain.ToolOperation, raw []byte, projection string) ToolEvidenceReference {
	hash := digestToolOperationBytes(raw)
	var evidence ToolEvidence
	_ = json.Unmarshal(raw, &evidence)
	return ToolEvidenceReference{ResultSHA256: digestToolOperationBytes([]byte(evidence.Result)), ResultByteCount: len(evidence.Result), FailureSHA256: digestToolOperationBytes([]byte(evidence.Failure)), FailureByteCount: len(evidence.Failure), Schema: ToolEvidenceSchema, ID: ToolEvidenceResultID(op, hash), SHA256: hash, ByteCount: len(raw), RunID: op.RunID, OperationID: op.OperationID, Projection: projection}
}
func DecodeToolEvidenceReference(op domain.ToolOperation) (ToolEvidenceReference, error) {
	var ref ToolEvidenceReference
	if json.Unmarshal([]byte(op.Result), &ref) != nil || ref.Schema != ToolEvidenceSchema || ref.RunID != op.RunID || ref.OperationID != op.OperationID || len(ref.SHA256) != 64 || ref.ByteCount < 0 || len(ref.ResultSHA256) != 64 || ref.ResultByteCount < 0 || len(ref.FailureSHA256) != 64 || ref.FailureByteCount < 0 || ref.ID != ToolEvidenceResultID(op, ref.SHA256) {
		return ref, ErrSubjectEvidenceIncomplete
	}
	return ref, nil
}
func VerifyToolEvidence(ref ToolEvidenceReference, raw []byte) (ToolEvidence, error) {
	var evidence ToolEvidence
	if len(raw) != ref.ByteCount || digestToolOperationBytes(raw) != ref.SHA256 || json.Unmarshal(raw, &evidence) != nil {
		return evidence, fmt.Errorf("tool artifact %s: %w", ref.ID, ErrSubjectEvidenceIncomplete)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return evidence, ErrSubjectEvidenceIncomplete
	}
	if encoded, present := fields["outcome"]; present {
		var err error
		evidence.Outcome, err = decodeToolEvidenceOutcome(encoded)
		if err != nil {
			return evidence, err
		}
	}
	if _, present := fields["outcome"]; !present && evidence.Outcome != nil {
		return evidence, ErrSubjectEvidenceIncomplete
	}
	if ref.ResultSHA256 != digestToolOperationBytes([]byte(evidence.Result)) || ref.ResultByteCount != len(evidence.Result) || ref.FailureSHA256 != digestToolOperationBytes([]byte(evidence.Failure)) || ref.FailureByteCount != len(evidence.Failure) {
		return evidence, ErrSubjectEvidenceIncomplete
	}
	expected := evidence.ModelProjection
	if evidence.ModelFailure == "" {
		expected += "\n[exact tool result: " + ref.ID + " sha256=" + ref.SHA256 + "]"
	}
	if ref.Projection != expected {
		return evidence, ErrSubjectEvidenceIncomplete
	}
	return evidence, nil
}
func VerifyToolEvidenceArguments(op domain.ToolOperation, raw []byte) error {
	if digestToolOperationBytes(raw) != op.RequestDigest || !utf8.Valid(raw) || !json.Valid(raw) {
		return ErrSubjectEvidenceIncomplete
	}
	return nil
}

// ToolArgumentsReference names model/user-supplied arguments only. Private
// middleware/effective arguments never enter this public reference surface.
type ToolArgumentsReference struct {
	ID        string `json:"id"`
	SHA256    string `json:"sha256"`
	ByteCount int    `json:"byte_count"`
}

func NewToolArgumentsReference(op domain.ToolOperation, raw []byte) ToolArgumentsReference {
	return ToolArgumentsReference{ID: ToolEvidenceArgumentsID(op), SHA256: op.RequestDigest, ByteCount: len(raw)}
}
func VerifyToolArgumentsReference(op domain.ToolOperation, ref ToolArgumentsReference, raw []byte) error {
	if ref.ID != ToolEvidenceArgumentsID(op) || ref.SHA256 != op.RequestDigest || ref.ByteCount != len(raw) {
		return ErrSubjectEvidenceIncomplete
	}
	return VerifyToolEvidenceArguments(op, raw)
}
