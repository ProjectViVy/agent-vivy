package runtime

import (
	"context"
	"fmt"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
)

// finalizeCognitiveSessions runs outside projectionMu: canceled producers and
// the existing ObserverHost must persist terminal delivery before messages
// disappear. Failure keeps the Session sealed and its source rows intact.
func (s *Service) finalizeCognitiveSessions(ctx context.Context, sessions []domain.SessionID, runs []domain.Run) error {
	if s.deps.Cognitive == nil {
		return nil
	}
	finalizer, ok := s.deps.Cognitive.Sink.(cognitivecontract.SessionFinalizer)
	if !ok {
		return nil
	}
	for _, run := range runs {
		if run.Kind != domain.RunKindPrimary || run.SessionID == cognitiveSupervisorSessionID {
			continue
		}
		for {
			current, err := s.deps.Runs.GetRun(ctx, run.ID)
			if err != nil {
				return err
			}
			if current.Status.Terminal() {
				break
			}
			if err := waitCaptureBoundary(ctx); err != nil {
				return err
			}
		}
		// The same host owns recovery, worker delivery and cursor persistence.
		// Do not call the capture sink directly or mint a substitute event.
		found := false
		for _, hook := range s.deps.Hooks {
			drain, ok := hook.(interface {
				DeliverRun(context.Context, domain.RunID) error
			})
			if !ok {
				continue
			}
			found = true
			if err := drain.DeliverRun(ctx, run.ID); err != nil {
				return fmt.Errorf("session terminal delivery: %w", err)
			}
		}
		if !found {
			return fmt.Errorf("session terminal delivery host unavailable")
		}
	}
	for _, session := range sessions {
		if err := finalizer.FinalizeSession(ctx, string(session)); err != nil {
			return fmt.Errorf("session activity archive: %w", err)
		}
	}
	return nil
}

func waitCaptureBoundary(ctx context.Context) error {
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
