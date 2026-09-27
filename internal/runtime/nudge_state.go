package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// nudgeTemplateVersion is the payload contract version of tool.nudge
// (NUDGE-DESIGN §7).
const nudgeTemplateVersion = "nudge-v1"

// completedCall is the durable outcome record for one dispatched tool
// call. Outcome values are already redacted and bounded by the mapper;
// the state copies them by value and never retains raw arguments beyond
// the outstanding batch.
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

// nudgeBatch tracks one model turn's tool calls in request order.
type nudgeBatch struct {
	ids       []string
	remaining map[string]struct{}
	calls     map[string]completedCall
	satisfied map[string]struct{}
	implicit  bool
}

// nudgeState is the single run-local observation point for bounded tool
// outcomes (ND-2): it owns the loop detector, correlates typed failure
// marks, seals a batch only after all of its tool.finished events have
// durably persisted, and yields at most one reminder to the model
// boundary per settled batch. It is created per drive/resume leg, shared
// by pointer through context to the mapper/adapter/wrapper seam, and
// performs no Journal or model work itself.
//
// Journal order guarantees Register precedes every Complete of the same
// batch (tool.requested is always journaled before tool.finished), so a
// completion for an unregistered id only legitimately occurs on resume
// legs — where it is admitted as an implicit singleton batch.
type nudgeState struct {
	mu      sync.Mutex
	changed chan struct{}

	batch  *nudgeBatch
	marks  map[string]toolFailure
	window loopWindow

	notice      *nudgeNotice
	noticeTaken bool
	sealedIDs   map[string]struct{}

	terminal error
}

func newNudgeState() *nudgeState {
	return &nudgeState{changed: make(chan struct{}), marks: map[string]toolFailure{}}
}

func (s *nudgeState) broadcastLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

func (s *nudgeState) setTerminalLocked(cause error) {
	if s.terminal == nil {
		s.terminal = cause
	}
	s.broadcastLocked()
}

// terminalErr reports the stop cause (loop detection, seal/abort error)
// once set; the consuming Service reads it after Seal to emit the run's
// terminal event.
func (s *nudgeState) terminalErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.terminal
}

// Register opens the outstanding batch for one model turn's tool calls,
// in request order. Empty ids, duplicate ids, and a register that
// overlaps an unsealed batch are invariant errors; the caller aborts the
// run.
func (s *nudgeState) Register(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if len(ids) == 0 {
		return errors.New("runtime: nudge register requires at least one tool call id")
	}
	if s.batch != nil {
		return fmt.Errorf("runtime: nudge register overlaps an unsealed batch of %d tool calls", len(s.batch.ids))
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			return errors.New("runtime: nudge register requires non-empty tool call ids")
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("runtime: nudge register duplicates tool call id %q", id)
		}
		seen[id] = struct{}{}
	}
	batch := &nudgeBatch{
		ids: append([]string(nil), ids...), remaining: make(map[string]struct{}, len(ids)),
		calls: make(map[string]completedCall, len(ids)), satisfied: make(map[string]struct{}, len(ids)),
	}
	for _, id := range ids {
		batch.remaining[id] = struct{}{}
	}
	s.batch = batch
	return nil
}

// MarkFailure records the typed failure mark for one call id on the
// side channel between the adapter and the mapper. The mark is keyed by
// id only: dispatch may run before the consuming Service journals the
// matching tool.requested, so registration is not required here. A
// missing id is an invariant failure (NUDGE-DESIGN §5), never a
// name-based fallback.
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

// Failure reads the typed failure mark for one call id.
func (s *nudgeState) Failure(id string) (toolFailure, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.marks[id]
	return f, ok
}

// Complete records one durable outcome. With an outstanding batch the id
// must belong to it — a duplicate or foreign id is an invariant error.
// With no outstanding batch the completion is admitted into an implicit
// batch, which is how tool results replayed by a resume leg are grouped
// before the model boundary reveals the complete result tail.
func (s *nudgeState) Complete(call completedCall) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if call.ID == "" {
		return errors.New("runtime: nudge completion requires a tool call id")
	}
	batch := s.batch
	if batch == nil {
		batch = &nudgeBatch{
			ids:       []string{call.ID},
			remaining: map[string]struct{}{call.ID: {}},
			calls:     map[string]completedCall{},
			satisfied: map[string]struct{}{},
			implicit:  true,
		}
		s.batch = batch
	} else if batch.implicit && len(batch.remaining) == 0 {
		if _, exists := batch.calls[call.ID]; exists {
			return fmt.Errorf("runtime: nudge completion duplicates tool call id %q", call.ID)
		}
		batch.ids = append(batch.ids, call.ID)
		batch.remaining[call.ID] = struct{}{}
	}
	if _, ok := batch.remaining[call.ID]; !ok {
		_, done := batch.calls[call.ID]
		_, settled := batch.satisfied[call.ID]
		if done || settled {
			return fmt.Errorf("runtime: nudge completion duplicates tool call id %q", call.ID)
		}
		return fmt.Errorf("runtime: nudge completion for unregistered tool call id %q", call.ID)
	}
	delete(batch.remaining, call.ID)
	if call.Failure != nil {
		failure := *call.Failure
		call.Failure = &failure
	}
	batch.calls[call.ID] = call
	s.broadcastLocked()
	return nil
}

// SatisfyDurable marks a sibling result already persisted by an earlier leg
// of the same interrupted model batch. Such results satisfy the handoff
// barrier, but are intentionally excluded from this leg's fresh detector.
func (s *nudgeState) SatisfyDurable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if id == "" || s.batch == nil || s.batch.implicit {
		return errors.New("runtime: durable nudge result requires a registered resume batch")
	}
	if _, ok := s.batch.remaining[id]; !ok {
		return fmt.Errorf("runtime: durable nudge result has unexpected tool call id %q", id)
	}
	delete(s.batch.remaining, id)
	s.batch.satisfied[id] = struct{}{}
	s.broadcastLocked()
	return nil
}

// Seal closes the outstanding batch after the consuming Service has
// persisted every tool.finished of it: completed outcomes flush into the
// loop window in request order, the highest-threshold failure notice is
// prepared (ties: earliest request position), and the waiting model
// boundary is released. A non-nil err abandons the batch and releases
// waiters with that cause; detection of the repetition limit sets the
// same terminal path. Calling Seal before the batch is fully persisted
// is an invariant error.
func (s *nudgeState) Seal(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealLocked(err)
}

func (s *nudgeState) sealLocked(err error) {
	if err != nil {
		s.setTerminalLocked(err)
		return
	}
	batch := s.batch
	if batch == nil {
		return
	}
	if len(batch.remaining) != 0 {
		s.setTerminalLocked(fmt.Errorf("runtime: nudge seal before all %d tool results persisted", len(batch.remaining)))
		return
	}
	var best *nudgeNotice
	for _, id := range batch.ids {
		if _, settled := batch.satisfied[id]; settled {
			continue
		}
		call := batch.calls[id]
		count, recErr := s.window.record(call.Name, call.ArgsJSON, call.Result, call.Error)
		if errors.Is(recErr, errLoopDetected) {
			s.setTerminalLocked(errLoopDetected)
			return
		}
		// Successful calls feed the hard stop but never produce a
		// reminder (§6): only marked failures are notice candidates.
		if call.Failure == nil || (count != 3 && count != 5) {
			continue
		}
		if best == nil || count > best.Count {
			best = &nudgeNotice{
				CallID:          call.ID,
				ToolName:        call.Name,
				Reason:          call.Failure.Reason,
				Status:          call.Failure.Status,
				Count:           count,
				TemplateVersion: nudgeTemplateVersion,
			}
		}
	}
	s.notice = best
	s.noticeTaken = false
	s.sealedIDs = make(map[string]struct{}, len(batch.ids))
	for _, id := range batch.ids {
		s.sealedIDs[id] = struct{}{}
		delete(s.marks, id)
	}
	s.batch = nil
	s.broadcastLocked()
}

// Take is the model boundary's only blocking API: it waits until a
// batch covering ids — the tool results at the input tail — has been
// registered and sealed (or the context is cancelled), returns the
// terminal error when one is set, and yields the sealed batch's
// prepared notice. Keying on ids matters: the engine can reach the
// next model call before the consumer registers the batch, so
// batch==nil alone cannot distinguish "not yet durable" from
// "sealed without a reminder". The same notice is returned on every
// Take while the batch stays current — a provider retry must observe
// the identical request — but first=true exactly once, which is the
// single scheduling signal for the journal emitter.
func (s *nudgeState) Take(ctx context.Context, ids []string) (*nudgeNotice, bool, error) {
	if len(ids) == 0 {
		return nil, false, nil
	}
	for {
		s.mu.Lock()
		if s.terminal != nil {
			err := s.terminal
			s.mu.Unlock()
			return nil, false, err
		}
		if batch := s.batch; batch != nil && len(batch.remaining) == 0 && len(batch.ids) == len(ids) {
			covered := true
			for _, id := range ids {
				_, completed := batch.calls[id]
				_, settled := batch.satisfied[id]
				if !completed && !settled {
					covered = false
					break
				}
			}
			if covered {
				batch.ids = append(batch.ids[:0], ids...)
				s.sealLocked(nil)
			}
		}
		if s.batch == nil && s.sealedIDs != nil {
			covered := true
			for _, id := range ids {
				if _, ok := s.sealedIDs[id]; !ok {
					covered = false
					break
				}
			}
			if covered {
				notice := s.notice
				first := false
				if notice != nil && !s.noticeTaken {
					s.noticeTaken = true
					first = true
				}
				s.mu.Unlock()
				return notice, first, nil
			}
		}
		wait := s.changed
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-wait:
		}
	}
}

// Abort releases every waiting boundary with the given cause and
// prevents any further model handoff. It is idempotent: the first cause
// wins.
func (s *nudgeState) Abort(err error) {
	if err == nil {
		err = context.Canceled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setTerminalLocked(err)
}

type nudgeStateContextKey struct{}

// withNudgeState attaches the leg's detector so the adapter, wrapper and
// other in-context seams share exactly this pointer (§6).
func withNudgeState(ctx context.Context, state *nudgeState) context.Context {
	return context.WithValue(ctx, nudgeStateContextKey{}, state)
}

// nudgeStateFromContext returns the leg's detector, or nil outside
// drive/resume legs.
func nudgeStateFromContext(ctx context.Context) *nudgeState {
	state, _ := ctx.Value(nudgeStateContextKey{}).(*nudgeState)
	return state
}
