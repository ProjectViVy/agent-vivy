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
// admission re-journals the remaining tail onto the new run at start.
// Restart recovery replays the session's latest run journal only.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// ErrQueueUnavailable marks steer attempts that cannot reach a live run
// AND cannot degrade (e.g. boundary cancel unavailable); callers fall back
// to the follow-up lane or a fresh run.
var ErrQueueUnavailable = errors.New("runtime: no active run for session")

type sessionQueue struct {
	// admittedBy maps a settling run id to the follow-up run the kernel
	// auto-started for it. Faces resolve it via queue/state{after_run_id}
	// after the terminal — a post-terminal wire hint cannot be journaled
	// and the bus closes on terminal, so discovery goes through state.
	admittedBy map[string]string
	// lastAdmitted is the most recently auto-started run id, exposed on
	// session/get for face-free consumers.
	lastAdmitted string
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
	q, ok := s.queues[sessionID]
	if !ok {
		q = newSessionQueue()
		s.queues[sessionID] = q
	}
	if q.rebuilt {
		s.mu.Unlock()
		return q
	}
	q.rebuilt = true
	s.mu.Unlock()

	s.rebuildQueue(ctx, sessionID, q)
	return q
}

// rebuildQueue replays the session's newest run journal and recovers items
// queued but never consumed (restart mid-queue).
func (s *Service) rebuildQueue(ctx context.Context, sessionID domain.SessionID, q *sessionQueue) {
	runs, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) == 0 {
		return
	}
	it, err := s.deps.Journal.Replay(ctx, runs[len(runs)-1].ID, 0)
	if err != nil {
		return
	}
	defer func() { _ = it.Close() }()
	queued := map[string]domain.QueuedTurn{}
	consumed := map[string]bool{}
	for it.Next() {
		ev := it.Value().Event
		switch ev.Type {
		case domain.EventTurnQueued:
			var p payloadTurnQueued
			if json.Unmarshal(ev.Payload, &p) != nil {
				continue
			}
			queued[p.QueueID] = domain.QueuedTurn{
				ID: p.QueueID, SessionID: sessionID, Track: p.Track,
				Text: p.Text, EnqueuedOn: runs[len(runs)-1].ID, CreatedAt: time.UnixMilli(ev.CreatedAt),
			}
		case domain.EventTurnDequeued, domain.EventTurnSteered:
			var p struct {
				QueueID string `json:"queue_id"`
			}
			if json.Unmarshal(ev.Payload, &p) == nil {
				consumed[p.QueueID] = true
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range queued {
		if consumed[item.ID] {
			continue
		}
		switch item.Track {
		case domain.QueueTrackSteer:
			q.steer = append(q.steer, item)
		default:
			q.followUp = append(q.followUp, item)
		}
	}
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

// journalQueueMarker appends a queue marker on the enqueuing run's journal.
// A settled journal rejects the append — the item stays in memory and is
// re-materialized onto the next run at admission (durability invariant).
func (s *Service) journalQueueMarker(ctx context.Context, item domain.QueuedTurn, eventType domain.EventType, payload any) {
	if item.EnqueuedOn == "" {
		return
	}
	s.journalReviewEvent(ctx, item.EnqueuedOn, eventType, payload)
}

// Steer queues text on the steer lane and schedules a boundary-safe cancel
// of the session's active run. With no active run it returns
// ErrQueueUnavailable so the caller can start a run (steer-as-prompt) or
// fall back to the follow-up lane.
func (s *Service) Steer(ctx context.Context, sessionID domain.SessionID, text string) (domain.QueuedTurn, error) {
	runID := s.activeRunForSession(sessionID)
	if runID == "" {
		return domain.QueuedTurn{}, ErrQueueUnavailable
	}
	s.mu.Lock()
	_, suspended := s.pending[runID]
	cancelFn, cancellable := s.steerCancels[runID]
	s.mu.Unlock()
	if suspended || !cancellable {
		// Suspended on approval/question, or the run predates the cancel
		// seam: the text belongs on the follow-up lane — steering mid-tool
		// would violate the boundary rule anyway.
		return s.enqueueFollowUp(ctx, sessionID, text, runID)
	}
	item := domain.QueuedTurn{
		ID: newQueueID(), SessionID: sessionID, Track: domain.QueueTrackSteer,
		Text: text, CreatedAt: time.Now(), EnqueuedOn: runID,
	}
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	q.steer = append(q.steer, item)
	s.mu.Unlock()
	s.journalQueueMarker(ctx, item, domain.EventTurnQueued, payloadTurnQueued{
		QueueID: item.ID, Track: item.Track, Text: text,
	})
	// Boundary-safe cancel: CancelAfterChatModel lets the in-flight
	// tool-call batch and the current model turn settle before
	// interrupting — never mid-tool-flight, and the checkpointed history
	// ends with an assistant message so the resume can append the steer
	// text as a user message (CancelAfterToolCalls checkpoints with a
	// pending assistant(tool_calls) tail, which ToolNode rejects).
	// steerArmed lets consume tell this cancel apart from a user abort.
	s.mu.Lock()
	s.steerArmed[runID] = true
	s.mu.Unlock()
	if _, ok := cancelFn(adk.WithAgentCancelMode(adk.CancelAfterChatModel)); !ok {
		// Run already settled between lookup and cancel: demote to follow-up
		// so the text is not lost.
		s.mu.Lock()
		q.steer = removeQueued(q.steer, item.ID)
		s.mu.Unlock()
		s.journalQueueMarker(ctx, item, domain.EventTurnDequeued, payloadTurnDequeued{
			QueueID: item.ID, Track: domain.QueueTrackSteer, Reason: "aborted", Text: item.Text,
		})
		item.Track = domain.QueueTrackFollowUp
		s.mu.Lock()
		q.followUp = append(q.followUp, item)
		s.mu.Unlock()
		s.journalQueueMarker(ctx, item, domain.EventTurnQueued, payloadTurnQueued{
			QueueID: item.ID, Track: domain.QueueTrackFollowUp, Text: text,
		})
	}
	return item, nil
}

// FollowUp queues text behind the session's active run. With no active run
// it returns ErrQueueUnavailable — the caller then issues the equivalent of
// a prompt (turn/start) instead of queuing.
func (s *Service) FollowUp(ctx context.Context, sessionID domain.SessionID, text string) (domain.QueuedTurn, error) {
	runID := s.activeRunForSession(sessionID)
	if runID == "" {
		return domain.QueuedTurn{}, ErrQueueUnavailable
	}
	return s.enqueueFollowUp(ctx, sessionID, text, runID)
}

func (s *Service) enqueueFollowUp(ctx context.Context, sessionID domain.SessionID, text string, runID domain.RunID) (domain.QueuedTurn, error) {
	item := domain.QueuedTurn{
		ID: newQueueID(), SessionID: sessionID, Track: domain.QueueTrackFollowUp,
		Text: text, CreatedAt: time.Now(), EnqueuedOn: runID,
	}
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	q.followUp = append(q.followUp, item)
	s.mu.Unlock()
	s.journalQueueMarker(ctx, item, domain.EventTurnQueued, payloadTurnQueued{
		QueueID: item.ID, Track: item.Track, Text: text,
	})
	return item, nil
}

// ClearQueue drains both lanes and returns the texts (pi returns them for
// editor restore). Consumption is journaled per item.
func (s *Service) ClearQueue(ctx context.Context, sessionID domain.SessionID) QueueState {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	state := QueueState{
		Steering:     append([]domain.QueuedTurn(nil), q.steer...),
		FollowUps:    append([]domain.QueuedTurn(nil), q.followUp...),
		SteerMode:    q.steerMode,
		FollowUpMode: q.followUpMode,
	}
	cleared := append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...)
	q.steer, q.followUp = nil, nil
	s.mu.Unlock()
	for _, item := range cleared {
		s.journalQueueMarker(ctx, item, domain.EventTurnDequeued, payloadTurnDequeued{
			QueueID: item.ID, Track: item.Track, Reason: "cleared", Text: item.Text,
		})
	}
	return state
}

// Dequeue pops the most recently enqueued still-pending item (follow-up
// tail — steer items arm a boundary cancel at enqueue and are already in
// flight, so only the follow-up lane is withdrawable) and journals
// turn.dequeued{reason:"dequeued"}. pi: Alt+Up restores queued text into
// the editor. Returns ok=false when the lane is empty.
func (s *Service) Dequeue(ctx context.Context, sessionID domain.SessionID) (domain.QueuedTurn, bool) {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	if len(q.followUp) == 0 {
		s.mu.Unlock()
		return domain.QueuedTurn{}, false
	}
	item := q.followUp[len(q.followUp)-1]
	q.followUp = q.followUp[:len(q.followUp)-1]
	s.mu.Unlock()
	s.journalQueueMarker(ctx, item, domain.EventTurnDequeued, payloadTurnDequeued{
		QueueID: item.ID, Track: item.Track, Reason: "dequeued", Text: item.Text,
	})
	return item, true
}

// QueueRemove cancels one still-pending item by id (either lane) and
// journals turn.dequeued{reason:"dequeued"}. GUI's per-item cancel
// affordance; items already armed/admitted are gone from the lanes and
// report ok=false.
func (s *Service) QueueRemove(ctx context.Context, sessionID domain.SessionID, queueID string) (domain.QueuedTurn, bool) {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	var item domain.QueuedTurn
	found := false
	for _, lane := range []*[]domain.QueuedTurn{&q.steer, &q.followUp} {
		for _, candidate := range *lane {
			if candidate.ID == queueID {
				item = candidate
				*lane = removeQueued(*lane, queueID)
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	s.mu.Unlock()
	if !found {
		return domain.QueuedTurn{}, false
	}
	s.journalQueueMarker(ctx, item, domain.EventTurnDequeued, payloadTurnDequeued{
		QueueID: item.ID, Track: item.Track, Reason: "dequeued", Text: item.Text,
	})
	return item, true
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
		LastAdmittedRunID: q.lastAdmitted,
		AdmittedRunID:     q.admittedBy[string(afterRun)],
		Steering:          append([]domain.QueuedTurn(nil), q.steer...),
		FollowUps:         append([]domain.QueuedTurn(nil), q.followUp...),
		SteerMode:         q.steerMode,
		FollowUpMode:      q.followUpMode,
	}
}

// SetQueueMode sets a lane's drain mode ("all" | "one-at-a-time").
func (s *Service) SetQueueMode(ctx context.Context, sessionID domain.SessionID, track, mode string) error {
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

// takeSteerHead pops the next steer item the boundary-cancel should inject.
// Mode "all" drains every queued steer into one delivery (the resume leg
// appends them all); "one-at-a-time" serves only the head.
func (s *Service) takeSteerTurn(sessionID domain.SessionID, runID domain.RunID) []domain.QueuedTurn {
	q := s.queueFor(context.Background(), sessionID)
	s.mu.Lock()
	if len(q.steer) == 0 {
		s.mu.Unlock()
		return nil
	}
	var out []domain.QueuedTurn
	if q.steerMode == domain.QueueModeAll {
		out = q.steer
		q.steer = nil
	} else {
		out = q.steer[:1]
		q.steer = q.steer[1:]
	}
	s.mu.Unlock()
	// Journal the continuity marker outside the service lock — journal
	// append blocks on storage. The steered text also lands in the message
	// store (pi: a steered message is a permanent transcript row); the
	// HistoryModifier injects it into model-visible history but never
	// touches Messages.
	for _, item := range out {
		s.journalQueueMarker(context.Background(), item, domain.EventTurnSteered, payloadTurnSteered{
			QueueID: item.ID, Text: item.Text,
		})
		if s.deps.Messages != nil {
			_ = s.deps.Messages.AppendMessage(context.Background(), domain.Message{
				ID:        newMessageID(),
				SessionID: sessionID,
				Role:      domain.RoleUser,
				Content:   item.Text,
				CreatedAt: time.Now().UnixMilli(),
			})
		}
	}
	return out
}

// drainFollowUps runs at terminal settle of a human turn: pop the follow-up
// lane per its mode, start the next run with the admitted texts joined into
// one prompt, journal each admission as turn.dequeued{reason:"started"}
// on the NEW run's journal, announce it on the settling run's topic
// (publish-only wire hint carrying next_run_id — subscribed faces learn
// the auto-started run), then re-materialize the tail (the "newest run
// holds the whole queue" invariant). Runs inline — the session admission
// gate only serializes run starts.
// drainFollowUps admits the next follow-up delivery when a human turn
// settles. Admission markers journal on the NEW run's journal (the
// queue's materialized truth); the admitted run id is recorded on the
// session queue for faces to resolve via queue/state after the terminal.
func (s *Service) drainFollowUps(sessionID domain.SessionID, settlingRun domain.RunID) {
	ctx := context.Background()
	items := s.popFollowUps(ctx, sessionID)
	if len(items) == 0 {
		return
	}
	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, item.Text)
	}
	runID, err := s.Run(ctx, sessionID, joinLines(texts))
	if err != nil {
		// Admission failed: re-queue the items at the head so the next
		// settle (or an explicit dequeue) can still serve them.
		s.mu.Lock()
		if q := s.queues[sessionID]; q != nil {
			q.followUp = append(items, q.followUp...)
		}
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	if q := s.queues[sessionID]; q != nil {
		q.lastAdmitted = string(runID)
		if q.admittedBy == nil {
			q.admittedBy = map[string]string{}
		}
		q.admittedBy[string(settlingRun)] = string(runID)
	}
	s.mu.Unlock()
	for _, item := range items {
		item.EnqueuedOn = runID
		s.journalQueueMarker(ctx, item, domain.EventTurnDequeued, payloadTurnDequeued{
			QueueID: item.ID, Track: item.Track, Reason: "started", NextRunID: string(runID),
		})
	}
	s.materializeQueueTail(ctx, sessionID, runID)
}

// popFollowUps removes the next follow-up delivery per lane mode. Steer
// items stranded by a normal completion (the boundary cancel never fired)
// demote into the head of the follow-up lane first — a steer that missed
// its window degrades to "next-turn prompt", never a lost message.
func (s *Service) popFollowUps(ctx context.Context, sessionID domain.SessionID) []domain.QueuedTurn {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	if len(q.steer) > 0 {
		demoted := make([]domain.QueuedTurn, 0, len(q.steer))
		for _, item := range q.steer {
			item.Track = domain.QueueTrackFollowUp
			demoted = append(demoted, item)
		}
		q.followUp = append(demoted, q.followUp...)
		q.steer = nil
	}
	var admitted []domain.QueuedTurn
	switch {
	case len(q.followUp) == 0:
	case q.followUpMode == domain.QueueModeAll:
		admitted = q.followUp
		q.followUp = nil
	default:
		admitted = q.followUp[:1]
		q.followUp = q.followUp[1:]
	}
	s.mu.Unlock()
	return admitted
}

// materializeQueueTail moves leftover steer items into the follow-up lane
// and re-journals everything pending onto the new run's journal.
func (s *Service) materializeQueueTail(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	var tail []domain.QueuedTurn
	for _, item := range q.steer {
		item.Track = domain.QueueTrackFollowUp
		tail = append(tail, item)
	}
	q.steer = nil
	q.followUp = append(tail, q.followUp...)
	for i := range q.followUp {
		q.followUp[i].EnqueuedOn = runID
	}
	pending := append([]domain.QueuedTurn(nil), q.followUp...)
	s.mu.Unlock()
	for _, item := range pending {
		s.journalQueueMarker(ctx, item, domain.EventTurnQueued, payloadTurnQueued{
			QueueID: item.ID, Track: item.Track, Text: item.Text,
		})
	}
}

// steerPendingCancel consumes the armed flag and pops the steer lane when
// the consume loop observed the boundary cancel it triggered.
func (s *Service) steerPendingCancel(runID domain.RunID, sessionID domain.SessionID) []domain.QueuedTurn {
	s.mu.Lock()
	armed := s.steerArmed[runID]
	delete(s.steerArmed, runID)
	s.mu.Unlock()
	if !armed {
		return nil
	}
	return s.takeSteerTurn(sessionID, runID)
}

// demoteSteerItems returns steered items to the follow-up lane when the
// resume leg could not be built — the text survives as a queued turn
// instead of silently dropping.
func (s *Service) demoteSteerItems(ctx context.Context, sessionID domain.SessionID, items []domain.QueuedTurn) {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	for i := range items {
		items[i].Track = domain.QueueTrackFollowUp
	}
	q.followUp = append(items, q.followUp...)
	s.mu.Unlock()
	for _, item := range items {
		s.journalQueueMarker(ctx, item, domain.EventTurnQueued, payloadTurnQueued{
			QueueID: item.ID, Track: item.Track, Text: item.Text,
		})
	}
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
	delete(s.queues, sessionID)
	s.mu.Unlock()
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

// steeredMessage builds the user message a resume leg injects into the
// model-visible history. Multiple steer items under mode "all" join into
// one delivery — pi concatenates queued steering into the next turn.
func steeredMessage(items []domain.QueuedTurn) *schema.Message {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Text)
	}
	return schema.UserMessage(joinLines(parts))
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

// resumeSteeredRun builds the continuation iterator after a boundary-cancel:
// checkpoint resume with a HistoryModifier that appends the steered user
// message — the model sees it in the SAME run's history (turn.steered is the
// single continuity marker). Nil iterator = resume unavailable; the caller
// falls back to terminal settle and the items re-enter the follow-up lane.
func (s *Service) resumeSteeredRun(ctx context.Context, runID domain.RunID, items []domain.QueuedTurn, info *adk.InterruptInfo) *adk.AsyncIterator[*adk.AgentEvent] {
	if s.engine == nil || s.engine.cfg.Checkpoints == nil {
		return nil
	}
	msg := steeredMessage(items)
	modifier := func(_ context.Context, history []adk.Message) []adk.Message {
		// A boundary-cancel checkpoint pins the next node to ToolNode
		// with a trailing assistant(tool_calls) message, which must stay
		// last or ToolNode rejects the resume. Land the steer text just
		// before it: the pending calls still execute, and the model sees
		// the steer inside the same turn's history.
		if n := len(history); n > 0 {
			if last := history[n-1]; last.Role == schema.Assistant && len(last.ToolCalls) > 0 {
				out := make([]adk.Message, 0, n+1)
				out = append(out, history[:n-1]...)
				return append(out, msg, last)
			}
		}
		return append(history, msg)
	}
	// Resume targets must be actual InterruptCtx IDs. The boundary-cancel
	// interrupt carries the agent context plus the pending tool context;
	// walk every context and its parent chain and address the agent-level
	// targets with the history modifier (per adk interrupt_test.go: agent
	// ctx -> *ChatModelAgentResumeData, tool ctx -> the tool's resume
	// value; a missing tool target re-executes the tool, which is what we
	// want for a boundary steer).
	targets := map[string]any{}
	if info != nil {
		for _, c := range info.InterruptContexts {
			for ctx := c; ctx != nil; ctx = ctx.Parent {
				if len(ctx.Address) > 0 && ctx.Address[len(ctx.Address)-1].Type == adk.AddressSegmentAgent {
					targets[ctx.ID] = &adk.ChatModelAgentResumeData{HistoryModifier: modifier}
				}
			}
		}
	}
	if len(targets) == 0 {
		targets["agent:"+s.engine.agentName] = &adk.ChatModelAgentResumeData{HistoryModifier: modifier}
	}
	iter, err := s.engine.Resume(ctx, checkpointIDFor(runID), &adk.ResumeParams{Targets: targets})
	if err != nil {
		slog.Warn("steer resume failed", "run", string(runID), "err", err)
		return nil
	}
	return iter
}

// flushQueue empties both lanes on a user abort, journaling every item as
// turn.dequeued{reason:"aborted"}.
func (s *Service) flushQueue(sessionID domain.SessionID) {
	ctx := context.Background()
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	cleared := append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...)
	q.steer, q.followUp = nil, nil
	s.mu.Unlock()
	for _, item := range cleared {
		s.journalQueueMarker(ctx, item, domain.EventTurnDequeued, payloadTurnDequeued{
			QueueID: item.ID, Track: item.Track, Reason: "aborted", Text: item.Text,
		})
	}
}

// resumeSteeredAsync drives the steer resume leg on its own goroutine: the
// boundary-cancelled iterator is still open inside the old consume frame,
// so the checkpoint resume and the new consume loop must run elsewhere —
// the same pattern as approval/question resumes (resumeSuspended drives
// from the responding goroutine). The run stays in s.active throughout:
// no terminal is emitted for the boundary itself.
func (s *Service) resumeSteeredAsync(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode, ledger *BudgetLedger, items []domain.QueuedTurn, beforeComplete func() error, execution runExecutionOptions) {
	runID := m.runID
	go func() {
		var info *adk.InterruptInfo
		if m.interrupt != nil {
			info = m.interrupt.Raw
		}
		next := s.resumeSteeredRun(context.Background(), runID, items, info)
		if next == nil {
			s.demoteSteerItems(context.Background(), sessionID, items)
			steerCtx := withNudgeEmitter(withNudgeState(context.Background(), newNudgeState()), s.nudgeEmitter(m, sessionID))
			s.emitTerminal(steerCtx, m, m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: "steer resume unavailable"}))
			return
		}
		steerState := newNudgeState()
		m.setNudgeState(steerState)
		steerCtx := withNudgeEmitter(withNudgeState(context.Background(), steerState), s.nudgeEmitter(m, sessionID))
		s.consume(steerCtx, m, sessionID, selectedTools, mode, ledger, next, steerState, beforeComplete, execution)
	}()
}
