package storage

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

func TestWorkReceiptComparesExactIntentAndScope(t *testing.T) {
	mutation := domain.WorkMutation{SessionID: "session-A", RequestID: "operation", RequestInput: `{"objective":"task-AA","owner":"run-A"}`, Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal", Revision: 1}, Objective: "task-AA", MaxRounds: 2}
	event := domain.WorkEvent{SessionID: mutation.SessionID, RequestID: mutation.RequestID, RequestInput: mutation.RequestInput, Kind: mutation.Kind, Mutation: mutation}
	retry := mutation
	retry.ExpectedVersion = 5
	if !SameWorkRequest(event, retry) {
		t.Fatal("exact model retry against newer cursor rejected")
	}
	for _, change := range []func(*domain.WorkMutation){
		func(m *domain.WorkMutation) { m.RequestInput = `{"objective":"task-BB","owner":"run-A"}` },
		func(m *domain.WorkMutation) { m.RequestInput = `{"objective":"task-AA","owner":"run-B"}` },
		func(m *domain.WorkMutation) { m.SessionID = "session-B" },
		func(m *domain.WorkMutation) { m.Objective = "task-BB" },
		func(m *domain.WorkMutation) { m.RequestID = "different" },
	} {
		changed := mutation
		change(&changed)
		if SameWorkRequest(event, changed) {
			t.Fatalf("changed input or scope accepted: %+v", changed)
		}
	}
	event.RequestInput = ""
	if SameWorkRequest(event, mutation) {
		t.Fatal("unavailable legacy input reused")
	}
	encoded, err := json.Marshal(event)
	if err != nil || strings.Contains(string(encoded), "owner") || strings.Contains(string(encoded), "RequestInput") {
		t.Fatalf("private receipt leaked: %s %v", encoded, err)
	}
}
