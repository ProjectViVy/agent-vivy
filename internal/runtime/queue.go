package runtime

// The Journal owns the queue. Each admission carries the complete pending
// queue and its consumption markers in the same transaction as run.started.
import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"log/slog"
	"reflect"
	"sync"
	"time"
)

var ErrQueueUnavailable = errors.New("runtime: no active run for session")
var errQueueChanged = errors.New("runtime: queue changed before admission")

type sessionQueue struct {
	latestCreatedAt         int64
	runOptions              RunOptions
	mu                      sync.Mutex
	admittedBy              map[string]string
	lastAdmitted            string
	steer, followUp         []domain.QueuedTurn
	steerMode, followUpMode string
	rebuilt                 bool
}

func newSessionQueue() *sessionQueue {
	return &sessionQueue{steerMode: domain.QueueModeAll, followUpMode: domain.QueueModeAll}
}

type QueueState struct {
	LastAdmittedRunID string
	AdmittedRunID     string
	Steering          []domain.QueuedTurn `json:"steering"`
	FollowUps         []domain.QueuedTurn `json:"follow_up"`
	SteerMode         string              `json:"steer_mode"`
	FollowUpMode      string              `json:"follow_up_mode"`
}

func (s *Service) queueFor(ctx context.Context, sid domain.SessionID) (*sessionQueue, error) {
	s.mu.Lock()
	q := s.queues[sid]
	if q == nil {
		q = newSessionQueue()
		s.queues[sid] = q
	}
	s.mu.Unlock()
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.rebuilt {
		if err := s.rebuildQueue(ctx, sid, q); err != nil {
			return nil, err
		}
		q.rebuilt = true
	}
	return q, nil
}

// Replay into a private ordered fold and publish only after iterator and close
// succeed. An upsert keeps its slot; removal followed by enqueue gets a new slot.
func (s *Service) rebuildQueue(ctx context.Context, sid domain.SessionID, q *sessionQueue) error {
	runs, err := s.deps.Runs.ListRunsBySession(ctx, sid)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		return nil
	}
	latest := runs[len(runs)-1].ID
	it, err := s.deps.Journal.Replay(ctx, latest, 0)
	if err != nil {
		return err
	}
	var pending []domain.QueuedTurn
	for it.Next() {
		ev := it.Value().Event
		switch ev.Type {
		case domain.EventTurnQueued:
			var p payloadTurnQueued
			if err = json.Unmarshal(ev.Payload, &p); err != nil {
				_ = it.Close()
				return fmt.Errorf("runtime: decode queue: %w", err)
			}
			item := domain.QueuedTurn{ID: p.QueueID, SessionID: sid, Track: p.Track, Text: p.Text, CreatedAt: time.UnixMilli(ev.CreatedAt)}
			if p.Turn != nil {
				item = *p.Turn
			}
			item.EnqueuedOn = latest
			found := false
			for i := range pending {
				if pending[i].ID == item.ID {
					pending[i] = item
					found = true
					break
				}
			}
			if !found {
				pending = append(pending, item)
			}
		case domain.EventTurnDequeued, domain.EventTurnSteered:
			var p payloadTurnDequeued
			if err = json.Unmarshal(ev.Payload, &p); err != nil {
				_ = it.Close()
				return err
			}
			pending = removeQueued(pending, p.QueueID)
		}
	}
	iterErr := it.Err()
	closeErr := it.Close()
	if err = errors.Join(iterErr, closeErr); err != nil {
		return err
	}
	var steer, follow []domain.QueuedTurn
	for _, item := range pending {
		if item.Track == domain.QueueTrackSteer {
			steer = append(steer, item)
		} else {
			follow = append(follow, item)
		}
	}
	q.steer, q.followUp = steer, follow
	q.latestCreatedAt = runs[len(runs)-1].CreatedAt
	return nil
}
func (s *Service) activeRunForSession(sid domain.SessionID) domain.RunID {
	s.mu.Lock()
	defer s.mu.Unlock()
	for rid, session := range s.runSessions {
		if session == sid && s.active[rid] != nil {
			return rid
		}
	}
	return ""
}
func queuedPayload(item domain.QueuedTurn) payloadTurnQueued {
	return payloadTurnQueued{QueueID: item.ID, Track: item.Track, Text: item.Text, Turn: &item}
}
func (s *Service) journalQueueMarker(ctx context.Context, item domain.QueuedTurn, kind domain.EventType, payload any) error {
	if item.EnqueuedOn == "" {
		return ErrQueueUnavailable
	}
	_, err := s.appendRunEvent(ctx, newEventMapper(item.EnqueuedOn, s.engine.cfg.MaxEventPayloadBytes).build(kind, payload), false)
	return err
}
func (s *Service) Steer(ctx context.Context, sid domain.SessionID, text string) (domain.QueuedTurn, error) {
	return s.SteerWithOptions(ctx, sid, domain.QueuedTurn{Text: text})
}
func (s *Service) FollowUp(ctx context.Context, sid domain.SessionID, text string) (domain.QueuedTurn, error) {
	return s.FollowUpWithOptions(ctx, sid, domain.QueuedTurn{Text: text})
}
func (s *Service) FollowUpWithOptions(ctx context.Context, sid domain.SessionID, item domain.QueuedTurn) (domain.QueuedTurn, error) {
	rid := s.activeRunForSession(sid)
	if rid == "" {
		return domain.QueuedTurn{}, ErrQueueUnavailable
	}
	return s.enqueueTurn(ctx, sid, rid, item, domain.QueueTrackFollowUp)
}
func (s *Service) enqueueTurn(ctx context.Context, sid domain.SessionID, rid domain.RunID, item domain.QueuedTurn, track string) (domain.QueuedTurn, error) {
	q, err := s.queueFor(ctx, sid)
	if err != nil {
		return domain.QueuedTurn{}, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(item.ContextPaths) > 0 && len(item.FileContexts) == 0 {
		return domain.QueuedTurn{}, errors.New("runtime: queued context paths require captured file contexts")
	}
	captured, captureErr := normalizeFileContextsContext(ctx, item.FileContexts)
	if captureErr != nil {
		return domain.QueuedTurn{}, captureErr
	}
	item.FileContexts = captured
	if _, err = normalizeFace(item.Face); err != nil {
		return domain.QueuedTurn{}, err
	}
	item = cloneQueuedTurn(item)
	item.ID = newQueueID()
	item.SessionID = sid
	item.Track = track
	item.CreatedAt = time.Now()
	item.EnqueuedOn = rid
	if _, _, _, _, err = normalizeRunOptions(item.Mode, item.Profile, item.CollaborationMode, item.CollaborationVersion); err != nil {
		return domain.QueuedTurn{}, err
	}
	if _, err = normalizeThinkingMode(item.Thinking); err != nil {
		return domain.QueuedTurn{}, err
	}
	if err = s.journalQueueMarker(ctx, item, domain.EventTurnQueued, queuedPayload(item)); err != nil {
		return domain.QueuedTurn{}, err
	}
	if track == domain.QueueTrackSteer {
		q.steer = append(q.steer, item)
	} else {
		q.followUp = append(q.followUp, item)
	}
	return cloneQueuedTurn(item), nil
}
func (s *Service) SteerWithOptions(ctx context.Context, sid domain.SessionID, item domain.QueuedTurn) (domain.QueuedTurn, error) {
	rid := s.activeRunForSession(sid)
	if rid == "" {
		return domain.QueuedTurn{}, ErrQueueUnavailable
	}
	s.mu.Lock()
	_, suspended := s.pending[rid]
	cancel := s.steerCancels[rid]
	s.mu.Unlock()
	// Attachments and changed run preferences require a new admission; the
	// checkpoint history modifier only delivers text under the current policy.
	q, err := s.queueFor(ctx, sid)
	if err != nil {
		return domain.QueuedTurn{}, err
	}
	q.mu.Lock()
	active := q.runOptions
	compatible := (item.Mode == "" || item.Mode == active.Mode) && (item.Thinking == "" || item.Thinking == active.Thinking) &&
		(item.Face == "" || item.Face == active.Face) && (item.Profile == "" || item.Profile == active.Profile) &&
		(item.CollaborationMode == "" || (item.CollaborationMode == active.CollaborationMode && item.CollaborationVersion == active.CollaborationVersion))
	q.mu.Unlock()
	if suspended || cancel == nil || !compatible || len(item.Attachments) > 0 || len(item.FileContexts) > 0 || item.Continuity != nil {
		return s.enqueueTurn(ctx, sid, rid, item, domain.QueueTrackFollowUp)
	}
	queued, err := s.enqueueTurn(ctx, sid, rid, item, domain.QueueTrackSteer)
	if err != nil {
		return domain.QueuedTurn{}, err
	}
	s.mu.Lock()
	s.steerArmed[rid] = true
	s.mu.Unlock()
	if _, ok := cancel(adk.WithAgentCancelMode(adk.CancelAfterChatModel)); !ok {
		q, err := s.queueFor(ctx, sid)
		if err != nil {
			return domain.QueuedTurn{}, err
		}
		q.mu.Lock()
		defer q.mu.Unlock()
		queued.Track = domain.QueueTrackFollowUp
		if err = s.journalQueueMarker(ctx, queued, domain.EventTurnQueued, queuedPayload(queued)); err != nil {
			return domain.QueuedTurn{}, err
		}
		q.steer = removeQueued(q.steer, queued.ID)
		q.followUp = append(q.followUp, queued)
	}
	return cloneQueuedTurn(queued), nil
}
func queueState(q *sessionQueue) QueueState {
	return QueueState{LastAdmittedRunID: q.lastAdmitted, Steering: cloneQueuedTurns(q.steer), FollowUps: cloneQueuedTurns(q.followUp), SteerMode: q.steerMode, FollowUpMode: q.followUpMode}
}
func (s *Service) QueueState(ctx context.Context, sid domain.SessionID, after domain.RunID) (QueueState, error) {
	q, err := s.queueFor(ctx, sid)
	if err != nil {
		return QueueState{}, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	state := queueState(q)
	state.AdmittedRunID = q.admittedBy[string(after)]
	return state, nil
}
func (s *Service) ClearQueue(ctx context.Context, sid domain.SessionID) (QueueState, error) {
	return s.clearQueue(ctx, sid, "cleared")
}
func (s *Service) clearQueue(ctx context.Context, sid domain.SessionID, reason string) (QueueState, error) {
	q, err := s.queueFor(ctx, sid)
	if err != nil {
		return QueueState{}, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	state := queueState(q)
	items := append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...)
	if len(items) == 0 {
		return state, nil
	}
	carrier := items[0].EnqueuedOn
	if carrier == "" {
		return QueueState{}, ErrQueueUnavailable
	}
	events := make([]domain.RunEvent, 0, len(items))
	for _, item := range items {
		if item.EnqueuedOn != carrier {
			return QueueState{}, errQueueChanged
		}
		events = append(events, newEventMapper(carrier, s.engine.cfg.MaxEventPayloadBytes).build(domain.EventTurnDequeued, payloadTurnDequeued{QueueID: item.ID, Track: item.Track, Reason: reason, Text: item.Text, Turn: &item}))
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.sessionDeleted(sid) {
		return QueueState{}, storage.ErrNotFound
	}
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: carrier, Events: events})
	if err != nil {
		return QueueState{}, err
	}
	q.steer, q.followUp = nil, nil
	for i, event := range events {
		event.Seq = seq - domain.EventSeq(len(events)-i-1)
		s.publish(ctx, event)
	}
	return state, nil
}
func (s *Service) Dequeue(ctx context.Context, sid domain.SessionID) (domain.QueuedTurn, bool, error) {
	q, err := s.queueFor(ctx, sid)
	if err != nil {
		return domain.QueuedTurn{}, false, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.followUp) == 0 {
		return domain.QueuedTurn{}, false, nil
	}
	return s.removeQueueLocked(ctx, q, q.followUp[len(q.followUp)-1])
}
func (s *Service) QueueRemove(ctx context.Context, sid domain.SessionID, id string) (domain.QueuedTurn, bool, error) {
	q, err := s.queueFor(ctx, sid)
	if err != nil {
		return domain.QueuedTurn{}, false, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, lane := range [][]domain.QueuedTurn{q.steer, q.followUp} {
		for _, item := range lane {
			if item.ID == id {
				return s.removeQueueLocked(ctx, q, item)
			}
		}
	}
	return domain.QueuedTurn{}, false, nil
}
func (s *Service) removeQueueLocked(ctx context.Context, q *sessionQueue, item domain.QueuedTurn) (domain.QueuedTurn, bool, error) {
	if err := s.journalQueueMarker(ctx, item, domain.EventTurnDequeued, payloadTurnDequeued{QueueID: item.ID, Track: item.Track, Reason: "dequeued", Text: item.Text, Turn: &item}); err != nil {
		return domain.QueuedTurn{}, false, err
	}
	q.steer = removeQueued(q.steer, item.ID)
	q.followUp = removeQueued(q.followUp, item.ID)
	return cloneQueuedTurn(item), true, nil
}
func (s *Service) SetQueueMode(ctx context.Context, sid domain.SessionID, track, mode string) error {
	if mode != domain.QueueModeAll && mode != domain.QueueModeOneAtATime {
		return fmt.Errorf("runtime: invalid queue mode %q", mode)
	}
	q, err := s.queueFor(ctx, sid)
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
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
func (s *Service) takeSteerTurn(sid domain.SessionID, rid domain.RunID, expected ...[]domain.QueuedTurn) ([]domain.QueuedTurn, error) {
	ctx := context.Background()
	q, err := s.queueFor(ctx, sid)
	if err != nil {
		return nil, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	count := len(q.steer)
	if q.steerMode == domain.QueueModeOneAtATime && count > 1 {
		count = 1
	}
	if len(expected) > 0 {
		count = len(expected[0])
		if !queuePrefixMatches(q.steer, expected[0]) {
			return nil, errQueueChanged
		}
	}
	if count == 0 {
		return nil, nil
	}
	out := append([]domain.QueuedTurn(nil), q.steer[:count]...)
	var events []domain.RunEvent
	var messages []domain.Message
	for _, item := range out {
		events = append(events, newEventMapper(rid, s.engine.cfg.MaxEventPayloadBytes).build(domain.EventTurnSteered, payloadTurnSteered{QueueID: item.ID, Text: item.Text}))
		messages = append(messages, domain.Message{ID: "steer-" + item.ID, RunID: rid, SessionID: sid, Role: domain.RoleUser, Content: item.Text, CreatedAt: time.Now().UnixMilli()})
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.sessionDeleted(sid) {
		return nil, storage.ErrNotFound
	}
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: rid, Events: events, Messages: messages})
	if err != nil {
		return nil, err
	}
	q.steer = q.steer[count:]
	for i, ev := range events {
		ev.Seq = seq - domain.EventSeq(len(events)-i-1)
		s.publish(ctx, ev)
	}
	return out, nil
}
func turnOptions(item domain.QueuedTurn) RunOptions {
	return RunOptions{Mode: item.Mode, Thinking: item.Thinking, Face: item.Face, Profile: item.Profile, Attachments: item.Attachments, FileContexts: item.FileContexts, CollaborationMode: item.CollaborationMode, CollaborationVersion: item.CollaborationVersion, Continuity: item.Continuity}
}

// Batch only contiguous deliveries with matching options. Multimodal and
// continuity submissions remain individual admissions, preserving their bounds.
func compatibleTurns(a, b domain.QueuedTurn) bool {
	if len(a.Attachments)+len(b.Attachments)+len(a.FileContexts)+len(b.FileContexts) > 0 || a.Continuity != nil || b.Continuity != nil {
		return false
	}
	return reflect.DeepEqual(turnOptions(a), turnOptions(b))
}
func (s *Service) drainFollowUps(sid domain.SessionID, settling domain.RunID) {
	ctx := context.Background()
	for {
		q, err := s.queueFor(ctx, sid)
		if err != nil {
			slog.Warn("queue replay failed", "err", err)
			return
		}
		q.mu.Lock()
		pending := append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...)
		if len(pending) == 0 {
			q.mu.Unlock()
			return
		}
		count := 1
		if q.followUpMode == domain.QueueModeAll {
			for count < len(pending) && compatibleTurns(pending[0], pending[count]) {
				count++
			}
		}
		items := cloneQueuedTurns(pending[:count])
		q.mu.Unlock()
		texts := make([]string, 0, len(items))
		for _, item := range items {
			texts = append(texts, item.Text)
		}
		opts := turnOptions(items[0])
		opts.queueItems = items
		opts.queueAfter = settling
		_, err = s.RunWithOptions(ctx, sid, joinLines(texts), opts)
		if errors.Is(err, errQueueChanged) {
			continue
		}
		if err != nil {
			slog.Warn("follow-up admission failed", "session", string(sid), "err", err)
		}
		return
	}
}
func queuePrefixMatches(pending, selected []domain.QueuedTurn) bool {
	if len(pending) < len(selected) {
		return false
	}
	for i, item := range selected {
		if pending[i].ID != item.ID {
			return false
		}
	}
	return true
}

// Called under q.mu in the existing admission gate. All markers are part of
// the admission transaction, so the newest journal always owns the entire tail.
func queueAdmissionEvents(m *eventMapper, q *sessionQueue, items []domain.QueuedTurn) []domain.RunEvent {
	var events []domain.RunEvent
	for _, item := range append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...) {
		item.Track = domain.QueueTrackFollowUp
		item.EnqueuedOn = m.runID
		events = append(events, m.build(domain.EventTurnQueued, queuedPayload(item)))
	}
	for _, item := range items {
		events = append(events, m.build(domain.EventTurnDequeued, payloadTurnDequeued{QueueID: item.ID, Track: item.Track, Reason: "started", NextRunID: string(m.runID)}))
	}
	return events
}
func queueAdmitted(q *sessionQueue, items []domain.QueuedTurn, rid, after domain.RunID) {
	q.followUp = append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...)
	q.steer = nil
	for _, item := range items {
		q.followUp = removeQueued(q.followUp, item.ID)
	}
	for i := range q.followUp {
		q.followUp[i].Track = domain.QueueTrackFollowUp
		q.followUp[i].EnqueuedOn = rid
	}
	if len(items) > 0 {
		q.lastAdmitted = string(rid)
		if q.admittedBy == nil {
			q.admittedBy = map[string]string{}
		}
		q.admittedBy[string(after)] = string(rid)
	}
}
func (s *Service) steerPendingCancel(rid domain.RunID, sid domain.SessionID) ([]domain.QueuedTurn, error) {
	s.mu.Lock()
	armed := s.steerArmed[rid]
	delete(s.steerArmed, rid)
	s.mu.Unlock()
	if !armed {
		return nil, nil
	}
	q, err := s.queueFor(context.Background(), sid)
	if err != nil {
		return nil, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	count := len(q.steer)
	if q.steerMode == domain.QueueModeOneAtATime && count > 1 {
		count = 1
	}
	return cloneQueuedTurns(q.steer[:count]), nil
}
func (s *Service) pendingQueueDepth(ctx context.Context, sid domain.SessionID) int {
	state, err := s.QueueState(ctx, sid, "")
	if err != nil {
		slog.Warn("queue replay failed", "err", err)
		return 0
	}
	return len(state.Steering) + len(state.FollowUps)
}
func (s *Service) dropSessionQueue(sid domain.SessionID) {
	s.mu.Lock()
	delete(s.queues, sid)
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
func (s *Service) resumeSteeredRun(ctx context.Context, runID domain.RunID, items []domain.QueuedTurn, info *adk.InterruptInfo, ready <-chan struct{}, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	if s.engine == nil || s.engine.cfg.Checkpoints == nil {
		return nil
	}
	msg := steeredMessage(items)
	modifier := func(ctx context.Context, history []adk.Message) []adk.Message {
		select {
		case <-ready:
		case <-ctx.Done():
			return history
		}
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
	iter, err := s.engine.Resume(ctx, checkpointIDFor(runID), &adk.ResumeParams{Targets: targets}, opts...)
	if err != nil {
		slog.Warn("steer resume failed", "run", string(runID), "err", err)
		return nil
	}
	return iter
}

func (s *Service) resumeSteeredAsync(ctx context.Context, m *eventMapper, sid domain.SessionID, selected []string, mode domain.RunMode, ledger *BudgetLedger, items []domain.QueuedTurn, beforeComplete func() error, execution runExecutionOptions) {
	rid := m.runID
	// Preserve run identity, thinking and observers; install cancellation for the
	// new phase before a second steer or user abort can reach it.
	resumedCtx, cancel := context.WithCancel(ctx)
	cancelOpt, steerCancel := adk.WithCancel()
	s.mu.Lock()
	if s.active[rid] == nil {
		s.mu.Unlock()
		cancel()
		return
	}
	s.steerCancels[rid] = steerCancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// A following resumed phase inherits this run context. Its lifetime ends
		// with the original active cancel, not when this phase hands off.
		var info *adk.InterruptInfo
		if m.interrupt != nil {
			info = m.interrupt.Raw
		}
		state := newNudgeState()
		m.setNudgeState(state)
		resumedCtx = withNudgeEmitter(withNudgeState(resumedCtx, state), s.nudgeEmitter(m, sid))
		ready := make(chan struct{})
		next := s.resumeSteeredRun(resumedCtx, rid, items, info, ready, cancelOpt)
		if next == nil {
			s.emitTerminal(resumedCtx, m, s.terminalEvent(resumedCtx, m, errors.New("steer resume unavailable")))
			return
		}
		if _, err := s.takeSteerTurn(sid, rid, items); err != nil {
			cancel()
			// Drain the cancelled native iterator so its checkpoint lifecycle ends
			// before terminal fallback admits another run.
			for {
				if _, ok := next.Next(); !ok {
					break
				}
			}
			s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
			return
		}
		close(ready)
		s.consume(resumedCtx, m, sid, selected, mode, ledger, next, state, beforeComplete, execution)
	}()
}

func cloneQueuedTurns(items []domain.QueuedTurn) []domain.QueuedTurn {
	out := make([]domain.QueuedTurn, len(items))
	for i, item := range items {
		out[i] = cloneQueuedTurn(item)
	}
	return out
}
func cloneQueuedTurn(item domain.QueuedTurn) domain.QueuedTurn {
	item.ContextPaths = append([]string(nil), item.ContextPaths...)
	item.Attachments = append([]domain.Attachment(nil), item.Attachments...)
	for i := range item.Attachments {
		item.Attachments[i].Data = append([]byte(nil), item.Attachments[i].Data...)
	}
	item.FileContexts = append([]domain.FileContext(nil), item.FileContexts...)
	for i := range item.FileContexts {
		item.FileContexts[i].Content = append([]byte(nil), item.FileContexts[i].Content...)
	}
	if item.Continuity != nil {
		data, _ := json.Marshal(item.Continuity)
		var copy domain.ContinuityInput
		_ = json.Unmarshal(data, &copy)
		item.Continuity = &copy
	}
	return item
}
