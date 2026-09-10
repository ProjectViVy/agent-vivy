package observerhost

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/observer"
)

type memoryJournal struct {
	mu     sync.Mutex
	events map[domain.RunID][]domain.RunEvent
}

func (journal *memoryJournal) Append(_ context.Context, commit storage.Commit) (domain.EventSeq, error) {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if journal.events == nil {
		journal.events = map[domain.RunID][]domain.RunEvent{}
	}
	seq := domain.EventSeq(len(journal.events[commit.RunID]))
	for _, event := range commit.Events {
		seq++
		event.RunID = commit.RunID
		event.Seq = seq
		journal.events[commit.RunID] = append(journal.events[commit.RunID], event)
	}
	return seq, nil
}

func (journal *memoryJournal) Replay(_ context.Context, runID domain.RunID, after domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	entries := []storage.Entry{}
	for _, event := range journal.events[runID] {
		if event.Seq > after {
			entries = append(entries, storage.Entry{Event: event})
		}
	}
	return &memoryIterator{entries: entries, index: -1}, nil
}

type memoryIterator struct {
	entries []storage.Entry
	index   int
}

func (iterator *memoryIterator) Next() bool {
	iterator.index++
	return iterator.index < len(iterator.entries)
}
func (iterator *memoryIterator) Value() storage.Entry { return iterator.entries[iterator.index] }
func (*memoryIterator) Err() error                    { return nil }
func (*memoryIterator) Close() error                  { return nil }

type memorySnapshots struct {
	mu       sync.Mutex
	values   map[string][]byte
	versions map[string]int64
	failPut  int
}

func (snapshots *memorySnapshots) Get(_ context.Context, key string) ([]byte, int64, error) {
	snapshots.mu.Lock()
	defer snapshots.mu.Unlock()
	return append([]byte(nil), snapshots.values[key]...), snapshots.versions[key], nil
}
func (snapshots *memorySnapshots) Put(_ context.Context, key string, value []byte, expected int64) error {
	snapshots.mu.Lock()
	defer snapshots.mu.Unlock()
	if snapshots.failPut > 0 {
		snapshots.failPut--
		return errors.New("cursor store unavailable")
	}
	if snapshots.values == nil {
		snapshots.values = map[string][]byte{}
		snapshots.versions = map[string]int64{}
	}
	if snapshots.versions[key] != expected {
		return storage.ErrVersionConflict
	}
	snapshots.values[key] = append([]byte(nil), value...)
	snapshots.versions[key]++
	return nil
}

type recordingRunObserver struct {
	id     string
	mu     sync.Mutex
	events []observer.RunEvent
	err    error
}

func (provider *recordingRunObserver) ID() string { return provider.id }
func (provider *recordingRunObserver) ObserveRun(_ context.Context, event observer.RunEvent) error {
	provider.mu.Lock()
	provider.events = append(provider.events, event)
	provider.mu.Unlock()
	return provider.err
}

type blockingDiagnosticObserver struct {
	id      string
	started chan struct{}
	block   chan struct{}
}

func (provider *blockingDiagnosticObserver) ID() string { return provider.id }
func (provider *blockingDiagnosticObserver) ObserveDiagnostic(context.Context, observer.Diagnostic) {
	select {
	case provider.started <- struct{}{}:
	default:
	}
	<-provider.block
}

func TestRunObserverSeesOnlyCommittedEvents(t *testing.T) {
	journal := &memoryJournal{}
	cursors := &memorySnapshots{}
	provider := &recordingRunObserver{id: "fixture/run"}
	host, err := New(Config{Journal: journal, Cursors: cursors, RunProviders: []observer.RunProvider{provider}})
	if err != nil {
		t.Fatal(err)
	}

	if err := host.DeliverRun(context.Background(), "run-1"); err != nil {
		t.Fatal(err)
	}
	if len(provider.events) != 0 {
		t.Fatalf("observer saw %d uncommitted events", len(provider.events))
	}

	_, err = journal.Append(context.Background(), storage.Commit{RunID: "run-1", Events: []domain.RunEvent{{
		Type:      domain.EventType("tool.completed"),
		CreatedAt: 123,
		Payload:   json.RawMessage(`{"secret":"sk-test-1234567890123456"}`),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.DeliverRun(context.Background(), "run-1"); err != nil {
		t.Fatal(err)
	}
	if len(provider.events) != 1 || provider.events[0].ID.String() != "run-1:1" {
		t.Fatalf("observer events = %#v", provider.events)
	}
	if string(provider.events[0].Payload) == `{"secret":"sk-test-1234567890123456"}` {
		t.Fatal("observer payload was not redacted")
	}
}

func TestRunObserverCanReceiveDuplicateStableEventID(t *testing.T) {
	journal := &memoryJournal{}
	_, _ = journal.Append(context.Background(), storage.Commit{RunID: "run-dup", Events: []domain.RunEvent{{Type: domain.EventType("run.started")}}})
	cursors := &memorySnapshots{failPut: 1}
	provider := &recordingRunObserver{id: "fixture/run"}
	host, err := New(Config{Journal: journal, Cursors: cursors, RunProviders: []observer.RunProvider{provider}})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.DeliverRun(context.Background(), "run-dup"); err == nil {
		t.Fatal("cursor persistence failure should surface to delivery worker")
	}
	if err := host.DeliverRun(context.Background(), "run-dup"); err != nil {
		t.Fatal(err)
	}
	if len(provider.events) != 2 {
		t.Fatalf("observer deliveries = %d, want 2", len(provider.events))
	}
	if provider.events[0].ID != provider.events[1].ID {
		t.Fatalf("duplicate ids differ: %#v vs %#v", provider.events[0].ID, provider.events[1].ID)
	}
}

func TestDiagnosticOverloadIncrementsDropCounterWithoutBlocking(t *testing.T) {
	provider := &blockingDiagnosticObserver{id: "fixture/diag", started: make(chan struct{}, 1), block: make(chan struct{})}
	host, err := New(Config{DiagnosticProviders: []observer.DiagnosticProvider{provider}, DiagnosticBuffer: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host.Start(ctx)
	defer func() {
		close(provider.block)
		host.Close()
	}()

	host.EmitDiagnostic(observer.NewDiagnostic("lsp", "warn", "one", nil))
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("diagnostic worker did not start")
	}
	host.EmitDiagnostic(observer.NewDiagnostic("lsp", "warn", "two", nil))
	started := time.Now()
	host.EmitDiagnostic(observer.NewDiagnostic("lsp", "warn", "three", nil))
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("diagnostic overload blocked caller")
	}
	if got := host.DiagnosticDrops("fixture/diag"); got != 1 {
		t.Fatalf("diagnostic drops = %d, want 1", got)
	}
}
