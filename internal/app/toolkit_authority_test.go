package app

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	actionport "agent-vivy/sdk/port/controlaction"
)

func TestToolkitHumanOriginAdmitsDirectManagementWithoutWeakeningPolicy(t *testing.T) {
	definition := actionport.Definition{
		ID: "vivy.bundles.preview", Owner: "vivy/toolkits", Effect: actionport.EffectExternalEffect,
	}
	input := json.RawMessage(`{"location":"/a/source"}`)
	identity := actionhost.Identity{ID: "face/connection", SessionID: "sess_human"}

	t.Run("authenticated direct control is admitted under the default profile", func(t *testing.T) {
		policy, err := runtime.NewPolicyEngine(nil)
		if err != nil {
			t.Fatal(err)
		}
		authorize := withToolkitHumanOrigin(actionAuthorize(policy, domain.PolicyProfileDefault), policy, domain.PolicyProfileDefault)
		if err := authorize(context.Background(), identity, definition, input); err != nil {
			t.Fatalf("direct human management action denied: %v", err)
		}
	})

	for name, actor := range map[string]actionhost.Identity{
		"anonymous":  {},
		"no session": {ID: "face/connection"},
		"agent run":  {ID: "face/connection", SessionID: "sess_human", RunID: "run_agent"},
	} {
		t.Run(name, func(t *testing.T) {
			policy, err := runtime.NewPolicyEngine(nil)
			if err != nil {
				t.Fatal(err)
			}
			authorize := withToolkitHumanOrigin(actionAuthorize(policy, domain.PolicyProfileDefault), policy, domain.PolicyProfileDefault)
			if err := authorize(context.Background(), actor, definition, input); err == nil {
				t.Fatal("unbound or agent action was admitted as human control")
			}
		})
	}

	t.Run("another owner is not covered by the exemption", func(t *testing.T) {
		policy, err := runtime.NewPolicyEngine(nil)
		if err != nil {
			t.Fatal(err)
		}
		authorize := withToolkitHumanOrigin(actionAuthorize(policy, domain.PolicyProfileDefault), policy, domain.PolicyProfileDefault)
		other := definition
		other.Owner = "vivy/memory"
		if err := authorize(context.Background(), identity, other, input); err == nil {
			t.Fatal("human-origin exemption escaped the Toolkits owner")
		}
	})

	for _, decision := range []domain.PolicyDecision{domain.PolicyDeny, domain.PolicyPrompt} {
		t.Run("explicit "+string(decision)+" rule remains decisive", func(t *testing.T) {
			policy, err := runtime.NewPolicyEngine(map[domain.PolicyProfile]runtime.PolicyDefinition{
				domain.PolicyProfileDefault: {Rules: []runtime.PolicyRule{{Tool: definition.ID, Decision: decision}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			authorize := withToolkitHumanOrigin(actionAuthorize(policy, domain.PolicyProfileDefault), policy, domain.PolicyProfileDefault)
			if err := authorize(context.Background(), identity, definition, input); err == nil {
				t.Fatal("explicit operator policy rule was bypassed")
			}
		})
	}

	t.Run("approval-marked provider remains gated", func(t *testing.T) {
		policy, err := runtime.NewPolicyEngine(nil)
		if err != nil {
			t.Fatal(err)
		}
		authorize := withToolkitHumanOrigin(actionAuthorize(policy, domain.PolicyProfileDefault), policy, domain.PolicyProfileDefault)
		marked := definition
		marked.RequiresApproval = true
		if err := authorize(context.Background(), identity, marked, input); err == nil {
			t.Fatal("provider approval requirement was bypassed")
		}
	})
}
