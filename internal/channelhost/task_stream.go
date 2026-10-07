// taskStream is the §8 replay-then-live subscription: subscribe before
// reading, emit the committed prefix under one fixed watermark, then follow
// the tail through the wakeup bus with a 1-second fallback tick. Neither a
// closed bus nor channel EOF proves termination — only a committed
// terminal (or delivered interrupted) state ends the stream.
package channelhost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/journalview"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/channel"
)

// taskCursor is the opaque subscription cursor (§8.4): version, the bound
// run, the last delivered event seq, and an ordinal for multi-update
// events — base64url JSON, at most 256 bytes.
type taskCursor struct {
	V       int             `json:"v"`
	RunID   domain.RunID    `json:"r"`
	Seq     domain.EventSeq `json:"s"`
	Ordinal int             `json:"o"`
}

const taskCursorMaxBytes = 256

func encodeTaskCursor(c taskCursor) (string, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	if len(raw) > taskCursorMaxBytes {
		return "", taskErr(channel.TaskErrLimit, "task cursor exceeds 256 bytes")
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeTaskCursor(s string, wantRun domain.RunID, maxSeq domain.EventSeq) (taskCursor, error) {
	inv := taskErr(channel.TaskErrCursorInvalid, "invalid task cursor")
	if len(s) > taskCursorMaxBytes*4/3+8 {
		return taskCursor{}, inv
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return taskCursor{}, inv
	}
	var c taskCursor
	if err := json.Unmarshal(raw, &c); err != nil || c.V != 1 {
		return taskCursor{}, inv
	}
	if c.RunID != wantRun || c.Seq < 0 || c.Seq > maxSeq {
		// Foreign run or a seq newer than committed reality.
		return taskCursor{}, taskErr(channel.TaskErrCursorInvalid, "task cursor does not match the task")
	}
	return c, nil
}

// isInterruptEvent — INPUT_REQUIRED / AUTH_REQUIRED deliver their status
// and then the stream ends (the run does not).
func isInterruptEvent(t domain.EventType) bool {
	return t == domain.EventUserQuestionRequired || t == domain.EventToolApprovalRequired
}

func isInterruptState(st channel.TaskState) bool {
	return st == channel.TaskStateInputRequired || st == channel.TaskStateAuthorizationRequired
}

func isTerminalState(st channel.TaskState) bool {
	return st == channel.TaskStateCompleted || st == channel.TaskStateFailed || st == channel.TaskStateCanceled
}

func isTerminalStreamEvent(t domain.EventType) bool {
	return t == domain.EventRunCompleted || t == domain.EventRunFailed || t == domain.EventRunCancelled
}

// taskStream implements channel.TaskStream with synchronous backpressure:
// the producer blocks until a Next call accepts the record or Close fires.
type taskStream struct {
	out      chan channel.TaskUpdate
	done     chan struct{}
	err      chan error // buffered 1: terminal failure of the producer
	closeCh  sync.Once
	closeAll func() // unregisters the live bus subscription
}

func newTaskStream() *taskStream {
	return &taskStream{
		out:  make(chan channel.TaskUpdate, 64),
		done: make(chan struct{}),
		err:  make(chan error, 1),
	}
}

func (s *taskStream) Next(ctx context.Context) (channel.TaskUpdate, error) {
	select {
	case u, ok := <-s.out:
		if !ok {
			return channel.TaskUpdate{}, io.EOF
		}
		return u, nil
	case err := <-s.err:
		return channel.TaskUpdate{}, err
	case <-ctx.Done():
		return channel.TaskUpdate{}, ctx.Err()
	case <-s.done:
		return channel.TaskUpdate{}, io.EOF
	}
}

func (s *taskStream) Close() error {
	s.closeCh.Do(func() {
		close(s.done)
		if s.closeAll != nil {
			s.closeAll()
		}
	})
	return nil
}

// push applies write backpressure; returns false when the stream closed.
func (s *taskStream) push(u channel.TaskUpdate) bool {
	select {
	case s.out <- u:
		return true
	case <-s.done:
		return false
	}
}

// SubscribeTask registers the wakeup, then reads the committed prefix under
// one watermark before waiting — replay-then-live (§8).
func (h *taskHost) SubscribeTask(ctx context.Context, sub channel.TaskSubscription) (channel.TaskStream, error) {
	p, err := h.authorize(ctx)
	if err != nil {
		return nil, err
	}
	if !validTaskID(sub.TaskID) {
		return nil, taskErr(channel.TaskErrInvalid, "task_id missing or too long")
	}
	runID := domain.RunID(sub.TaskID)
	sessionID, err := h.taskSession(ctx, p.scope(), runID, "")
	if err != nil {
		return nil, err
	}

	var startSeq domain.EventSeq
	if sub.After != "" {
		// Cursor validation is synchronous: malformed, foreign and
		// too-new cursors reject the subscription before it opens.
		max, err := h.committedMax(ctx, runID)
		if err != nil {
			return nil, err
		}
		cur, err := decodeTaskCursor(sub.After, runID, max)
		if err != nil {
			return nil, err
		}
		startSeq = cur.Seq
	}

	// Subscribe before the snapshot read so no commit lands between the
	// watermark and the live registration.
	live, cancel := h.deps.Subscribe(runID)
	stream := newTaskStream()
	stream.closeAll = cancel
	go h.runTaskStream(ctx, stream, runID, sessionID, sub.After != "", startSeq, live)
	return stream, nil
}

// committedMax returns the run's committed maximum seq (0 when empty).
func (h *taskHost) committedMax(ctx context.Context, runID domain.RunID) (domain.EventSeq, error) {
	page, err := h.deps.Journal.ReadJournalPage(ctx, storage.JournalPageQuery{
		RunID: runID, MaxEvents: 1, MaxBytes: storage.JournalPageMaxBytes,
	})
	if err != nil {
		return 0, mapAdmissionError(err)
	}
	return page.ThroughSeq, nil
}

// runTaskStream is the producer loop: snapshot-first when no cursor, then
// ordered safe updates until a terminal or delivered interrupted state.
func (h *taskHost) runTaskStream(ctx context.Context, s *taskStream, runID domain.RunID, sessionID domain.SessionID, hasCursor bool, startSeq domain.EventSeq, live <-chan domain.RunEvent) {
	defer close(s.out)
	projector := newTaskStreamProjector(runID, h.deps.SafePrompts)

	fail := func(err error) {
		if err != nil && err != io.EOF {
			select {
			case s.err <- err:
			default:
			}
		}
	}

	if hasCursor {
		// Feed the cursor's committed prefix through the projector so the
		// continuation tail reduces with the same state.
		prefix, _, err := h.journalTail(ctx, runID, 0)
		if err != nil {
			fail(mapAdmissionError(err))
			return
		}
		for _, ev := range prefix {
			if ev.Seq <= startSeq {
				projector.apply(ev)
			}
		}
		if !h.emitTaskEvents(ctx, s, projector, runID, sessionID, &startSeq) {
			return
		}
	} else {
		// Fresh subscription: emit the committed prefix as a snapshot
		// under one watermark, folded through the live projector so dedup
		// stays consistent.
		all, through, err := h.journalTail(ctx, runID, 0)
		if err != nil {
			fail(mapAdmissionError(err))
			return
		}
		for _, ev := range all {
			projector.apply(ev)
		}
		snap := projectTaskEvents(runID, sessionID, all, taskProjectionOptions{
			historyLimit:     taskHistoryCap,
			includeArtifacts: true,
			safePrompts:      h.deps.SafePrompts,
			userMessage:      h.userMessageLookup(ctx, sessionID, runID),
		})
		cursor, err := encodeTaskCursor(taskCursor{V: 1, RunID: runID, Seq: through})
		if err != nil {
			fail(err)
			return
		}
		if !s.push(channel.TaskUpdate{Cursor: cursor, Snapshot: &snap}) {
			return
		}
		startSeq = through
		if isTerminalState(snap.Status.State) || isInterruptState(snap.Status.State) {
			return
		}
	}
	lastSeq := startSeq

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case ev, ok := <-live:
			if !ok {
				// Bus closed or subscriber dropped: re-register, then
				// catch up from the journal — closure never proves a
				// terminal state (§8).
				liveCh, cancel := h.deps.Subscribe(runID)
				s.closeAll = cancel
				live = liveCh
				if !h.emitTaskEvents(ctx, s, projector, runID, sessionID, &lastSeq) {
					return
				}
				continue
			}
			if ev.Seq <= lastSeq {
				continue
			}
			lastSeq = ev.Seq
			if !h.emitTaskEvent(ctx, s, projector, runID, sessionID, ev) {
				return
			}
			if isTerminalStreamEvent(ev.Type) {
				// Drain the committed tail after the terminal event:
				// ordered evidence committed in the same window still
				// belongs to the stream before EOF.
				if !h.emitTaskEvents(ctx, s, projector, runID, sessionID, &lastSeq) {
					return
				}
				return
			}
		case <-tick.C:
			// Fallback sweep for a missed publish.
			if !h.emitTaskEvents(ctx, s, projector, runID, sessionID, &lastSeq) {
				return
			}
		case <-s.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// emitTaskEvents reads the committed tail after *lastSeq under one
// watermark and emits each contributing update; *lastSeq advances. Returns
// false when the stream must end (terminal/interrupt delivered, close, or
// error already reported).
func (h *taskHost) emitTaskEvents(ctx context.Context, s *taskStream, projector *taskStreamProjector, runID domain.RunID, sessionID domain.SessionID, lastSeq *domain.EventSeq) bool {
	events, _, err := h.journalTail(ctx, runID, *lastSeq)
	if err != nil {
		select {
		case s.err <- mapAdmissionError(err):
		default:
		}
		return false
	}
	for _, ev := range events {
		if ev.Seq <= *lastSeq {
			continue
		}
		*lastSeq = ev.Seq
		if !h.emitTaskEvent(ctx, s, projector, runID, sessionID, ev) {
			return false
		}
		if isTerminalStreamEvent(ev.Type) || isInterruptEvent(ev.Type) {
			return false
		}
	}
	return true
}

// emitTaskEvent projects one committed event into its safe stream updates
// and pushes them with ordinal cursors. Returns false to end the stream.
func (h *taskHost) emitTaskEvent(ctx context.Context, s *taskStream, projector *taskStreamProjector, runID domain.RunID, sessionID domain.SessionID, ev domain.RunEvent) bool {
	updates := projector.apply(ev)
	for i, u := range updates {
		cursor, err := encodeTaskCursor(taskCursor{V: 1, RunID: runID, Seq: ev.Seq, Ordinal: i})
		if err != nil {
			select {
			case s.err <- err:
			default:
			}
			return false
		}
		u.Cursor = cursor
		if !s.push(u) {
			return false
		}
	}
	return !isInterruptEvent(ev.Type)
}

// taskStreamProjector is the incremental safe projection for the live
// path: one shared journalview reducer accumulates assistant text across
// events, the §7 map derives state changes, and the ordinal in the cursor
// separates multiple updates inside one event.
type taskStreamProjector struct {
	runID       domain.RunID
	safePrompts bool
	reducer     *journalview.TextReducer
}

func newTaskStreamProjector(runID domain.RunID, safePrompts bool) *taskStreamProjector {
	return &taskStreamProjector{runID: runID, safePrompts: safePrompts, reducer: journalview.NewTextReducer(runID)}
}

// apply folds one committed event into ordered safe stream updates:
// assistant text segments (full-replacement artifacts) precede the status
// change the event carries.
func (p *taskStreamProjector) apply(ev domain.RunEvent) []channel.TaskUpdate {
	var st *channel.TaskStatus
	mark := func(state channel.TaskState, prompt string) {
		st = &channel.TaskStatus{State: state, UpdatedAt: time.UnixMilli(ev.CreatedAt).UTC()}
		if prompt != "" && p.safePrompts {
			st.Message = &channel.TaskMessage{
				ID: fmt.Sprintf("prompt_%s_%020d", p.runID, ev.Seq), Role: "agent",
				Parts: []channel.TaskTextPart{{Text: prompt}},
			}
		}
	}
	switch ev.Type {
	case domain.EventRunStarted:
		mark(channel.TaskStateWorking, "")
	case domain.EventUserQuestionRequired:
		var q struct {
			Prompt string `json:"prompt"`
		}
		_ = json.Unmarshal(ev.Payload, &q)
		mark(channel.TaskStateInputRequired, q.Prompt)
	case domain.EventUserQuestionAnswered, domain.EventUserQuestionCancelled, domain.EventUserQuestionExpired,
		domain.EventToolApprovalDecided:
		mark(channel.TaskStateWorking, "")
	case domain.EventToolApprovalRequired:
		mark(channel.TaskStateAuthorizationRequired, "")
	case domain.EventRunCompleted:
		mark(channel.TaskStateCompleted, "")
	case domain.EventRunFailed:
		mark(channel.TaskStateFailed, "")
	case domain.EventRunCancelled:
		mark(channel.TaskStateCanceled, "")
	}

	var updates []channel.TaskUpdate
	segments, _ := p.reducer.Apply(ev)
	for _, seg := range segments {
		updates = append(updates, channel.TaskUpdate{Artifact: &channel.TaskArtifact{
			ID: "text_" + seg.ID, Name: "message",
			Parts: []channel.TaskTextPart{{Text: seg.Text}},
		}})
	}
	if st != nil {
		updates = append(updates, channel.TaskUpdate{Status: st})
	}
	return updates
}
