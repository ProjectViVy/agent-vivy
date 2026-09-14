package observerhost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/observer"
)

type receiptRunObserver struct {
	id            string
	mu            sync.Mutex
	receipts      map[observer.EventID]observer.DeliveryReceipt
	events        []observer.RunEvent
	outage        bool
	ambiguousOnce bool
	attempts      int
}

func (provider *receiptRunObserver) setOutage(value bool) {
	provider.mu.Lock()
	provider.outage = value
	provider.mu.Unlock()
}

func (provider *receiptRunObserver) ID() string { return provider.id }

func (provider *receiptRunObserver) ObserveRun(context.Context, observer.RunEvent) error {
	return errors.New("receipt-aware path required")
}

func (provider *receiptRunObserver) ObserveRunWithReceipt(_ context.Context, event observer.RunEvent) (observer.DeliveryReceipt, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.attempts++
	if provider.outage {
		return observer.DeliveryReceipt{}, errors.New("receiver unavailable")
	}
	if receipt, ok := provider.receipts[event.ID]; ok {
		return receipt, nil
	}
	receipt := observer.NewDeliveryReceipt(event.ID, "fake-receipt-1", observer.DeliveryCompleted)
	provider.receipts[event.ID] = receipt
	provider.events = append(provider.events, event)
	if provider.ambiguousOnce {
		provider.ambiguousOnce = false
		return observer.DeliveryReceipt{}, errors.New("ambiguous acknowledgement")
	}
	return receipt, nil
}

func TestSCXMemoryReturnUsesCommittedFilteredReceiptAwareDelivery(t *testing.T) {
	journal := &memoryJournal{}
	cursors := &memorySnapshots{}
	provider := &receiptRunObserver{id: "fake-memory", receipts: map[observer.EventID]observer.DeliveryReceipt{}, outage: true}
	host, err := New(Config{Journal: journal, Cursors: cursors, RunSubscriptions: []RunSubscription{{
		Provider: provider, EventTypes: []string{"run.completed", "run.cancelled"},
		AllowedPayloadFields: []string{"outcome", "result", "view"},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	if err := host.DeliverRun(context.Background(), "run-001"); err != nil {
		t.Fatal(err)
	}
	if len(provider.events) != 0 {
		t.Fatal("terminal delivery occurred before commit")
	}
	payload := json.RawMessage(`{"outcome":"completed","result":"SCX architecture discussed","view":"V1","raw_files":["plan.txt"],"secret":"sk-test-1234567890123456"}`)
	run := domain.Run{ID: "run-001", Status: domain.RunCompleted}
	_, err = journal.Append(context.Background(), storage.Commit{RunID: "run-001", Events: []domain.RunEvent{
		{Type: domain.EventModelCompleted, Payload: json.RawMessage(`{"content":"not committed terminal output"}`)},
		{Type: domain.EventRunCompleted, Payload: payload},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.DeliverRun(context.Background(), "run-001"); err == nil {
		t.Fatal("receiver outage should leave delivery pending")
	}
	if run.Status != domain.RunCompleted {
		t.Fatal("fixture terminal outcome changed during observer outage")
	}

	provider.outage = false
	provider.ambiguousOnce = true
	if err := host.DeliverRun(context.Background(), "run-001"); err == nil {
		t.Fatal("ambiguous acknowledgement should retain cursor for retry")
	}
	if err := host.DeliverRun(context.Background(), "run-001"); err != nil {
		t.Fatal(err)
	}
	if len(provider.events) != 1 {
		t.Fatalf("logical receiver updates = %d, want 1", len(provider.events))
	}
	var projected map[string]any
	if err := json.Unmarshal(provider.events[0].Payload, &projected); err != nil {
		t.Fatal(err)
	}
	if len(projected) != 3 || projected["outcome"] != "completed" || projected["view"] != "V1" {
		t.Fatalf("projected payload = %#v", projected)
	}
	if _, ok := projected["raw_files"]; ok {
		t.Fatal("raw file metadata crossed the export projection")
	}
	if _, ok := projected["secret"]; ok {
		t.Fatal("unsubscribed secret field crossed the export projection")
	}
}

func TestRunSubscriptionRejectsBroadcastConfiguration(t *testing.T) {
	_, err := New(Config{Journal: &memoryJournal{}, Cursors: &memorySnapshots{}, RunSubscriptions: []RunSubscription{{
		Provider: &recordingRunObserver{id: "bad"},
	}}})
	if !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("unscoped subscription error = %v", err)
	}
}

func TestSCXObserverWorkerResumesPendingDeliveryAfterReconnect(t *testing.T) {
	journal := &memoryJournal{}
	_, _ = journal.Append(context.Background(), storage.Commit{RunID: "run-reconnect", Events: []domain.RunEvent{{
		Type: domain.EventRunCompleted, Payload: json.RawMessage(`{"outcome":"completed"}`),
	}}})
	provider := &receiptRunObserver{id: "fake-memory", receipts: map[observer.EventID]observer.DeliveryReceipt{}, outage: true}
	host, err := New(Config{Journal: journal, Cursors: &memorySnapshots{}, RetryDelay: 5 * time.Millisecond, RunSubscriptions: []RunSubscription{{
		Provider: provider, EventTypes: []string{"run.completed"}, AllowedPayloadFields: []string{"outcome"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	host.Start(ctx)
	t.Cleanup(func() { cancel(); host.Close() })
	host.OnRunEvent(ctx, domain.RunEvent{RunID: "run-reconnect", Type: domain.EventRunCompleted})
	time.Sleep(15 * time.Millisecond)
	provider.setOutage(false)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		provider.mu.Lock()
		delivered := len(provider.events)
		provider.mu.Unlock()
		if delivered == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("pending delivery did not resume after receiver reconnect")
}

func TestSCXObserverRestartRecoversCommittedPendingRunAndCancelledVariant(t *testing.T) {
	journal := &memoryJournal{}
	cursors := &memorySnapshots{}
	_, _ = journal.Append(context.Background(), storage.Commit{RunID: "run-cancelled", Events: []domain.RunEvent{{
		Type: domain.EventRunCancelled, Payload: json.RawMessage(`{"outcome":"cancelled","reason":"user_requested","tenant_id":"local","workspace_id":"w1","session_id":"s1"}`),
	}}})
	provider := &receiptRunObserver{id: "fake-memory", receipts: map[observer.EventID]observer.DeliveryReceipt{}}
	host, err := New(Config{Journal: journal, Cursors: cursors, RecoverRunIDs: []domain.RunID{"run-cancelled"}, RetryDelay: 5 * time.Millisecond, RunSubscriptions: []RunSubscription{{
		Provider: provider, EventTypes: []string{"run.cancelled"}, AllowedPayloadFields: []string{"outcome", "reason", "tenant_id", "workspace_id", "session_id"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	host.Start(ctx)
	t.Cleanup(func() { cancel(); host.Close() })
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		provider.mu.Lock()
		if len(provider.events) == 1 {
			event := provider.events[0]
			provider.mu.Unlock()
			if event.Type != "run.cancelled" || !json.Valid(event.Payload) || !strings.Contains(string(event.Payload), `"session_id":"s1"`) {
				t.Fatalf("recovered cancelled projection = %#v", event)
			}
			return
		}
		provider.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("restart recovery did not deliver the committed cancelled run")
}

func TestSCXObserverStructuralRedactionAlwaysProducesValidJSON(t *testing.T) {
	raw := json.RawMessage(`{"summary":"authorization: secret-value","nested":{"token":"sk-test-12345678901234567890","api_key":"plain-secret"}}`)
	projected, err := projectPayload(raw, []string{"summary", "nested"})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(projected) || strings.Contains(string(projected), "secret-value") || strings.Contains(string(projected), "sk-test-") || strings.Contains(string(projected), "plain-secret") {
		t.Fatalf("structurally redacted payload = %q", projected)
	}
}

func TestRetryDelayForDoublesPerAttemptAndCapsAtMax(t *testing.T) {
	base := 5 * time.Millisecond
	max := 200 * time.Millisecond
	for _, tc := range []struct {
		attempts int
		want     time.Duration
	}{
		{attempts: 0, want: base},
		{attempts: 1, want: base},
		{attempts: 2, want: 10 * time.Millisecond},
		{attempts: 3, want: 20 * time.Millisecond},
		{attempts: 6, want: 160 * time.Millisecond},
		{attempts: 7, want: max},
		{attempts: 50, want: max},
	} {
		if got := retryDelayFor(base, max, tc.attempts); got != tc.want {
			t.Fatalf("retryDelayFor(attempts=%d) = %v, want %v", tc.attempts, got, tc.want)
		}
	}
	if got := retryDelayFor(100*time.Millisecond, 50*time.Millisecond, 3); got != 50*time.Millisecond {
		t.Fatalf("retryDelayFor(max below base) = %v, want max", got)
	}
}

func TestSCXObserverRetryBackoffBoundsPermanentFailureAttempts(t *testing.T) {
	journal := &memoryJournal{}
	_, _ = journal.Append(context.Background(), storage.Commit{RunID: "run-stuck", Events: []domain.RunEvent{{
		Type: domain.EventRunCompleted, Payload: json.RawMessage(`{"outcome":"completed"}`),
	}}})
	provider := &receiptRunObserver{id: "fake-stuck", receipts: map[observer.EventID]observer.DeliveryReceipt{}, outage: true}
	host, err := New(Config{Journal: journal, Cursors: &memorySnapshots{}, RetryDelay: 5 * time.Millisecond, MaxRetryDelay: 200 * time.Millisecond, RunSubscriptions: []RunSubscription{{
		Provider: provider, EventTypes: []string{"run.completed"}, AllowedPayloadFields: []string{"outcome"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	host.Start(ctx)
	t.Cleanup(func() { cancel(); host.Close() })
	host.OnRunEvent(ctx, domain.RunEvent{RunID: "run-stuck", Type: domain.EventRunCompleted})
	time.Sleep(250 * time.Millisecond)
	provider.mu.Lock()
	attempts := provider.attempts
	provider.mu.Unlock()
	// Without backoff a permanently failing receiver is hit roughly every
	// RetryDelay; the doubling schedule must keep the 250ms window far below
	// that while still retrying at least twice.
	if attempts < 2 || attempts > 8 {
		t.Fatalf("permanent-failure attempts in 250ms = %d, want 2..8", attempts)
	}
}
