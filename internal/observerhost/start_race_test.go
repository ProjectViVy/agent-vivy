package observerhost

import (
	"context"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
)

func TestStartWithConcurrentRecoveryNotification(t *testing.T) {
	for repeat := 0; repeat < 200; repeat++ {
		h, err := New(Config{Journal: &memoryJournal{}, Cursors: &memorySnapshots{}, RunSubscriptions: []RunSubscription{{Provider: &recordingRunObserver{id: "fixture/start"}, EventTypes: []string{"run.started"}}}, RecoverRunIDs: []domain.RunID{"recovered"}})
		if err != nil {
			t.Fatal(err)
		}
		var worker sync.WaitGroup
		worker.Add(1)
		go func() {
			defer worker.Done()
			for i := 0; i < 32; i++ {
				h.OnRunEvent(context.Background(), domain.RunEvent{RunID: "recovered"})
			}
		}()
		h.Start(context.Background())
		worker.Wait()
		h.Close()
	}
}
