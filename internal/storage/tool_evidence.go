package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
type ToolEvidence struct {
	Result          string `json:"result"`
	Failure         string `json:"failure"`
	ModelProjection string `json:"model_projection,omitempty"`
	ModelFailure    string `json:"model_failure,omitempty"`
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
