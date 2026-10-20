package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const maxToolOperationArgumentsBytes = 1 << 20

// ValidateToolOperationAdmission validates every private input independently.
func ValidateToolOperationAdmission(op domain.ToolOperation) error {
	if op.RunID == "" || strings.TrimSpace(op.OperationID) == "" || strings.TrimSpace(op.ToolName) == "" {
		return errors.New("storage: tool operation identity is incomplete")
	}
	for _, input := range []struct {
		name string
		data []byte
	}{{"request", op.RequestArguments}, {"middleware input", op.MiddlewareInputArguments}, {"effective", op.EffectiveArguments}} {
		if len(input.data) == 0 || len(input.data) > maxToolOperationArgumentsBytes || !json.Valid(input.data) {
			return fmt.Errorf("storage: tool operation %s must be nonempty valid JSON within %d bytes", input.name, maxToolOperationArgumentsBytes)
		}
	}
	if op.State != "" || op.ClaimOwner != "" || op.Outcome() != (domain.ToolOperationOutcome{}) {
		return errors.New("storage: tool operation admission contains a later state")
	}
	return nil
}

// CloneToolOperationInputs gives caller and persisted admission independent bytes.
func CloneToolOperationInputs(op domain.ToolOperation) domain.ToolOperation {
	op.RequestArguments = bytes.Clone(op.RequestArguments)
	op.MiddlewareInputArguments = bytes.Clone(op.MiddlewareInputArguments)
	op.EffectiveArguments = bytes.Clone(op.EffectiveArguments)
	return op
}

func ValidateToolOperationSource(op domain.ToolOperation) error {
	if len(op.RequestArguments) == 0 {
		return ErrSourceBindingUnavailable
	}
	return nil
}

func SameToolOperationBinding(a, b domain.ToolOperation) bool {
	return a.RunID == b.RunID && a.OperationID == b.OperationID && a.ToolName == b.ToolName && bytes.Equal(a.RequestArguments, b.RequestArguments) && bytes.Equal(a.MiddlewareInputArguments, b.MiddlewareInputArguments) && bytes.Equal(a.EffectiveArguments, b.EffectiveArguments)
}

func ValidateToolOperationOutcome(outcome domain.ToolOperationOutcome) error {
	switch outcome.Status {
	case domain.ToolOutcomeSuccess, domain.ToolOutcomeFatal:
		if outcome.Reason != "" {
			return errors.New("storage: unexpected tool outcome reason")
		}
	case domain.ToolOutcomeRecoverable, domain.ToolOutcomeRefused:
		switch outcome.Reason {
		case domain.ToolReasonInvalidArguments, domain.ToolReasonNotFound, domain.ToolReasonPolicyDenied, domain.ToolReasonUserDenied, domain.ToolReasonCommandFailed, domain.ToolReasonRemoteTool:
		default:
			return errors.New("storage: invalid tool outcome reason")
		}
	default:
		return errors.New("storage: invalid tool outcome status")
	}
	switch outcome.Effects {
	case domain.ToolEffectsNotExecuted, domain.ToolEffectsNone, domain.ToolEffectsUnknown:
	default:
		return errors.New("storage: invalid tool outcome effects")
	}
	return nil
}

// NewToolOperationEvent preserves the public v1 lifecycle without private inputs.
func NewToolOperationEvent(op domain.ToolOperation) (domain.RunEvent, error) {
	if !op.State.Valid() || op.OperationID == "" || op.ToolName == "" {
		return domain.RunEvent{}, errors.New("storage: invalid tool operation event")
	}
	payload := struct {
		OperationID string                    `json:"operation_id"`
		ToolName    string                    `json:"tool_name"`
		State       domain.ToolOperationState `json:"state"`
		Result      string                    `json:"result,omitempty"`
		Failure     string                    `json:"failure,omitempty"`
	}{op.OperationID, op.ToolName, op.State, op.Result, op.Failure}
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: encode tool operation event: %w", err)
	}
	return domain.RunEvent{RunID: op.RunID, Type: domain.EventToolOperation, CreatedAt: op.UpdatedAt, PayloadVersion: 1, Payload: data}, nil
}
