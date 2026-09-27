package app

import (
	"context"
	"strings"
	"testing"
)

func TestStartAgentTaskRequiresWiredManager(t *testing.T) {
	ref := &agentToolRef{}
	if _, err := ref.StartAgentTask(context.Background(), "task"); err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Fatalf("err = %v, want not-wired failure", err)
	}
}

func TestStartAgentTaskRequiresRunScopedParent(t *testing.T) {
	ref := &agentToolRef{}
	ref.arm(newWorkerManager(nil, nil))
	if _, err := ref.StartAgentTask(context.Background(), "task"); err == nil || !strings.Contains(err.Error(), "run-scoped") {
		t.Fatalf("err = %v, want run-scoped parent failure", err)
	}
}
