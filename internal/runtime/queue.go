package runtime

// Dual-track message queue (VCP-B1, pi parity): the steer lane injects a
// user message at the next turn boundary of the active run (boundary-safe
// cancel + checkpoint resume with a history modifier — mechanism M2, see
// the Eino capability check in the story log); the follow-up lane admits
// after the run settles.
//
// Durability: turn.queued/turn.dequeued/turn.steered markers on the run
// journal. The invariant that keeps recovery O(one replay): the pending
// queue is always fully materialized on the NEWEST run's journal — a new
// admission carries the remaining tail atomically in run.started. Idle
// removal tombstones use a synthetic Session control stream after sealing.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// ErrQueueUnavailable marks steer attempts that cannot reach a live run
// AND cannot degrade (e.g. boundary cancel unavailable); callers fall back
// to the follow-up lane or a fresh run.
var ErrQueueUnavailable = errors.New("runtime: no active run for session")

type sessionQueue struct {
	rebuildMu sync.Mutex
	err       error
	// admittedBy maps a settling run id to the follow-up run the kernel
	// auto-started for it. Faces resolve it via queue/state{after_run_id}
	// after the terminal — a post-terminal wire hint cannot be journaled
	// and the bus closes on terminal, so discovery goes through state.
	admittedBy map[string]string
	// lastAdmitted is the most recently auto-started run id, exposed on
	// session/get for face-free consumers.
	lastAdmitted string
	inFlight     []domain.QueuedTurn
	steer        []domain.QueuedTurn
	followUp     []domain.QueuedTurn
	// pi lane modes: "all" drains at the boundary/settle, "one-at-a-time"
	// serves the head only and keeps the tail queued.
	steerMode    string
	followUpMode string
	rebuilt      bool
}

func newSessionQueue() *sessionQueue {
	return &sessionQueue{
		steerMode:    domain.QueueModeAll,
		followUpMode: domain.QueueModeAll,
	}
}

// QueueState reports the session's queue snapshot (RPC queue/clear, TUI
// dequeue, get_state pending count).
type QueueState struct {
	Err               error `json:"-"`
	LastAdmittedRunID string
	// AdmittedRunID resolves queue/state{after_run_id}: the run the kernel
	// auto-started when that specific run settled ("" = none).
	AdmittedRunID string
	Steering      []domain.QueuedTurn `json:"steering"`
	FollowUps     []domain.QueuedTurn `json:"follow_up"`
	SteerMode     string              `json:"steer_mode"`
	FollowUpMode  string              `json:"follow_up_mode"`
}

// queueFor returns (and lazily rebuilds) the session's queue.
func (s *Service) queueFor(ctx context.Context, sessionID domain.SessionID) *sessionQueue {
	s.mu.Lock()
	q := s.queues[sessionID]
	if q == nil {
		q = newSessionQueue()
		s.queues[sessionID] = q
	}
	s.mu.Unlock()
	q.rebuildMu.Lock()
	defer q.rebuildMu.Unlock()
	if !q.rebuilt {
		err := s.rebuildQueue(ctx, sessionID, q)
		s.mu.Lock()
		q.err = err
		q.rebuilt = err == nil
		s.mu.Unlock()
	}
	return q
}
func (s *Service) rebuildQueue(ctx context.Context, sessionID domain.SessionID, q *sessionQueue) error {
	runs, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("runtime: list queue carrier: %w", err)
	}
	if len(runs) == 0 {
		return nil
	}
	var runID domain.RunID
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].Kind == "" || runs[i].Kind == domain.RunKindPrimary {
			runID = runs[i].ID
			break
		}
	}
	if runID == "" {
		return nil
	}
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return fmt.Errorf("runtime: replay queue: %w", err)
	}
	defer it.Close()
	var inFlight []domain.QueuedTurn
	steerMode, followMode := q.steerMode, q.followUpMode
	var pending []domain.QueuedTurn
	for it.Next() {
		ev := it.Value().Event
		switch ev.Type {
		case domain.EventRunStarted:
			var p payloadRunStarted
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				return err
			}
			if p.Queue != nil {
				pending = append([]domain.QueuedTurn(nil), p.Queue.Pending...)
				steerMode, followMode = p.Queue.SteerMode, p.Queue.FollowUpMode
			}
		case domain.EventTurnQueued:
			var p payloadTurnQueued
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				return err
			}
			if p.QueueID == "" || (p.Track != domain.QueueTrackSteer && p.Track != domain.QueueTrackFollowUp) {
				return errors.New("runtime: invalid queued turn")
			}
			pending = removeQueued(pending, p.QueueID)
			pending = append(pending, domain.QueuedTurn{ID: p.QueueID, SessionID: sessionID, Track: p.Track, Text: p.Text, RequestRef: p.RequestRef, CreatedAt: time.UnixMilli(ev.CreatedAt), InterruptCurrent: p.InterruptCurrent})
		case domain.EventTurnDequeued, domain.EventTurnSteered:
			var p struct {
				QueueID string `json:"queue_id"`
			}
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				return err
			}
			if p.QueueID == "" {
				return errors.New("runtime: empty queue removal id")
			}
			if ev.Type == domain.EventTurnSteered {
				for _, item := range pending {
					if item.ID == p.QueueID {
						inFlight = append(inFlight, item)
					}
				}
			}
			pending = removeQueued(pending, p.QueueID)
		case domain.EventRunCompleted:
			inFlight = nil
		case domain.EventRunFailed:
			for i := range inFlight {
				inFlight[i].Track = domain.QueueTrackFollowUp
			}
			pending = append(inFlight, pending...)
			inFlight = nil
		case domain.EventRunCancelled:
			// A send-now survives the cancellation it triggered: replay must
			// keep it queued so a crash between cancel and admission still
			// delivers the turn. Everything else was flushed by the
			// terminal's aborted turn.dequeued markers.
			kept := make([]domain.QueuedTurn, 0, len(pending))
			for _, item := range pending {
				if item.InterruptCurrent {
					item.Track = domain.QueueTrackFollowUp
					kept = append(kept, item)
				}
			}
			pending, inFlight = kept, nil
		}
	}
	if err := it.Err(); err != nil {
		return fmt.Errorf("runtime: replay queue: %w", err)
	}
	if err := it.Close(); err != nil {
		return fmt.Errorf("runtime: close queue replay: %w", err)
	}
	pending, err = s.applyQueueControls(ctx, sessionID, pending)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, item := range append(append([]domain.QueuedTurn(nil), pending...), inFlight...) {
		if item.ID == "" || seen[item.ID] || item.SessionID != sessionID || (item.Track != domain.QueueTrackSteer && item.Track != domain.QueueTrackFollowUp) {
			return errors.New("runtime: invalid queue snapshot")
		}
		seen[item.ID] = true
	}
	for i := range pending {
		pending[i].EnqueuedOn = runID
		if err := s.loadQueuedRequest(ctx, &pending[i]); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	q.steer, q.followUp = nil, nil
	q.inFlight = inFlight
	q.steerMode, q.followUpMode = steerMode, followMode
	for _, item := range pending {
		if item.Track == domain.QueueTrackSteer {
			q.steer = append(q.steer, item)
		} else {
			q.followUp = append(q.followUp, item)
		}
	}
	return nil
}

// activeRunForSession returns the run currently driving the session, if any.
func (s *Service) activeRunForSession(sessionID domain.SessionID) domain.RunID {
	s.mu.Lock()
	defer s.mu.Unlock()
	for runID, sid := range s.runSessions {
		if sid != sessionID {
			continue
		}
		if _, ok := s.active[runID]; ok {
			return runID
		}
	}
	return ""
}

// Caller holds projectionMu; persistence always precedes acknowledgement.
func (s *Service) journalQueueMarker(ctx context.Context, item domain.QueuedTurn, eventType domain.EventType, payload any) error {
	if item.EnqueuedOn == "" {
		return errors.New("runtime: queue has no journal carrier")
	}
	ev := newEventMapper(item.EnqueuedOn, s.engine.cfg.MaxEventPayloadBytes).build(eventType, payload)
	_, err := s.appendRunEventLocked(ctx, ev, false)
	if err != nil {
		return fmt.Errorf("runtime: persist queue marker: %w", err)
	}
	return nil
}
func (s *Service) Steer(ctx context.Context, sessionID domain.SessionID, text string) (domain.QueuedTurn, error) {
	return s.SteerWithOptions(ctx, sessionID, text, RunOptions{})
}
func (s *Service) SteerWithOptions(ctx context.Context, sessionID domain.SessionID, text string, o RunOptions) (domain.QueuedTurn, error) {
	return s.enqueueTurn(ctx, sessionID, text, o, true, false)
}
func (s *Service) FollowUp(ctx context.Context, sessionID domain.SessionID, text string) (domain.QueuedTurn, error) {
	return s.FollowUpWithOptions(ctx, sessionID, text, RunOptions{})
}
func (s *Service) FollowUpWithOptions(ctx context.Context, sessionID domain.SessionID, text string, o RunOptions) (domain.QueuedTurn, error) {
	return s.enqueueTurn(ctx, sessionID, text, o, false, false)
}

// SendNow interrupts the session's active run and admits the text as the
// next run. The turn is a flagged follow-up: cancellation flushes every
// other queued item, the flagged turn survives the terminal, and the
// settle drains it — one durable admission path, replay-safe.
func (s *Service) SendNow(ctx context.Context, sessionID domain.SessionID, text string) (domain.QueuedTurn, error) {
	return s.SendNowWithOptions(ctx, sessionID, text, RunOptions{})
}
func (s *Service) SendNowWithOptions(ctx context.Context, sessionID domain.SessionID, text string, o RunOptions) (domain.QueuedTurn, error) {
	item, err := s.enqueueTurn(ctx, sessionID, text, o, false, true)
	if err != nil {
		return domain.QueuedTurn{}, err
	}
	// The admission fence is released: resolve the run driving the session
	// now instead of cancelling an id read under the fence.
	runID := s.activeRunForSession(sessionID)
	if runID == "" {
		return item, nil
	}
	if _, err := s.CancelRun(context.WithoutCancel(ctx), runID); err != nil {
		slog.Warn("send now could not cancel the active run", "session", string(sessionID), "run", string(runID), "err", err)
	}
	return item, nil
}
func (s *Service) enqueueTurn(ctx context.Context, sessionID domain.SessionID, text string, o RunOptions, steer, interruptCurrent bool) (domain.QueuedTurn, error) {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	runID := s.activeRunForSession(sessionID)
	if runID == "" {
		return domain.QueuedTurn{}, ErrQueueUnavailable
	}
	q := s.queueFor(ctx, sessionID)
	if err := s.queueError(q); err != nil {
		return domain.QueuedTurn{}, err
	}
	request, err := queueRequest(ctx, o)
	if err != nil {
		return domain.QueuedTurn{}, err
	}
	item := domain.QueuedTurn{ID: newQueueID(), SessionID: sessionID, Track: domain.QueueTrackFollowUp, Text: text, CreatedAt: time.Now(), EnqueuedOn: runID, Request: request, InterruptCurrent: interruptCurrent}
	s.mu.Lock()
	_, suspended := s.pending[runID]
	cancelFn, cancellable := s.steerCancels[runID]
	s.mu.Unlock()
	if steer && !interruptCurrent && request == nil && !suspended && cancellable {
		item.Track = domain.QueueTrackSteer
	}
	if request != nil {
		if s.deps.QueueRequests == nil {
			return domain.QueuedTurn{}, errors.New("runtime: queued request store not wired")
		}
		item.RequestRef = "queue/request/" + item.ID
		data, err := json.Marshal(request)
		if err != nil {
			return domain.QueuedTurn{}, err
		}
		if err := s.deps.QueueRequests.Put(ctx, item.RequestRef, data); err != nil {
			return domain.QueuedTurn{}, fmt.Errorf("runtime: save queued request: %w", err)
		}
	}
	ev := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes).build(domain.EventTurnQueued, payloadTurnQueued{QueueID: item.ID, Track: item.Track, Text: item.Text, RequestRef: item.RequestRef, InterruptCurrent: item.InterruptCurrent})
	item.CreatedAt = time.UnixMilli(ev.CreatedAt)
	if _, err := s.appendRunEventLocked(ctx, ev, false); err != nil {
		if item.RequestRef != "" {
			if e := s.deps.QueueRequests.Delete(context.WithoutCancel(ctx), item.RequestRef); e != nil {
				slog.Warn("queue snapshot cleanup failed", "queue_id", item.ID, "err", e)
			}
		}
		return domain.QueuedTurn{}, fmt.Errorf("runtime: persist queue marker: %w", err)
	}
	s.mu.Lock()
	if item.InterruptCurrent {
		// The send-now is the first turn admitted after the cancellation
		// that follows this enqueue.
		q.followUp = append([]domain.QueuedTurn{item}, q.followUp...)
	} else if item.Track == domain.QueueTrackSteer {
		q.steer = append(q.steer, item)
		s.steerArmed[runID] = true
	} else {
		q.followUp = append(q.followUp, item)
	}
	s.mu.Unlock()
	if item.Track == domain.QueueTrackSteer {
		cancelFn(adk.WithAgentCancelMode(adk.CancelAfterChatModel))
	}
	return item, nil
}

// ClearQueue drains both lanes and returns the texts (pi returns them for
// editor restore). Consumption is journaled per item.
func (s *Service) ClearQueue(ctx context.Context, sessionID domain.SessionID) (QueueState, error) {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	q := s.queueFor(ctx, sessionID)
	if err := s.queueError(q); err != nil {
		return QueueState{}, err
	}
	s.mu.Lock()
	state := QueueState{Steering: append([]domain.QueuedTurn(nil), q.steer...), FollowUps: append([]domain.QueuedTurn(nil), q.followUp...), SteerMode: q.steerMode, FollowUpMode: q.followUpMode}
	s.mu.Unlock()
	items := append(append([]domain.QueuedTurn(nil), state.Steering...), state.FollowUps...)
	if err := s.persistQueueRemoval(ctx, items, "cleared"); err != nil {
		return QueueState{}, err
	}
	s.mu.Lock()
	q.steer, q.followUp = nil, nil
	s.mu.Unlock()
	s.deleteQueuedRequests(ctx, items)
	return state, nil
}
func (s *Service) Dequeue(ctx context.Context, sessionID domain.SessionID) (domain.QueuedTurn, bool, error) {
	return s.removeQueueTurn(ctx, sessionID, "", true)
}
func (s *Service) QueueRemove(ctx context.Context, sessionID domain.SessionID, queueID string) (domain.QueuedTurn, bool, error) {
	return s.removeQueueTurn(ctx, sessionID, queueID, false)
}
func (s *Service) removeQueueTurn(ctx context.Context, sessionID domain.SessionID, id string, newest bool) (domain.QueuedTurn, bool, error) {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	q := s.queueFor(ctx, sessionID)
	if err := s.queueError(q); err != nil {
		return domain.QueuedTurn{}, false, err
	}
	s.mu.Lock()
	var item domain.QueuedTurn
	if newest && len(q.followUp) > 0 {
		item = q.followUp[len(q.followUp)-1]
	} else if !newest {
		for _, candidate := range append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...) {
			if candidate.ID == id {
				item = candidate
				break
			}
		}
	}
	s.mu.Unlock()
	if item.ID == "" {
		return item, false, nil
	}
	if err := s.persistQueueRemoval(ctx, []domain.QueuedTurn{item}, "dequeued"); err != nil {
		return domain.QueuedTurn{}, false, err
	}
	s.mu.Lock()
	q.steer = removeQueued(q.steer, item.ID)
	q.followUp = removeQueued(q.followUp, item.ID)
	s.mu.Unlock()
	s.deleteQueuedRequests(ctx, []domain.QueuedTurn{item})
	return item, true, nil
}
func (s *Service) persistQueueRemoval(ctx context.Context, items []domain.QueuedTurn, reason string) error {
	if len(items) == 0 {
		return nil
	}
	runID := items[0].EnqueuedOn
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	events := make([]domain.RunEvent, 0, len(items))
	for _, item := range items {
		if item.EnqueuedOn != runID {
			return errors.New("runtime: inconsistent queue carrier")
		}
		events = append(events, m.build(domain.EventTurnDequeued, payloadTurnDequeued{QueueID: item.ID, Track: item.Track, Reason: reason, Text: item.Text}))
	}
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: runID, Events: events})
	if errors.Is(err, storage.ErrRunClosed) {
		runID = queueControlRunID(items[0].SessionID)
		for i := range events {
			events[i].RunID = runID
		}
		seq, err = s.deps.Journal.Append(ctx, storage.Commit{RunID: runID, Events: events})
	}
	if err != nil {
		return fmt.Errorf("runtime: persist queue removal: %w", err)
	}
	for i := range events {
		events[i].Seq = seq - domain.EventSeq(len(events)-1-i)
		s.publish(ctx, events[i])
	}
	return nil
}

// persistQueueRemovalGrouped flushes items that ride different run journals.
// persistQueueRemoval fails on a mixed carrier, so the caller groups by the
// enqueue carrier instead of dropping the removal.
func (s *Service) persistQueueRemovalGrouped(ctx context.Context, items []domain.QueuedTurn, reason string) error {
	if len(items) == 0 {
		return nil
	}
	groups := make(map[domain.RunID][]domain.QueuedTurn)
	order := make([]domain.RunID, 0, 2)
	for _, item := range items {
		if _, ok := groups[item.EnqueuedOn]; !ok {
			order = append(order, item.EnqueuedOn)
		}
		groups[item.EnqueuedOn] = append(groups[item.EnqueuedOn], item)
	}
	for _, runID := range order {
		if err := s.persistQueueRemoval(ctx, groups[runID], reason); err != nil {
			return err
		}
	}
	return nil
}

// QueueState returns the session's pending queue without mutating it.
// afterRun, when non-empty, resolves the run the kernel auto-started for
// that specific settle (admission races the terminal publish, so callers
// poll briefly).
func (s *Service) QueueState(ctx context.Context, sessionID domain.SessionID, afterRun domain.RunID) QueueState {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	return QueueState{
		Err: q.err, LastAdmittedRunID: q.lastAdmitted,
		AdmittedRunID: q.admittedBy[string(afterRun)],
		Steering:      append([]domain.QueuedTurn(nil), q.steer...),
		FollowUps:     append([]domain.QueuedTurn(nil), q.followUp...),
		SteerMode:     q.steerMode,
		FollowUpMode:  q.followUpMode,
	}
}

// SetQueueMode sets a lane's drain mode ("all" | "one-at-a-time").
func (s *Service) SetQueueMode(ctx context.Context, sessionID domain.SessionID, track, mode string) error {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if mode != domain.QueueModeAll && mode != domain.QueueModeOneAtATime {
		return fmt.Errorf("runtime: invalid queue mode %q", mode)
	}
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	switch track {
	case domain.QueueTrackSteer:
		q.steerMode = mode
	case domain.QueueTrackFollowUp:
		q.followUpMode = mode
	default:
		return fmt.Errorf("runtime: invalid queue track %q", track)
	}
	return nil
}

// steerHeadFor returns the steer items a delivery should inject. Mode "all"
// drains every queued steer into one delivery; "one-at-a-time" serves only
// the head. It does not mutate the lane.
func (s *Service) steerHeadFor(q *sessionQueue) []domain.QueuedTurn {
	if q == nil || len(q.steer) == 0 {
		return nil
	}
	out := append([]domain.QueuedTurn(nil), q.steer...)
	if q.steerMode != domain.QueueModeAll {
		out = out[:1]
	}
	return out
}

// commitSteerDelivery makes an injected steer durable: it journals
// turn.steered, moves exactly those items from the steer lane to in-flight,
// and records the user transcript rows. It runs from the steer middleware
// when the text actually reached a model call, so a cancel or a further
// interrupt before that point leaves the guidance queued.
func (s *Service) commitSteerDelivery(sessionID domain.SessionID, runID domain.RunID, items []domain.QueuedTurn) {
	if len(items) == 0 {
		return
	}
	s.projectionMu.Lock()
	q := s.queueFor(context.Background(), sessionID)
	s.mu.Lock()
	if _, active := s.active[runID]; !active {
		s.mu.Unlock()
		s.projectionMu.Unlock()
		return
	}
	queued := make(map[string]struct{}, len(q.steer))
	for _, item := range q.steer {
		queued[item.ID] = struct{}{}
	}
	delivered := make([]domain.QueuedTurn, 0, len(items))
	for _, item := range items {
		if _, ok := queued[item.ID]; ok {
			delivered = append(delivered, item)
		}
	}
	s.mu.Unlock()
	if len(delivered) == 0 {
		s.projectionMu.Unlock()
		return
	}
	events := make([]domain.RunEvent, 0, len(delivered))
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	for _, item := range delivered {
		events = append(events, m.build(domain.EventTurnSteered, payloadTurnSteered{QueueID: item.ID, Text: item.Text}))
	}
	seq, err := s.deps.Journal.Append(context.Background(), storage.Commit{RunID: runID, Events: events})
	if err != nil {
		slog.Error("steer persistence failed", "run", string(runID), "err", err)
		s.projectionMu.Unlock()
		return
	}
	s.mu.Lock()
	for _, item := range delivered {
		q.steer = removeQueued(q.steer, item.ID)
		q.inFlight = append(q.inFlight, item)
	}
	s.mu.Unlock()
	for i := range events {
		events[i].Seq = seq - domain.EventSeq(len(events)-1-i)
		s.publish(context.Background(), events[i])
	}
	s.projectionMu.Unlock()
	for _, item := range delivered {
		if s.deps.Messages == nil {
			continue
		}
		if err := s.deps.Messages.AppendMessage(context.Background(), domain.Message{ID: newMessageID(), SessionID: sessionID, Role: domain.RoleUser, Content: item.Text, CreatedAt: time.Now().UnixMilli()}); err != nil {
			slog.Error("steer transcript persistence failed", "run", string(runID), "err", err)
		}
	}
}

// disarmSteer drops a pending boundary trigger without touching the lane.
func (s *Service) disarmSteer(runID domain.RunID) {
	s.mu.Lock()
	delete(s.steerArmed, runID)
	s.mu.Unlock()
}

// peekSteerForResume reports the steer items a resume leg should inject. It
// never consumes them: an interrupt landing before the next model call must
// keep the guidance queued for the leg that really reaches the model.
func (s *Service) peekSteerForResume(sessionID domain.SessionID, runID domain.RunID) []domain.QueuedTurn {
	q := s.queueFor(context.Background(), sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, active := s.active[runID]; !active {
		return nil
	}
	return s.steerHeadFor(q)
}

// drainFollowUps starts one complete request per Run. The new Run's atomic
// admission consumes the head and transfers the tail before inference starts.
func (s *Service) drainFollowUps(sessionID domain.SessionID, settlingRun domain.RunID) error {
	ctx := context.Background()
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	var item domain.QueuedTurn
	if len(q.steer) > 0 {
		item = q.steer[0]
	} else if len(q.followUp) > 0 {
		item = q.followUp[0]
	}
	queueErr := q.err
	s.mu.Unlock()
	if queueErr != nil {
		slog.Error("queue replay failed", "session", string(sessionID), "err", queueErr)
		return queueErr
	}
	if item.ID == "" {
		return nil
	}
	options := queuedRunOptions(item)
	options.queuedID = item.ID
	runID, err := s.RunWithOptions(ctx, sessionID, item.Text, options)
	if err != nil {
		slog.Error("queued admission failed", "session", string(sessionID), "queue_id", item.ID, "err", err)
		return err
	}
	s.mu.Lock()
	q.lastAdmitted = string(runID)
	if q.admittedBy == nil {
		q.admittedBy = map[string]string{}
	}
	q.admittedBy[string(settlingRun)] = string(runID)
	s.mu.Unlock()
	return nil
}

// steerPendingCancel consumes the armed flag and reports the steer items the
// resumed leg should deliver. The lane itself is untouched: delivery becomes
// durable only when the middleware injects it into a model call.
func (s *Service) steerPendingCancel(runID domain.RunID, sessionID domain.SessionID) []domain.QueuedTurn {
	s.mu.Lock()
	defer s.mu.Unlock()
	armed := s.steerArmed[runID]
	delete(s.steerArmed, runID)
	if !armed {
		return nil
	}
	return s.steerHeadFor(s.queues[sessionID])
}

// pendingQueueDepth reports pending follow-ups that need admitting — used at
// terminal settle to decide whether to auto-start the next run.
func (s *Service) pendingQueueDepth(ctx context.Context, sessionID domain.SessionID) int {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(q.followUp) + len(q.steer)
}

// dropSessionQueue releases the in-memory queue when the session is deleted.
func (s *Service) dropSessionQueue(sessionID domain.SessionID) {
	s.mu.Lock()
	q := s.queues[sessionID]
	var removed []domain.QueuedTurn
	if q != nil {
		removed = append(append(append(removed, q.steer...), q.followUp...), q.inFlight...)
	}
	delete(s.queues, sessionID)
	s.mu.Unlock()
	s.deleteQueuedRequests(context.Background(), removed)
}

func removeQueued(items []domain.QueuedTurn, id string) []domain.QueuedTurn {
	out := items[:0]
	for _, item := range items {
		if item.ID != id {
			out = append(out, item)
		}
	}
	return out
}

func newQueueID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "q-" + hex.EncodeToString(b[:])
}

func joinLines(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n\n"
		}
		out += p
	}
	return out
}

// resumeSteeredRun builds the continuation iterator after a boundary-cancel.
// The steer text rides the resume context and is injected by the typed
// steerMiddleware on the resumed run's next model call, so the model sees it
// in the SAME run's history (turn.steered is the single continuity marker).
// Nil iterator = resume unavailable; the caller falls back to terminal settle
// and the items re-enter the follow-up lane.
func (s *Service) resumeSteeredRun(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, items []domain.QueuedTurn, info *adk.InterruptInfo) *adk.AsyncIterator[*adk.TypedAgentEvent[*schema.AgenticMessage]] {
	eng := engineFromContext(ctx)
	if eng == nil {
		eng = s.engine
	}
	if eng == nil || eng.cfg.Checkpoints == nil {
		return nil
	}
	ctx = withSteerText(ctx, steerMessageText(items))
	ctx = withSteerDelivery(ctx, s.newSteerDelivery(sessionID, runID, items))
	// Resume targets must be actual InterruptCtx IDs. The boundary-cancel
	// interrupt carries the agent context plus the pending tool context;
	// walk every context and its parent chain and address the agent-level
	// targets (per adk interrupt_test.go: agent ctx ->
	// *ChatModelAgentResumeData, tool ctx -> the tool's resume value; a
	// missing tool target re-executes the tool, which is what we want for a
	// boundary steer).
	targets := map[string]any{}
	if info != nil {
		for _, c := range info.InterruptContexts {
			for current := c; current != nil; current = current.Parent {
				if len(current.Address) > 0 && current.Address[len(current.Address)-1].Type == adk.AddressSegmentAgent {
					targets[current.ID] = &adk.ChatModelAgentResumeData{}
				}
			}
		}
	}
	if len(targets) == 0 {
		targets["agent:"+eng.agentName] = &adk.ChatModelAgentResumeData{}
	}
	cancelOpt, steerCancel := adk.WithCancel()
	s.mu.Lock()
	s.steerCancels[runID] = steerCancel
	s.mu.Unlock()
	iter, err := eng.Resume(ctx, checkpointIDFor(runID), &adk.ResumeParams{Targets: targets}, cancelOpt)
	if err != nil {
		slog.Warn("steer resume failed", "run", string(runID), "err", err)
		return nil
	}
	return iter
}

// resumeSteeredAsync drives the steer resume leg on its own goroutine: the
// boundary-cancelled iterator is still open inside the old consume frame,
// so the checkpoint resume and the new consume loop must run elsewhere —
// the same pattern as approval/question resumes (resumeSuspended drives
// from the responding goroutine). The run stays in s.active throughout:
// no terminal is emitted for the boundary itself.
func (s *Service) resumeSteeredAsync(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode, ledger *BudgetLedger, items []domain.QueuedTurn, beforeComplete func() error, execution runExecutionOptions) {
	runID := m.runID
	// Register before the old drive leg can leave WaitIdle's owner group.
	// The admitted Go context and its active cancel owner stay live across
	// Eino's native boundary cancellation; only its ADK cancel handle changes.
	s.mu.Lock()
	_, active := s.active[runID]
	s.mu.Unlock()
	if !active {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		var info *adk.InterruptInfo
		if m.interrupt != nil {
			info = m.interrupt.Raw
		}
		// A boundary cancellation ends the old leg, not its admitted authority.
		// Preserve the Run-owned prompt, policy and tool bindings for Eino's
		// checkpoint read; only the cancellation and nudge leg state are fresh.
		steerState := newNudgeState()
		steerCtx := withNudgeEmitter(withNudgeState(ctx, steerState), s.nudgeEmitter(m, sessionID))
		eng := engineFromContext(steerCtx)
		if eng == nil {
			eng = s.engine
		}
		var onDelta func(string)
		if execution.child != nil {
			onDelta = execution.child.appendOutput
		}
		warmer := s.newRunCacheWarmer(steerCtx, m, sessionID, eng, selectedTools)
		steerCtx = s.withLiveModelStreamObserver(steerCtx, m, sessionID, ledger, onDelta, execution.child != nil, warmer)
		steerCtx = withRunCacheWarmer(steerCtx, warmer)
		defer warmer.stop()
		next := s.resumeSteeredRun(steerCtx, sessionID, runID, items, info)
		if next == nil {
			steerCtx := withNudgeEmitter(withNudgeState(context.Background(), newNudgeState()), s.nudgeEmitter(m, sessionID))
			s.emitTerminal(steerCtx, m, m.build(domain.EventRunFailed, payloadRunFailed{Message: "steer resume unavailable"}))
			return
		}
		m.setNudgeState(steerState)
		s.consume(steerCtx, m, sessionID, selectedTools, mode, ledger, next, steerState, beforeComplete, execution)
	}()
}

func (s *Service) queueError(q *sessionQueue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return q.err
}
