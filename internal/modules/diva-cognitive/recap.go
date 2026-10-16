package divacognitive

import "agent-vivy/internal/cognitivecontract"

// Recap exposes the same Garden owner's trusted subject control-plane handle.
// Its lifecycle is the bundle's lifecycle; no additional Store is opened.
func (b *bundle) Recap() cognitivecontract.RecapPort { return b.recap }

var _ cognitivecontract.RecapProvider = (*bundle)(nil)
