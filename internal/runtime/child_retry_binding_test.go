package runtime

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// A derived child agent gets its own ModelRetryConfig closure. Without
// re-binding the decider, a delegated first call would fail on the same
// transient provider 2013 that a primary call now survives.
func TestChildEngineInheritsModelRetryDecider(t *testing.T) {
	ctx := context.Background()
	parent, err := NewEngine(ctx, NewScriptedModel(testAssistant("ok")), nil, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("build primary engine: %v", err)
	}
	decider := func(context.Context, *adk.TypedRetryContext[*schema.AgenticMessage]) *adk.TypedRetryDecision[*schema.AgenticMessage] {
		return nil
	}
	parent.SetRetryDecider(decider)

	for name, view := range map[string]func() (*Engine, error){
		"one-shot": func() (*Engine, error) { return parent.OneShotView(ctx, nil) },
		"child":    func() (*Engine, error) { return parent.ChildView(ctx, nil) },
	} {
		child, err := view()
		if err != nil {
			t.Fatalf("%s view: %v", name, err)
		}
		if child.retryDecider == nil {
			t.Fatalf("%s view lost the model retry decider", name)
		}
	}
}

// Overflow recovery folds durable session history. A child runs a delegated
// task inside the parent session, so the fold must never happen there. The
// service reserves the run's retry on entry, before compaction, so the budget
// entry proves which path was taken even with unwired compaction deps.
func TestChildModelRetrySkipsOverflowRecovery(t *testing.T) {
	overflow := provider400("maximum context length is 128000 tokens, but you requested 200000")
	svc := newDecisionService()
	svc.engine = &Engine{cfg: EngineConfig{Compaction: &CompactionPolicy{Enabled: true}}}
	input := []*schema.AgenticMessage{testUser("task")}
	retry := func(ctx context.Context) *adk.TypedRetryDecision[*schema.AgenticMessage] {
		return svc.modelRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
			RetryAttempt: 1, Err: overflow, InputMessages: input,
		})
	}

	primaryCtx := withRunID(withSessionID(context.Background(), "session-1"), "run-primary")
	if decision := retry(primaryCtx); decision != nil {
		t.Fatalf("primary overflow decision = %+v, want none without compaction deps", decision)
	}
	if _, ok := svc.modelRetries["run-primary"]; !ok {
		t.Fatal("primary overflow never entered the recovery path")
	}

	childCtx := withRunID(withSessionID(context.Background(), "session-1"), "child-overflow")
	childCtx = withChildTerminal(childCtx)
	if decision := retry(childCtx); decision != nil {
		t.Fatalf("child overflow decision = %+v, want none", decision)
	}
	if _, ok := svc.modelRetries["child-overflow"]; ok {
		t.Fatal("child overflow entered the session-folding recovery path")
	}
}

// The identical-replay decisions stay available to a child run.
func TestChildModelRetryStillRecoversBare2013(t *testing.T) {
	svc := newDecisionService()
	ctx := withRunID(withChildTerminal(context.Background()), "child-bare-2013")
	input := []*schema.AgenticMessage{testSystem("sys"), testUser("contract"), testUser("task")}

	decision := svc.modelRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 1, Err: bare2013Error(), InputMessages: input,
	})
	if decision == nil || !decision.Retry || decision.RejectReason != provider2013Reason {
		t.Fatalf("child bare 2013 decision = %+v, want one identical replay", decision)
	}
	if _, ok := svc.modelRetries["child-bare-2013"]; !ok {
		t.Fatal("child retry grant is not recorded under the child run id")
	}
}
