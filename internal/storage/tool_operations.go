package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const maxToolOperationArgumentsBytes = 1 << 20

// ValidateToolOperationAdmission checks the immutable identity and invocation
// bytes before a backend persists a new operation.
func ValidateToolOperationAdmission(op domain.ToolOperation) error {
	if op.RunID == "" || strings.TrimSpace(op.OperationID) == "" || strings.TrimSpace(op.ToolName) == "" {
		return errors.New("storage: tool operation identity is incomplete")
	}
	if op.RequestDigest == "" || len(op.MiddlewareInputArguments) == 0 || op.ArgumentsDigest == "" || len(op.EffectiveArguments) == 0 {
		return errors.New("storage: tool operation binding is incomplete")
	}
	if len(op.MiddlewareInputArguments) > maxToolOperationArgumentsBytes {
		return fmt.Errorf("storage: tool operation middleware input exceeds %d bytes", maxToolOperationArgumentsBytes)
	}
	if len(op.EffectiveArguments) > maxToolOperationArgumentsBytes {
		return fmt.Errorf("storage: tool operation arguments exceed %d bytes", maxToolOperationArgumentsBytes)
	}
	if !json.Valid(op.MiddlewareInputArguments) {
		return errors.New("storage: tool operation middleware input is invalid JSON")
	}
	if !json.Valid(op.EffectiveArguments) {
		return errors.New("storage: tool operation arguments are invalid JSON")
	}
	sum := sha256.Sum256(op.EffectiveArguments)
	if got := hex.EncodeToString(sum[:]); got != op.ArgumentsDigest {
		return errors.New("storage: tool operation arguments digest does not match invocation")
	}
	if op.State != "" || op.ClaimOwner != "" || op.Result != "" || op.Failure != "" {
		return errors.New("storage: tool operation admission contains a later state")
	}
	return nil
}

// NewToolOperationEvent creates the append-only Journal record mirrored by
// the SQL operation lookup row. The Journal carries immutable digests and
// lifecycle metadata; effective arguments remain in the private operation
// row so middleware-injected credentials are not exposed through event feeds.
func NewToolOperationEvent(op domain.ToolOperation) (domain.RunEvent, error) {
	if !op.State.Valid() || op.OperationID == "" || op.ToolName == "" {
		return domain.RunEvent{}, errors.New("storage: invalid tool operation event")
	}
	payload := struct {
		OperationID         string                    `json:"operation_id"`
		ToolName            string                    `json:"tool_name"`
		State               domain.ToolOperationState `json:"state"`
		RequestDigest       string                    `json:"request_digest,omitempty"`
		MiddlewareInputHash string                    `json:"middleware_input_digest,omitempty"`
		ArgumentsDigest     string                    `json:"arguments_digest,omitempty"`
		Result              string                    `json:"result,omitempty"`
		Failure             string                    `json:"failure,omitempty"`
	}{
		OperationID: op.OperationID, ToolName: op.ToolName, State: op.State,
		RequestDigest: op.RequestDigest, MiddlewareInputHash: digestToolOperationBytes(op.MiddlewareInputArguments),
		ArgumentsDigest: op.ArgumentsDigest,
		Result:          op.Result, Failure: op.Failure,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: encode tool operation event: %w", err)
	}
	return domain.RunEvent{
		RunID: op.RunID, Type: domain.EventToolOperation, CreatedAt: op.UpdatedAt,
		PayloadVersion: 1, Payload: data,
	}, nil
}

func digestToolOperationBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
