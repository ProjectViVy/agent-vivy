package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
)

func defaultPolicyForTest(t *testing.T, profile domain.PolicyProfile) domain.PolicySnapshot {
	t.Helper()
	engine, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := engine.Snapshot(profile)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestAdmittedPolicyRetainsPrivateDefinitionAndRejectsDrift(t *testing.T) {
	const private = "D08-private-policy-path-canary"
	engine, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault: {Rules: []PolicyRule{{Tool: "write_note", Field: "path", Prefix: private, Decision: domain.PolicyDeny}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := engine.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	var actual PolicyDefinition
	if err := json.Unmarshal([]byte(admitted.Definition), &actual); err != nil || len(actual.Rules) != 1 || actual.Rules[0].Prefix != private {
		t.Fatal("snapshot did not preserve actual policy")
	}
	ctx := withPolicySnapshot(context.Background(), admitted)
	if err := validateAdmittedPolicy(ctx, admitted); err != nil {
		t.Fatal(err)
	}
	changed := defaultPolicyForTest(t, domain.PolicyProfileDefault)
	if err := validateAdmittedPolicy(ctx, changed); !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("policy drift accepted: %v", err)
	}
	if policySnapshot(ctx) != admitted {
		t.Fatal("current configuration replaced admission")
	}
}

func TestPolicyDenyWinsOverAllowRegardlessOfRuleOrder(t *testing.T) {
	engine, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault: {
			Rules: []PolicyRule{
				{Tool: "write_note", Decision: domain.PolicyAllow},
				{Tool: "*", Field: "path", Prefix: "..", Decision: domain.PolicyDeny, Reason: "path escape"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	eval, err := engine.Evaluate(domain.PolicyProfileDefault, domain.ToolSpec{Name: "write_note", Readonly: false}, []byte(`{"path":"../outside"}`))
	if err != nil {
		t.Fatal(err)
	}
	if eval.Decision != domain.PolicyDeny || eval.Reason != "path escape" {
		t.Fatalf("evaluation = %+v, want deny/path escape", eval)
	}
}

func TestPolicyProfilesPreserveReadonlyAndEffectfulDefaults(t *testing.T) {
	engine, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	readonly := domain.ToolSpec{Name: "echo_info", Readonly: true}
	effectful := domain.ToolSpec{Name: "write_note"}
	cases := []struct {
		profile domain.PolicyProfile
		tool    domain.ToolSpec
		want    domain.PolicyDecision
	}{
		{domain.PolicyProfileDefault, readonly, domain.PolicyAllow},
		{domain.PolicyProfileDefault, effectful, domain.PolicyPrompt},
		{domain.PolicyProfilePlan, readonly, domain.PolicyAllow},
		{domain.PolicyProfilePlan, effectful, domain.PolicyDeny},
		{domain.PolicyProfileReadOnly, effectful, domain.PolicyDeny},
		{domain.PolicyProfileFullAuto, effectful, domain.PolicyAllow},
	}
	for _, tc := range cases {
		t.Run(string(tc.profile)+"/"+tc.tool.Name, func(t *testing.T) {
			eval, err := engine.Evaluate(tc.profile, tc.tool, nil)
			if err != nil {
				t.Fatal(err)
			}
			if eval.Decision != tc.want {
				t.Fatalf("decision = %q, want %q", eval.Decision, tc.want)
			}
		})
	}
}

func TestPolicySnapshotChangesWhenDefinitionChanges(t *testing.T) {
	one, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault: {Rules: []PolicyRule{{Tool: "echo_info", Decision: domain.PolicyDeny}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault: {Rules: []PolicyRule{{Tool: "echo_info", Decision: domain.PolicyPrompt}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := one.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	second, err := two.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	if first.Definition == second.Definition || first.Definition == "" {
		t.Fatalf("snapshots = %+v / %+v, want distinct exact definitions", first, second)
	}
}

func TestPolicyToolPrefixRuleMatchesModuleActions(t *testing.T) {
	engine, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault: {
			Rules: []PolicyRule{
				{Tool: "vivy.masks.*", Decision: domain.PolicyAllow, Reason: "mask preference"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tool string
		want domain.PolicyDecision
	}{
		{"vivy.masks.selection.set", domain.PolicyAllow},
		{"vivy.masks.catalog.list", domain.PolicyAllow},
		{"vivy.memory.add", domain.PolicyPrompt},
		// The prefix ends at the asterisk: a sibling namespace must not match.
		{"vivy.masksx.eviltwin", domain.PolicyPrompt},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			eval, err := engine.Evaluate(domain.PolicyProfileDefault, domain.ToolSpec{Name: tc.tool, Readonly: false}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if eval.Decision != tc.want {
				t.Fatalf("decision = %q (%s), want %q", eval.Decision, eval.Reason, tc.want)
			}
		})
	}
}
