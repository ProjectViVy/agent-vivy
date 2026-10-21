package runtime

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type workExactReceiptStore struct {
	storage.WorkStore
	event domain.WorkEvent
}

func (s workExactReceiptStore) CommitWork(context.Context, domain.WorkMutation) (storage.WorkCommitResult, error) {
	return storage.WorkCommitResult{Event: s.event}, nil
}

func TestWorkPublicationRemovesPrivateReceiptFromNativeSink(t *testing.T) {
	mutation := domain.WorkMutation{SessionID: "session", RequestID: "operation", RequestInput: `{"owner":"private-run","intent":"create"}`, Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal", Revision: 1}, Objective: "task", MaxRounds: 1}
	event := domain.WorkEvent{SessionID: mutation.SessionID, RequestID: mutation.RequestID, RequestInput: mutation.RequestInput, Kind: mutation.Kind, Mutation: mutation}
	var published domain.WorkEvent
	service := &Service{deps: ServiceDeps{Work: workExactReceiptStore{event: event}, WorkSink: workEventSinkFunc(func(value domain.WorkEvent) { published = value })}}
	result, err := service.CommitWork(context.Background(), mutation)
	if err != nil {
		t.Fatal(err)
	}
	if published.RequestInput != "" || published.Mutation.RequestInput != "" {
		t.Fatalf("native sink received private receipt: %+v", published)
	}
	if result.Event.RequestInput != mutation.RequestInput || result.Event.Mutation.RequestInput != mutation.RequestInput {
		t.Fatal("private durable receipt was altered by publication")
	}
}
