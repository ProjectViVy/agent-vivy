package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	laputaevolution "github.com/dashimaki/laputa/evolution"

	"agent-vivy/internal/storage"
)

// Pause a real Snapshot.Get after it captures its value. The fixture only
// controls scheduling; SQLite still owns every version, value and CAS write.
type cognitiveSnapshotReadGate struct {
	storage.SnapshotStore
	mu      sync.Mutex
	armed   bool
	reached chan struct{}
	release chan struct{}
}

func (g *cognitiveSnapshotReadGate) Get(ctx context.Context, key string) ([]byte, int64, error) {
	raw, version, err := g.SnapshotStore.Get(ctx, key)
	g.mu.Lock()
	pause := g.armed && key == cognitiveStateKey
	if pause {
		g.armed = false
	}
	g.mu.Unlock()
	if pause {
		close(g.reached)
		select {
		case <-g.release:
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		}
	}
	return raw, version, err
}

func newCognitiveSnapshotGate(store storage.SnapshotStore) *cognitiveSnapshotReadGate {
	return &cognitiveSnapshotReadGate{SnapshotStore: store, armed: true, reached: make(chan struct{}), release: make(chan struct{})}
}

// Give the second transaction an opportunity to finish before releasing the
// first captured read. A serialized implementation keeps it queued. This is
// a controller concurrency unit, not a process-crash-cut timing assertion.
func releaseCognitiveReadGate(t *testing.T, g *cognitiveSnapshotReadGate, second <-chan error) error {
	t.Helper()
	select {
	case err := <-second:
		close(g.release)
		return err
	case <-time.After(200 * time.Millisecond):
		close(g.release)
		select {
		case err := <-second:
			return err
		case <-time.After(5 * time.Second):
			t.Fatal("second cognitive transaction did not finish")
			return nil
		}
	}
}

func TestCognitivePolicyCASHasOneConcurrentWinner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	g := newCognitiveSnapshotGate(backend.Snapshot())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, nil, g, nil)
	first, second := make(chan error, 1), make(chan error, 1)
	go func() { first <- svc.UpdateCognitivePolicyCAS(ctx, laputaevolution.TriggerPolicy{Enabled: false}, 0) }()
	select {
	case <-g.reached:
	case <-ctx.Done():
		t.Fatal("first policy read not reached")
	}
	go func() {
		second <- svc.UpdateCognitivePolicyCAS(ctx, laputaevolution.TriggerPolicy{Enabled: true, MinIntervalMS: 99}, 0)
	}()
	secondErr := releaseCognitiveReadGate(t, g, second)
	firstErr := <-first
	winners, conflicts := 0, 0
	for _, err := range []error{firstErr, secondErr} {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrPolicyConflict):
			conflicts++
		default:
			t.Fatalf("unexpected policy error: %v", err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("same revision admitted %d writers and %d conflicts", winners, conflicts)
	}
}

func TestCognitivePolicyWritePreservesAcceptedSourceHigh(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	g := newCognitiveSnapshotGate(backend.Snapshot())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, nil, g, nil)
	policy, notify := make(chan error, 1), make(chan error, 1)
	go func() { policy <- svc.UpdateCognitivePolicyCAS(ctx, laputaevolution.TriggerPolicy{Enabled: false}, 0) }()
	select {
	case <-g.reached:
	case <-ctx.Done():
		t.Fatal("policy read not reached")
	}
	go func() { notify <- svc.NotifyCognitiveInput(ctx, 7) }()
	if err := releaseCognitiveReadGate(t, g, notify); err != nil {
		t.Fatal(err)
	}
	if err := <-policy; err != nil {
		t.Fatal(err)
	}
	st, _, err := svc.loadCognitiveState(ctx)
	if err != nil || st.SourceHigh != 7 || st.Policy.Enabled || st.PolicyRevision != 1 {
		t.Fatalf("accepted input or policy lost: %+v %v", st, err)
	}
}

func TestCognitivePolicyWritePreservesAdmittedWindow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	domainGate := make(chan struct{})
	t.Cleanup(func() { close(domainGate) })
	svc, backend := inofyExecService(t, cognitiveTestModel())
	g := newCognitiveSnapshotGate(backend.Snapshot())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{gate: domainGate}, &fakeSource{high: 5}, g, nil)
	policy, admission := make(chan error, 1), make(chan error, 1)
	go func() {
		policy <- svc.UpdateCognitivePolicyCAS(ctx, laputaevolution.TriggerPolicy{Enabled: true, MinIntervalMS: 55}, 0)
	}()
	select {
	case <-g.reached:
	case <-ctx.Done():
		t.Fatal("policy read not reached")
	}
	go func() {
		elig, err := svc.TriggerCognitive(ctx)
		if err == nil && !elig.Run {
			err = errors.New("fixture: manual admission was not eligible")
		}
		admission <- err
	}()
	if err := releaseCognitiveReadGate(t, g, admission); err != nil {
		t.Fatal(err)
	}
	if err := <-policy; err != nil {
		t.Fatal(err)
	}
	runs := listWorkflowRuns(t, svc, backend)
	st, _, err := svc.loadCognitiveState(ctx)
	if err != nil || len(runs) != 1 || st.ActiveRunID != string(runs[0].ID) || st.PendingThrough != 5 || st.PolicyRevision != 1 || st.Policy.MinIntervalMS != 55 {
		t.Fatalf("policy write lost an admitted window: state=%+v runs=%+v err=%v", st, runs, err)
	}
}

func TestCognitiveConcurrentManualAndAutomaticWakesCoalesce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	domainGate := make(chan struct{})
	d := &fakeCognitiveDomain{gate: domainGate}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	t.Cleanup(func() { close(domainGate) })
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 5}, backend.Snapshot(), nil)
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)
	start, done := make(chan struct{}), make(chan error, 13)
	for i := 0; i < 12; i++ {
		go func() {
			<-start
			_, err := svc.TriggerCognitive(ctx)
			done <- err
		}()
	}
	go func() {
		<-start
		done <- svc.NotifyCognitiveInput(ctx, 5)
	}()
	close(start)
	for i := 0; i < 13; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("concurrent wake did not finish")
		}
	}
	runs := listWorkflowRuns(t, svc, backend)
	st, _, err := svc.loadCognitiveState(ctx)
	if err != nil || len(runs) != 1 || st.ActiveRunID != string(runs[0].ID) || st.PendingThrough != 5 || st.SourceHigh != 5 || st.Watermark != 0 {
		t.Fatalf("overlapping wake admitted extra windows or lost state: state=%+v runs=%+v err=%v", st, runs, err)
	}
}
