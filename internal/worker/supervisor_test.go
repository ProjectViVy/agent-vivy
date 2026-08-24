package worker

import (
	"testing"

	"agent-vivy/internal/domain"
)

func TestAuthorityPinsParentPolicyAndWorkspace(t *testing.T) {
	authority := Authority{
		ParentRunID: "parent-1",
		Snapshot:    domain.PolicySnapshot{Profile: domain.PolicyProfileDefault, Hash: "hash-1"},
		WorkspaceID: "workspace-1",
	}
	valid := Spec{
		RunID: "child-1", ParentRunID: "parent-1", PolicyProfile: domain.PolicyProfileDefault,
		PolicyHash: "hash-1", WorkspaceID: "workspace-1",
	}
	if err := authority.Validate(valid); err != nil {
		t.Fatalf("valid authority rejected: %v", err)
	}
	for name, mutate := range map[string]func(*Spec){
		"parent":    func(spec *Spec) { spec.ParentRunID = "other-parent" },
		"profile":   func(spec *Spec) { spec.PolicyProfile = domain.PolicyProfileFullAuto },
		"hash":      func(spec *Spec) { spec.PolicyHash = "other-hash" },
		"workspace": func(spec *Spec) { spec.WorkspaceID = "other-workspace" },
	} {
		t.Run(name, func(t *testing.T) {
			spec := valid
			mutate(&spec)
			if err := authority.Validate(spec); err == nil {
				t.Fatal("authority widening must be rejected")
			}
		})
	}
}
