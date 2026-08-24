package runtime

import (
	"testing"

	"agent-vivy/internal/domain"
)

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
	if first.Hash == second.Hash || first.Hash == "" {
		t.Fatalf("snapshots = %+v / %+v, want distinct non-empty hashes", first, second)
	}
}
