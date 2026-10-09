package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

type modelSettlementFailJournal struct {
	storage.Journal
	failType        domain.EventType
	failOnlySettled bool
	failErr         error
	mu              sync.Mutex
	attempts        map[domain.EventType]int
}

func (j *modelSettlementFailJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	for _, event := range commit.Events {
		j.mu.Lock()
		j.attempts[event.Type]++
		fail := event.Type == j.failType
		if fail && j.failOnlySettled {
			var usage struct {
				Settlement bool `json:"settlement"`
			}
			_ = json.Unmarshal(event.Payload, &usage)
			fail = usage.Settlement
		}
		j.mu.Unlock()
		if fail {
			return 0, j.failErr
		}
	}
	return j.Journal.Append(ctx, commit)
}

func (j *modelSettlementFailJournal) count(eventType domain.EventType) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.attempts[eventType]
}

type modelSettlementFixture struct {
	svc     *Service
	backend *sqlite.Backend
	journal *modelSettlementFailJournal
	obs     *runModelCallObserver
	ctx     context.Context
	meta    modelCallMeta
}

func newModelSettlementFixture(t *testing.T, failType domain.EventType, failOnlySettled bool, budget BudgetPolicy, route modelCallRoute, child bool) modelSettlementFixture {
	t.Helper()
	svc, backend, _ := newObservedService(t, &fakeChatModel{}, budget)
	sessionID := domain.SessionID("sess-settlement")
	runID := domain.RunID("run-settlement")
	mustCreateSession(t, backend, sessionID)
	if err := backend.CreateRun(context.Background(), domain.Run{
		ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	failErr := errors.New("synthetic journal settlement failure")
	journal := &modelSettlementFailJournal{
		Journal: backend, failType: failType, failOnlySettled: failOnlySettled,
		failErr: failErr, attempts: map[domain.EventType]int{},
	}
	svc.deps.Journal = journal
	mapper := newEventMapper(runID, 64<<10)
	mapper.setUsageRoutes("test", "run-model", "summary-model")
	ledger, err := NewBudgetLedger(budget)
	if err != nil {
		t.Fatal(err)
	}
	ctx := svc.withLiveModelStreamObserver(context.Background(), mapper, sessionID, ledger, nil, child, nil)
	factory := modelCallObserverFactoryFrom(ctx)
	if factory == nil {
		t.Fatal("model observer factory missing")
	}
	observer := factory(route)
	obs, ok := observer.(*runModelCallObserver)
	if !ok {
		t.Fatalf("observer type=%T, want *runModelCallObserver", observer)
	}
	meta, err := obs.Begin(ctx, modelCallInput{Mode: "stream"})
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	return modelSettlementFixture{svc: svc, backend: backend, journal: journal, obs: obs, ctx: ctx, meta: meta}
}

func TestModelSettlementJournalFailure(t *testing.T) {
	for _, test := range []struct {
		name            string
		failType        domain.EventType
		failOnlySettled bool
		withUsage       bool
		wantFinished    int
	}{
		{name: "final usage", failType: domain.EventModelUsage, failOnlySettled: true, withUsage: true},
		{name: "call finished", failType: domain.EventModelCallFinished, wantFinished: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newModelSettlementFixture(t, test.failType, test.failOnlySettled,
				BudgetPolicy{MaxEvents: 1, MaxModelCalls: 1, MaxToolCalls: 1, MaxRetries: 1}, modelCallRoute{}, false)
			if test.withUsage {
				state := fixture.obs.call(fixture.meta.CallID)
				state.usage.record(&schema.TokenUsage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6})
			}
			err := fixture.obs.End(fixture.ctx, fixture.meta, modelCallResult{ResponseComplete: true})
			if !errors.Is(err, fixture.journal.failErr) {
				t.Fatalf("End error=%v, want original Journal cause", err)
			}
			var settlementErr *modelSettlementError
			if !errors.As(err, &settlementErr) {
				t.Fatalf("End error=%T, want modelSettlementError", err)
			}
			if test.withUsage && fixture.journal.count(domain.EventModelUsage) != 1 {
				t.Fatalf("settlement usage attempts=%d, want 1", fixture.journal.count(domain.EventModelUsage))
			}
			if fixture.journal.count(domain.EventModelCallFinished) != test.wantFinished {
				t.Fatalf("finish attempts=%d, want %d", fixture.journal.count(domain.EventModelCallFinished), test.wantFinished)
			}
			run, err := fixture.backend.GetRun(context.Background(), "run-settlement")
			if err != nil || run.Status != domain.RunFailed {
				t.Fatalf("terminal run=%+v err=%v, want run.failed", run, err)
			}
			events := replayAll(t, fixture.backend, "run-settlement")
			if got := terminalTypes(events); len(got) != 1 || got[0] != domain.EventRunFailed {
				t.Fatalf("terminal events=%v, want one run.failed", got)
			}
		})
	}
}

func TestModelSettlementFailureReachesGenerate(t *testing.T) {
	t.Run("call finished", func(t *testing.T) {
		chat := &fakeChatModel{generateMsg: &schema.Message{
			Role: schema.Assistant, Content: "provider succeeded",
			ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5}},
		}}
		svc, backend, _ := newObservedService(t, chat, DefaultBudgetPolicy())
		const sessionID = domain.SessionID("sess-settlement-generate")
		const runID = domain.RunID("run-settlement-generate")
		mustCreateSession(t, backend, sessionID)
		if err := backend.CreateRun(context.Background(), domain.Run{
			ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: time.Now().UnixMilli(),
		}); err != nil {
			t.Fatal(err)
		}
		failure := errors.New("synthetic Journal settlement failure")
		journal := &modelSettlementFailJournal{
			Journal: backend, failType: domain.EventModelCallFinished,
			failErr: failure, attempts: map[domain.EventType]int{},
		}
		svc.deps.Journal = journal
		mapper := newEventMapper(runID, 64<<10)
		mapper.setUsageRoutes("test", "run-model", "summary-model")
		ledger, err := NewBudgetLedger(DefaultBudgetPolicy())
		if err != nil {
			t.Fatal(err)
		}
		ctx := svc.withLiveModelStreamObserver(context.Background(), mapper, sessionID, ledger, nil, false, nil)
		got, err := observeChatModel(chat).Generate(ctx, []*schema.Message{schema.UserMessage("hello")})
		if got != nil || !errors.Is(err, failure) {
			t.Fatalf("Generate = (%v, %v), want nil and the original Journal cause", got, err)
		}
		run, err := backend.GetRun(context.Background(), runID)
		if err != nil || run.Status != domain.RunFailed {
			t.Fatalf("run after failed Generate settlement=%+v err=%v, want run.failed", run, err)
		}
		events := replayAll(t, backend, runID)
		if got := terminalTypes(events); len(got) != 1 || got[0] != domain.EventRunFailed || len(eventsOfType(events, domain.EventRunCompleted)) != 0 {
			t.Fatalf("terminal events=%v, want one run.failed and no run.completed", got)
		}
	})
}

func TestModelSettlementRoutesAndCancellation(t *testing.T) {
	for _, test := range []struct {
		name   string
		route  modelCallRoute
		child  bool
		source string
	}{
		{name: "main", source: "main"},
		{name: "child", child: true, source: "child"},
		{name: "summary", route: modelCallRoute{Source: modelCallSourceSummary}, source: modelCallSourceSummary},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newModelSettlementFixture(t, "", false,
				BudgetPolicy{MaxEvents: 1, MaxModelCalls: 1, MaxToolCalls: 1, MaxRetries: 1}, test.route, test.child)
			ctx, cancel := context.WithCancel(fixture.ctx)
			cancel()
			err := fixture.obs.End(ctx, fixture.meta, modelCallResult{Err: context.Canceled})
			if err != nil {
				t.Fatalf("End with cancelled caller context: %v", err)
			}
			events := replayAll(t, fixture.backend, "run-settlement")
			finished := eventsOfType(events, domain.EventModelCallFinished)
			if len(finished) != 1 {
				t.Fatalf("finished events=%d, want 1", len(finished))
			}
			var payload payloadModelCallFinished
			mustUnmarshal(t, finished[0].Payload, &payload)
			if payload.Source != test.source || payload.Status != "cancelled" {
				t.Fatalf("finish payload=%+v, want source=%q and cancelled", payload, test.source)
			}
			if fixture.journal.count(domain.EventModelCallFinished) != 1 {
				t.Fatalf("finish append attempts=%d, want 1", fixture.journal.count(domain.EventModelCallFinished))
			}
		})
	}
}

func TestModelSettlementFailureIsNotRetried(t *testing.T) {
	model := &overflowScriptedModel{skipFirst: 1, failCount: 1, errText: "maximum context length exceeded"}
	svc, backend := newOverflowService(t, model)
	mustCreateSession(t, backend, "sess-settlement-retry")
	seedID, err := svc.Run(context.Background(), "sess-settlement-retry", "seed history")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, seedID, domain.RunCompleted)
	callsBefore := model.calls()

	failure := errors.New("finish append rejected")
	journal := &modelSettlementFailJournal{
		Journal: backend, failType: domain.EventModelCallFinished,
		failErr: failure, attempts: map[domain.EventType]int{},
	}
	svc.deps.Journal = journal
	runID, err := svc.Run(context.Background(), "sess-settlement-retry", "overflow plus failed settlement")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)
	if got := model.calls() - callsBefore; got != 1 {
		t.Fatalf("provider calls for failed run=%d, want 1", got)
	}
	if got := journal.count(domain.EventAutoRetryStarted); got != 0 {
		t.Fatalf("overflow retry decisions=%d, want 0 after settlement failure", got)
	}
	events := replayAll(t, backend, runID)
	if len(eventsOfType(events, domain.EventRunCompleted)) != 0 || len(eventsOfType(events, domain.EventRunFailed)) != 1 {
		t.Fatalf("run terminal events=%v, want one failed and no completed", terminalTypes(events))
	}
}

func terminalTypes(events []domain.RunEvent) []domain.EventType {
	var out []domain.EventType
	for _, event := range events {
		if event.Type.Terminal() {
			out = append(out, event.Type)
		}
	}
	return out
}

var _ storage.Journal = (*modelSettlementFailJournal)(nil)
