package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"github.com/cloudwego/eino/compose"
)

// nudgeTemplateVersion is the payload contract version of tool.nudge
// (NUDGE-DESIGN §7).
const nudgeTemplateVersion = "nudge-v1"

// Producer projections use the same 1 MiB ceiling as private tool inputs.
// Oversized advisory values fail closed rather than truncate tuple identity.
const nudgeProducerMaxBytes = 1 << 20

// completedCall is the durable outcome record for one dispatched tool
// call. Values come from the existing effect owner or an explicitly typed,
// bounded producer projection, never from derived event acknowledgment.
type completedCall struct {
	ID       string
	Name     string
	ArgsJSON string
	Result   string
	Error    string
	Failure  *toolFailure
}

// nudgeNotice is the reminder handed to the model boundary at most once
// per settled batch (§4/§7). It carries only the bounded identifiers the
// audit event and the fixed template need — never arguments or
// diagnostic text.
type nudgeNotice struct {
	CallID   string
	ToolName string
	Reason   string
	// Status selects the fixed template variant: refusals get the
	// respect-the-decision text, recoverable failures the corrective one.
	Status          string
	Count           int
	TemplateVersion string
}

// producedCall is a leg-local coverage projection. Operation-backed calls keep
// only a reference; the immutable database row supplies their values at the
// native model boundary. Work/refusal calls retain their bounded producer value.
type producedCall struct {
	call      completedCall
	operation bool
	fresh     bool
}

// nudgeState owns one fresh detector and native-batch retry memo per leg.
// There is no event-consumer acknowledgement, completion queue or waiter.
type nudgeState struct {
	mu           sync.Mutex
	marks        map[string]toolFailure
	produced     map[string]producedCall
	window       loopWindow
	evaluatedIDs []string
	notice       *nudgeNotice
	noticeTaken  bool
	terminal     error
}

func newNudgeState() *nudgeState {
	return &nudgeState{marks: map[string]toolFailure{}, produced: map[string]producedCall{}}
}

func (s *nudgeState) terminalErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.terminal
}

func (s *nudgeState) MarkFailure(id string, failure toolFailure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		return errors.New("runtime: nudge failure mark requires a tool call id")
	}
	if s.terminal != nil {
		return s.terminal
	}
	s.marks[id] = failure
	return nil
}

func (s *nudgeState) Failure(id string) (toolFailure, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.marks[id]
	return f, ok
}

// A new native tool invocation is distinct from a provider retry of the same
// prepared model request, even if a model reuses its preceding call ID. The
// existing effect owner still validates/replays that invocation's identity.
func beginNudgeInvocation(ctx context.Context) {
	s := nudgeStateFromContext(ctx)
	if s == nil {
		return
	}
	id := compose.GetToolCallID(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if slices.Contains(s.evaluatedIDs, id) {
		for _, prior := range s.evaluatedIDs {
			delete(s.produced, prior)
		}
		s.evaluatedIDs = nil
		s.notice = nil
		s.noticeTaken = false
	}
}

func (s *nudgeState) record(call completedCall, operation, fresh bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if call.ID == "" {
		return errors.New("runtime: nudge producer requires a tool call id")
	}
	if old, ok := s.produced[call.ID]; ok {
		if old.call.Name != call.Name || old.operation != operation {
			return errors.New("runtime: conflicting nudge producer identity")
		}
		if !operation && !sameNudgeCall(old.call, call) {
			return errors.New("runtime: conflicting nudge producer outcome")
		}
		// A second waiter for the same committed operation cannot turn a fresh
		// owner completion into a replay or manufacture a second observation.
		fresh = fresh || old.fresh
	}
	if call.Failure != nil {
		f := *call.Failure
		call.Failure = &f
	}
	s.produced[call.ID] = producedCall{call: call, operation: operation, fresh: fresh}
	return nil
}

func sameNudgeCall(a, b completedCall) bool {
	if a.ID != b.ID || a.Name != b.Name || a.ArgsJSON != b.ArgsJSON || a.Result != b.Result || a.Error != b.Error {
		return false
	}
	if a.Failure == nil || b.Failure == nil {
		return a.Failure == b.Failure
	}
	return *a.Failure == *b.Failure
}

func recordNudgeOperation(ctx context.Context, op domain.ToolOperation, fresh bool) error {
	s := nudgeStateFromContext(ctx)
	if s == nil {
		return nil
	}
	if err := storage.ValidateToolOperationSource(op); err != nil {
		return err
	}
	if op.State != domain.ToolOperationCompleted {
		return errors.New("runtime: nudge operation is not completed")
	}
	if err := storage.ValidateToolOperationOutcome(op.Outcome()); err != nil {
		return err
	}
	return s.record(completedCall{ID: op.OperationID, Name: op.ToolName}, true, fresh)
}

func recordNudgeProjection(ctx context.Context, toolName, arguments string, outcome domain.ToolOperationOutcome) error {
	s := nudgeStateFromContext(ctx)
	if s == nil {
		return nil
	}
	id := compose.GetToolCallID(ctx)
	s.mu.Lock()
	prior, exists := s.produced[id]
	s.mu.Unlock()
	if exists && prior.operation {
		if prior.call.Name != toolName {
			return errors.New("runtime: conflicting nudge producer identity")
		}
		return nil
	}
	if len(arguments) > nudgeProducerMaxBytes || len(outcome.Result) > nudgeProducerMaxBytes || len(outcome.Failure) > nudgeProducerMaxBytes {
		return fmt.Errorf("%w: nudge producer value exceeds %d bytes", ErrContextBudgetExceeded, nudgeProducerMaxBytes)
	}
	canonical, err := canonicalToolArguments([]byte(arguments))
	if err != nil {
		// Invalid JSON is a typed refusal too; retain its exact bounded identity.
		canonical = []byte(arguments)
	}
	if len(canonical) > nudgeProducerMaxBytes {
		return fmt.Errorf("%w: canonical nudge arguments exceed %d bytes", ErrContextBudgetExceeded, nudgeProducerMaxBytes)
	}
	call := completedCall{ID: id, Name: toolName, ArgsJSON: string(canonical), Result: outcome.Result, Error: outcome.Failure}
	if f, ok := failureFromOutcome(outcome); ok {
		call.Failure = &f
	}
	return s.record(call, false, true)
}

// prepare validates complete native result coverage before advancing the
// detector. Database-backed values come from the existing operation owner.
// The full ordered batch is memoized even when every sibling is non-fresh.
func (s *nudgeState) prepare(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if slices.Equal(ids, s.evaluatedIDs) {
		return nil
	}
	seen := map[string]bool{}
	calls := make([]completedCall, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			return errors.New("runtime: invalid native nudge batch identity")
		}
		seen[id] = true
		produced, ok := s.produced[id]
		if !ok {
			return fmt.Errorf("runtime: native tool result lacks producer outcome: %s", id)
		}
		call := produced.call
		if produced.operation {
			coordinator, ok := toolOperationCoordinatorFromContext(ctx).(serviceToolOperationCoordinator)
			if !ok {
				return ErrToolOperationUnavailable
			}
			var err error
			call, err = coordinator.nudgeSample(ctx, id, call.Name)
			if err != nil {
				return err
			}
		}
		if produced.fresh {
			calls = append(calls, call)
		}
	}
	var best *nudgeNotice
	for _, call := range calls {
		count, err := s.window.record(call.Name, call.ArgsJSON, call.Result, call.Error)
		if err != nil {
			s.terminal = err
			return err
		}
		if call.Failure == nil || (count != 3 && count != 5) {
			continue
		}
		if best == nil || count > best.Count {
			best = &nudgeNotice{CallID: call.ID, ToolName: call.Name, Reason: call.Failure.Reason, Status: call.Failure.Status, Count: count, TemplateVersion: nudgeTemplateVersion}
		}
	}
	for _, id := range s.evaluatedIDs {
		delete(s.produced, id)
	}
	s.evaluatedIDs = append(s.evaluatedIDs[:0], ids...)
	s.notice, s.noticeTaken = best, false
	return nil
}

func (s *nudgeState) Take(ctx context.Context, ids []string) (*nudgeNotice, bool, error) {
	if len(ids) == 0 {
		return nil, false, nil
	}
	if err := s.prepare(ctx, ids); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	first := s.notice != nil && !s.noticeTaken
	if first {
		s.noticeTaken = true
	}
	return s.notice, first, nil
}

func (s *nudgeState) Abort(err error) {
	if err == nil {
		err = context.Canceled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal == nil {
		s.terminal = err
	}
	s.produced = nil
	s.marks = nil
}

type nudgeStateContextKey struct{}

func withNudgeState(ctx context.Context, state *nudgeState) context.Context {
	return context.WithValue(ctx, nudgeStateContextKey{}, state)
}
func nudgeStateFromContext(ctx context.Context) *nudgeState {
	state, _ := ctx.Value(nudgeStateContextKey{}).(*nudgeState)
	return state
}
