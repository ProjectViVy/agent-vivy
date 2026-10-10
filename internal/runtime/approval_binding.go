package runtime

import (
	"encoding/json"
	"errors"
)

const toolApprovalProposalEnvelopeKind = "vivy.tool-approval-proposal/v1"

type toolApprovalProposalEnvelope struct {
	Kind          string `json:"kind"`
	ArgumentsHash string `json:"arguments_sha256"`
	ProviderData  []byte `json:"provider_data,omitempty"`
}

func bindToolApprovalProposal(providerData json.RawMessage, argumentsHash string) (json.RawMessage, error) {
	if argumentsHash == "" {
		return nil, errors.New("runtime: empty tool approval arguments hash")
	}
	return json.Marshal(toolApprovalProposalEnvelope{
		Kind:          toolApprovalProposalEnvelopeKind,
		ArgumentsHash: argumentsHash,
		ProviderData:  append([]byte(nil), providerData...),
	})
}

func unbindToolApprovalProposal(data json.RawMessage) (providerData json.RawMessage, argumentsHash string, err error) {
	var envelope toolApprovalProposalEnvelope
	if json.Unmarshal(data, &envelope) != nil || envelope.Kind != toolApprovalProposalEnvelopeKind {
		// Legacy approvals stored provider data directly. Preserve that data for
		// compatibility, but leave the hash empty so effectful execution fails
		// closed instead of treating an unbound approval as authority.
		return append(json.RawMessage(nil), data...), "", nil
	}
	if envelope.ArgumentsHash == "" {
		return nil, "", errors.New("runtime: tool approval proposal has no arguments hash")
	}
	return append(json.RawMessage(nil), envelope.ProviderData...), envelope.ArgumentsHash, nil
}
