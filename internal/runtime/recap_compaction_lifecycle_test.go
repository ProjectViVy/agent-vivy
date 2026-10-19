package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"github.com/cloudwego/eino/schema"
)

func issue58BlockedCompaction(t *testing.T) (*issue58Wire, func(), <-chan error) {
	t.Helper()
	f := newIssue58Wire(t)
	issue58CompletedText(t, f, "lifecycle-source", "lifecycle-a", strings.Repeat("A", 10000))
	f.completed(t, "lifecycle-tail", "lifecycle-b")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unlock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unlock)
	f.reply = func(int) *schema.Message {
		close(entered)
		<-release
		return schema.AssistantMessage("blocked native summary", nil)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.CompactSession(context.Background(), "main-reader", CompactOptions{})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		unlock()
		t.Fatal("summary did not enter actual HTTP")
	}
	return f, unlock, done
}

func TestIssue58ExplicitCompactionLifecycle(t *testing.T) {
	for _, kind := range []string{"drain-cancel", "model-switch", "engine-reload", "background-recovery", "toolkit-drain"} {
		t.Run(kind, func(t *testing.T) {
			f, release, done := issue58BlockedCompaction(t)
			defer release()
			switch kind {
			case "drain-cancel":
				waitCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				idle := f.svc.WaitIdle(waitCtx)
				cancel()
				if idle {
					release()
					<-done
					t.Fatal("Service.WaitIdle reported drained while explicit native HTTP was blocked")
				}
				f.svc.CancelAll()
				waitCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
				idle = f.svc.WaitIdle(waitCtx)
				cancel()
				if !idle {
					t.Fatal("CancelAll did not drain explicit native observer")
				}
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("cancelled compaction error %v", err)
					}
				case <-time.After(time.Second):
					t.Fatal("compaction did not observe cancellation")
				}
				var finished bool
				for _, e := range f.svc.deps.Sink.(*testSink).snapshot() {
					if strings.HasPrefix(string(e.RunID), "sessctl_recap_") && e.Type == domain.EventModelCallFinished {
						finished = true
					}
				}
				if !finished {
					t.Fatal("drain preceded cancellation closure")
				}
				return
			case "background-recovery":
				if err := f.svc.RecoverBackground(context.Background()); !errors.Is(err, ErrRecoveryBusy) {
					t.Fatalf("background recovery ignored active explicit control: %v", err)
				}
			case "toolkit-drain":
				if err := f.svc.BeginDrain(context.Background(), "recap-control-drain"); err != nil {
					t.Fatal(err)
				}
				defer f.svc.EndDrain("recap-control-drain")
				if idle, err := f.svc.toolkitDrainState(context.Background(), "recap-control-drain"); err != nil || idle {
					t.Fatalf("toolkit drain ignored active explicit control: idle=%v err=%v", idle, err)
				}
				if _, err := f.svc.CompactSession(context.Background(), "main-reader", CompactOptions{}); err == nil {
					t.Fatal("toolkit drain admitted new explicit control")
				}
			case "model-switch":
				persisted := false
				err := f.svc.ChangeModelWhenIdle("fixture", "other-model", func() error { persisted = true; return nil })
				if !errors.Is(err, ErrModelChangeBusy) || persisted {
					release()
					<-done
					t.Fatalf("model changed during explicit HTTP: %v persisted=%v", err, persisted)
				}
			case "engine-reload":
				old := f.svc.engine
				var rebuilt atomic.Int32
				f.svc.deps.RebuildEngine = func(ctx context.Context, cfg EngineConfig) (*Engine, error) {
					rebuilt.Add(1)
					return NewEngine(ctx, old.chatModel, old.activeTools, cfg)
				}
				next := old.cfg
				next.MaxContextBytes = 24000
				if err := f.svc.ScheduleEngineReload(next); err != nil {
					t.Fatal(err)
				}
				if rebuilt.Load() != 0 || f.svc.engine != old {
					release()
					<-done
					t.Fatal("engine reload ignored active explicit control")
				}
			}
			release()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if kind == "engine-reload" {
				if err := f.svc.applyPendingEngineReload(context.Background(), nil); err != nil {
					t.Fatal(err)
				}
				if f.svc.engine.cfg.MaxContextBytes != 24000 {
					t.Fatal("deferred reload never applied")
				}
			}
		})
	}
}

func TestIssue58ExplicitCompactionStoppingFence(t *testing.T) {
	f := newIssue58Wire(t)
	issue58CompletedText(t, f, "stopping-source", "stopping-a", strings.Repeat("A", 10000))
	f.completed(t, "stopping-tail", "stopping-b")
	f.svc.StopAutomaticWork()
	if _, err := f.svc.CompactSession(context.Background(), "main-reader", CompactOptions{}); err == nil {
		t.Fatal("stopping Service admitted explicit summary")
	}
	if len(f.capture()) != 0 {
		t.Fatal("stopping Service invoked provider")
	}
}
