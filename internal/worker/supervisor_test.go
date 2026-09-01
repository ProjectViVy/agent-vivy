package worker

import (
	"slices"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/logging"
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

func TestWorkerLogEnv(t *testing.T) {
	env := workerLogEnv(WorkerLog{Dir: "tmp/logs", Level: "warn", Format: "text"})
	want := []string{
		logging.EnvWorkerLogDir + "=tmp/logs",
		logging.EnvWorkerLogLevel + "=warn",
		logging.EnvWorkerLogFormat + "=text",
	}
	if !slices.Equal(env, want) {
		t.Fatalf("workerLogEnv = %v, want %v", env, want)
	}
	if got := workerLogEnv(WorkerLog{}); got != nil {
		t.Errorf("empty handoff = %v, want no env entries", got)
	}
	partial := workerLogEnv(WorkerLog{Dir: "tmp/logs"})
	if !slices.Equal(partial, []string{logging.EnvWorkerLogDir + "=tmp/logs"}) {
		t.Errorf("partial handoff = %v, want only the dir entry", partial)
	}
}
