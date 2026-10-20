package runtime

import (
	"encoding/json"
	"errors"

	"agent-vivy/internal/tools"
)

const toolApprovalProposalEnvelopeKind = "vivy.tool-approval-proposal/v2"

type toolApprovalProposalEnvelope struct {
	Kind              string          `json:"kind"`
	IntendedArguments json.RawMessage `json:"intended_arguments"`
	ProviderData      []byte          `json:"provider_data,omitempty"`
}

func canonicalToolArguments(raw json.RawMessage) (json.RawMessage, error) {
	value, err := decodeJSONWithNumbers(raw)
	if err != nil {
		return nil, &redactedToolError{cause: err, message: "runtime: invalid intended tool arguments"}
	}
	return json.Marshal(value)
}
func bindToolApprovalProposal(providerData json.RawMessage, intended json.RawMessage) (json.RawMessage, error) {
	canonical, err := canonicalToolArguments(intended)
	if err != nil {
		return nil, err
	}
	return json.Marshal(toolApprovalProposalEnvelope{Kind: toolApprovalProposalEnvelopeKind, IntendedArguments: canonical, ProviderData: append([]byte(nil), providerData...)})
}
func unbindToolApprovalProposal(data json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	var envelope toolApprovalProposalEnvelope
	if json.Unmarshal(data, &envelope) != nil || envelope.Kind != toolApprovalProposalEnvelopeKind || len(envelope.IntendedArguments) == 0 {
		return nil, nil, errors.New("runtime: unsupported or unbound tool approval proposal")
	}
	canonical, err := canonicalToolArguments(envelope.IntendedArguments)
	if err != nil {
		return nil, nil, err
	}
	return append(json.RawMessage(nil), envelope.ProviderData...), canonical, nil
}

func redactedApprovalArguments(arguments map[string]any) map[string]any {
	redacted, _ := redactApprovalArgumentValue(arguments).(map[string]any)
	if redacted == nil {
		return map[string]any{}
	}
	return redacted
}

func redactApprovalArgumentValue(value any) any {
	switch typed := value.(type) {
	case string:
		return tools.RedactSensitive(typed)
	case []any:
		out := make([]any, len(typed))
		for index := range typed {
			out[index] = redactApprovalArgumentValue(typed[index])
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = redactApprovalArgumentValue(item)
		}
		return out
	default:
		return value
	}
}
