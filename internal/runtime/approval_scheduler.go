package runtime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"agent-vivy/internal/storage"
)

// ApprovalSweepEvent is emitted when the scheduler expires approvals.
type ApprovalSweepEvent struct {
	ExpiredCount int
	Timestamp    time.Time
}

// ApprovalScheduler periodically scans for expired approval requests and
// marks them as expired. This prevents stale approvals from accumulating
// and ensures timely cleanup of timed-out requests (D-021).
type ApprovalScheduler struct {
	store         storage.ApprovalTimeoutStore
	checkInterval time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
	eventHandler  func(ApprovalSweepEvent)
	logger        *slog.Logger
}

// NewApprovalScheduler creates a new scheduler that will scan for expired
// approvals every checkInterval. The scheduler does not start until Start()
// is called. The store must implement ApprovalTimeoutStore.
func NewApprovalScheduler(store storage.ApprovalTimeoutStore, checkInterval time.Duration, logger *slog.Logger) *ApprovalScheduler {
	if checkInterval <= 0 {
		checkInterval = 10 * time.Second // Default to 10 seconds
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ApprovalScheduler{
		store:         store,
		checkInterval: checkInterval,
		stopCh:        make(chan struct{}),
		logger:        logger,
	}
}

// SetEventHandler registers a callback that will be invoked each time the
// scheduler sweeps expired approvals. This allows the service layer to
// react to expirations (e.g., cancel runs, emit events).
func (s *ApprovalScheduler) SetEventHandler(handler func(ApprovalSweepEvent)) {
	s.eventHandler = handler
}

// Start begins the background scanning loop. It returns immediately and
// runs in a goroutine. Call Stop() to shut it down cleanly.
func (s *ApprovalScheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.run(ctx)
}

// Stop signals the scheduler to stop and waits for the current scan to
// complete. It is safe to call multiple times.
func (s *ApprovalScheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *ApprovalScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	s.logger.Info("approval scheduler started", "interval", s.checkInterval)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("approval scheduler stopped", "reason", "context done")
			return
		case <-s.stopCh:
			s.logger.Info("approval scheduler stopped", "reason", "stop signal")
			return
		case <-ticker.C:
			s.sweep(ctx)
		}
	}
}

func (s *ApprovalScheduler) sweep(ctx context.Context) {
	started := time.Now()

	count, err := s.store.SweepExpiredApprovals(ctx)
	if err != nil {
		s.logger.Warn("approval sweep failed", "err", err)
		return
	}

	if count > 0 {
		duration := time.Since(started)
		s.logger.Info("approval sweep expired approvals",
			"count", count, "duration_ms", duration.Milliseconds())

		if s.eventHandler != nil {
			s.eventHandler(ApprovalSweepEvent{
				ExpiredCount: count,
				Timestamp:    started,
			})
		}
	}
}

// SweepOnce performs a single sweep operation without starting the
// background loop. This is useful for testing or manual triggering.
func (s *ApprovalScheduler) SweepOnce(ctx context.Context) (int, error) {
	return s.store.SweepExpiredApprovals(ctx)
}
