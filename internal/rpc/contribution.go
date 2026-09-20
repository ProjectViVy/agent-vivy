package rpc

import "agent-vivy/internal/rpccontract"

// These aliases keep the dispatcher-facing API stable while the canonical
// contribution contract remains independent from the dispatcher package.
type MethodBinding = rpccontract.MethodBinding
type Contribution = rpccontract.Contribution
type ContributionPeer = rpccontract.Peer
type ContributionHandlerFunc = rpccontract.HandlerFunc

var _ rpccontract.Peer = (*Peer)(nil)

func ValidateMethodBindings(coreMethods map[string]struct{}, bindings []MethodBinding) error {
	return rpccontract.ValidateMethodBindings(coreMethods, bindings)
}
