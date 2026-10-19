package runtime

import (
	"context"
	"fmt"

	"agent-vivy/internal/domain"
)

type recapSessionControlKey struct{}

// beginRecapSessionControl uses the existing startup/catalog fences and Service
// cancellation/drain semantics, without fabricating a conversation Run. A mutex-
// captured idle channel avoids introducing zero-count wg.Add/Wait races.
func (s *Service) beginRecapSessionControl(ctx context.Context, sid domain.SessionID) (context.Context, func(), error) {
	releaseAdmission, err := s.reserveBundleAdmission()
	if err != nil {
		return ctx, nil, err
	}
	defer releaseAdmission()
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	s.mu.Lock()
	stopped := s.stopping
	busy := len(s.active) > 0 || len(s.pending) > 0 || len(s.shellPending) > 0
	s.mu.Unlock()
	if stopped {
		return ctx, nil, fmt.Errorf("%w: service stopping", ErrCompactionBusy)
	}
	if busy {
		return ctx, nil, ErrCompactionBusy
	}
	if err = ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if err = s.applyPendingEngineReload(ctx, nil); err != nil {
		return ctx, nil, err
	}
	controlCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		cancel()
		return ctx, nil, fmt.Errorf("%w: service stopping", ErrCompactionBusy)
	}
	eng, provider, id := s.engine, s.provider, s.modelID
	if eng == nil {
		s.mu.Unlock()
		cancel()
		return ctx, nil, ErrRecapUnavailable
	}
	control := domain.RunID(newPrefixedID("sessctl_recap_" + sha256Hex([]byte(sid))[:16] + "_"))
	if s.sessionControls == nil {
		s.sessionControls = map[domain.RunID]context.CancelFunc{}
	}
	if len(s.sessionControls) == 0 {
		s.sessionControlsIdle = make(chan struct{})
	}
	s.sessionControls[control] = cancel
	s.mu.Unlock()
	finish := func() {
		cancel()
		s.mu.Lock()
		delete(s.sessionControls, control)
		if len(s.sessionControls) == 0 {
			close(s.sessionControlsIdle)
		}
		s.mu.Unlock()
	}
	controlCtx = withRunExecution(controlCtx, eng, executionFromContext(ctx))
	controlCtx = domain.WithModelRequestScope(controlCtx, provider, id)
	controlCtx = context.WithValue(controlCtx, recapSessionControlKey{}, control)
	// This creates only an observer/ledger. It performs no capture or model work
	// while projectionMu is held, and all CAS retries share this one call budget.
	controlCtx, err = s.withRecapSummaryObserver(controlCtx, &recapFold{Reader: sid})
	if err != nil {
		finish()
		return ctx, nil, err
	}
	return controlCtx, finish, nil
}
