package channelhost

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/journalview"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/channel"
)

// taskProjection is the §7 event/state map applied to committed journal
// events only. History carries the task's external user messages plus safe
// assistant text and approved interaction prompts — never tool arguments,
// tool results, reasoning, model prompts, paths, credentials or checkpoint
// details.
type taskProjectionOptions struct {
	historyLimit     int
	includeArtifacts bool
	safePrompts      bool
	userMessage      string // the admitted caller message text, if resolvable
}

// projectTaskEvents reduces one run's committed event prefix (seq-ordered)
// to its safe task snapshot.
func projectTaskEvents(runID domain.RunID, sessionID domain.SessionID, events []domain.RunEvent, opts taskProjectionOptions) channel.TaskSnapshot {
	snap := channel.TaskSnapshot{
		Ref:    channel.TaskRef{TaskID: string(runID), ContextID: string(sessionID)},
		Status: channel.TaskStatus{State: channel.TaskStateSubmitted},
	}
	reducer := journalview.NewTextReducer(runID)
	pendingPrompt := "" // safe text of the currently pending input request
	agentText := ""     // latest completed assistant segment
	var history []channel.TaskMessage
	var artifacts []channel.TaskArtifact

	flushSegment := func(ev domain.RunEvent, text string) {
		if text == "" {
			return
		}
		agentText = text
		id := journalview.TextMessageID(runID, ev.Seq, "0")
		if opts.includeArtifacts {
			artifacts = append(artifacts, channel.TaskArtifact{
				ID: "text_" + id, Name: "message",
				Parts: []channel.TaskTextPart{{Text: text}},
			})
		}
		history = append(history, channel.TaskMessage{
			ID: id, Role: "agent", Parts: []channel.TaskTextPart{{Text: text}},
		})
	}
	mark := func(ev domain.RunEvent, state channel.TaskState) {
		snap.Status.State = state
		snap.Status.UpdatedAt = time.UnixMilli(ev.CreatedAt).UTC()
	}

	for _, ev := range events {
		switch ev.Type {
		case domain.EventRunStarted:
			mark(ev, channel.TaskStateWorking)
			if _, err := reducer.Apply(ev); err != nil {
				mark(ev, channel.TaskStateFailed)
			}
		case domain.EventUserQuestionRequired:
			mark(ev, channel.TaskStateInputRequired)
			pendingPrompt = ""
			if opts.safePrompts {
				var p struct {
					Prompt string `json:"prompt"`
				}
				if err := json.Unmarshal(ev.Payload, &p); err == nil {
					pendingPrompt = p.Prompt
				}
			}
		case domain.EventUserQuestionAnswered, domain.EventUserQuestionCancelled, domain.EventUserQuestionExpired:
			// Answered/settled precedence: the pending-input state clears the
			// moment its resolution commits, regardless of interleavings.
			if snap.Status.State == channel.TaskStateInputRequired {
				mark(ev, channel.TaskStateWorking)
			}
			pendingPrompt = ""
		case domain.EventToolApprovalRequired:
			// The approval row never forwards its prompt: args, tool names
			// and policy details stay out of the remote view.
			mark(ev, channel.TaskStateAuthorizationRequired)
			pendingPrompt = ""
		case domain.EventToolApprovalDecided:
			if snap.Status.State == channel.TaskStateAuthorizationRequired {
				mark(ev, channel.TaskStateWorking)
			}
			pendingPrompt = ""
		case domain.EventRunCompleted:
			mark(ev, channel.TaskStateCompleted)
		case domain.EventRunFailed:
			mark(ev, channel.TaskStateFailed)
		case domain.EventRunCancelled:
			mark(ev, channel.TaskStateCanceled)
		}
		segments, err := reducer.Apply(ev)
		if err == nil {
			for _, seg := range segments {
				flushSegment(ev, seg.Text)
			}
		}
	}
	if pendingPrompt != "" && (snap.Status.State == channel.TaskStateInputRequired || snap.Status.State == channel.TaskStateAuthorizationRequired) {
		snap.Status.Message = &channel.TaskMessage{
			ID: "prompt_" + snap.Ref.TaskID, Role: "agent",
			Parts: []channel.TaskTextPart{{Text: pendingPrompt}},
		}
	} else if agentText != "" && snap.Status.State == channel.TaskStateCompleted {
		snap.Status.Message = &channel.TaskMessage{
			ID: "final_" + snap.Ref.TaskID, Role: "agent",
			Parts: []channel.TaskTextPart{{Text: agentText}},
		}
	} else if snap.Status.State == channel.TaskStateFailed {
		snap.Status.Message = &channel.TaskMessage{
			ID: "failed_" + snap.Ref.TaskID, Role: "agent",
			Parts: []channel.TaskTextPart{{Text: "the task failed"}},
		}
	}
	if opts.userMessage != "" {
		history = append([]channel.TaskMessage{{
			ID: "user_" + snap.Ref.TaskID, Role: "user",
			Parts: []channel.TaskTextPart{{Text: opts.userMessage}},
		}}, history...)
	}
	if opts.historyLimit >= 0 && len(history) > opts.historyLimit {
		history = history[len(history)-opts.historyLimit:]
	}
	snap.History = history
	snap.Artifacts = artifacts
	return snap
}

// projectTask reads the run's committed journal through one watermark and
// projects the safe snapshot.
func (h *taskHost) projectTask(ctx context.Context, scope domain.ChannelTaskScope, runID domain.RunID, sessionID domain.SessionID, historyLimit int, includeArtifacts bool) (channel.TaskSnapshot, error) {
	events, through, err := h.journalTail(ctx, runID, 0)
	if err != nil {
		return channel.TaskSnapshot{}, mapAdmissionError(err)
	}
	if len(events) == 0 {
		// A receipt-owned run with no committed journal events is
		// corruption: fail loudly, never invent a status.
		return channel.TaskSnapshot{}, taskErr(channel.TaskErrCorrupt, "task has no committed events")
	}
	snap := projectTaskEvents(runID, sessionID, events, taskProjectionOptions{
		historyLimit:     historyLimit,
		includeArtifacts: includeArtifacts,
		safePrompts:      h.deps.SafePrompts,
		userMessage:      h.userMessageLookup(ctx, sessionID, runID),
	})
	snap.Revision = taskRevision(runID, through)
	return snap, nil
}

// taskRevision is the Host-issued projection watermark used for internal
// convergence (never a wire extension): the fixed committed ceiling the
// snapshot was read under.
func taskRevision(runID domain.RunID, through domain.EventSeq) string {
	return fmt.Sprintf("h%d:%s", through, runID)
}

// userMessageLookup resolves the admitted caller text of one task run: its
// user message row (ChannelMessageID carries the caller's dedup key, RunID
// binds it to this task). Failure to resolve drops the user row from
// history; it never blocks the projection.
func (h *taskHost) userMessageLookup(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) string {
	msgs, err := h.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return ""
	}
	for _, m := range msgs {
		if m.RunID == runID && m.Role == domain.RoleUser {
			return m.Content
		}
	}
	return ""
}

// journalTailBudget is journalTail for scans shared across a scope: the
// event budget is charged per event across every task of the call so one
// List cannot read unbounded journal (§7.3).
func (h *taskHost) journalTailBudget(ctx context.Context, runID domain.RunID, afterSeq domain.EventSeq, budget *int) ([]domain.RunEvent, domain.EventSeq, error) {
	var (
		out     []domain.RunEvent
		through domain.EventSeq
		first   = true
	)
	for {
		page, err := h.deps.Journal.ReadJournalPage(ctx, storage.JournalPageQuery{
			RunID: runID, AfterSeq: afterSeq, ThroughSeq: through,
			MaxEvents: storage.JournalPageMaxEvents, MaxBytes: storage.JournalPageMaxBytes,
		})
		if err != nil {
			return nil, 0, mapAdmissionError(err)
		}
		if first {
			through = page.ThroughSeq
			first = false
		}
		out = append(out, page.Events...)
		*budget -= len(page.Events)
		if *budget < 0 {
			return nil, 0, taskErr(channel.TaskErrLimit, "task list exceeds the scan budget")
		}
		if !page.HasMore {
			return out, through, nil
		}
		if len(page.Events) == 0 {
			return nil, 0, taskErr(channel.TaskErrLimit, "non-progress journal page")
		}
		afterSeq = page.Events[len(page.Events)-1].Seq
	}
}

// pendingQuestionID returns the run's currently pending user question, or
// "" when none is open. It reads committed events only (§7: a question is
// answerable from user.question_required until its resolution commits).
func (h *taskHost) pendingQuestionID(ctx context.Context, runID domain.RunID) (string, error) {
	events, _, err := h.journalTail(ctx, runID, 0)
	if err != nil {
		return "", mapAdmissionError(err)
	}
	pending := ""
	for _, ev := range events {
		switch ev.Type {
		case domain.EventUserQuestionRequired:
			var p struct {
				QuestionID string `json:"question_id"`
			}
			if err := json.Unmarshal(ev.Payload, &p); err == nil {
				pending = p.QuestionID
			}
		case domain.EventUserQuestionAnswered, domain.EventUserQuestionCancelled, domain.EventUserQuestionExpired:
			var p struct {
				QuestionID string `json:"question_id"`
			}
			if err := json.Unmarshal(ev.Payload, &p); err != nil || p.QuestionID == pending {
				pending = ""
			}
		case domain.EventRunCompleted, domain.EventRunFailed, domain.EventRunCancelled:
			pending = ""
		}
	}
	return pending, nil
}

func sortTaskSnapshots(snaps []channel.TaskSnapshot) {
	sort.SliceStable(snaps, func(i, j int) bool {
		a, b := snaps[i].Status.UpdatedAt, snaps[j].Status.UpdatedAt
		if a.Equal(b) {
			return snaps[i].Ref.TaskID > snaps[j].Ref.TaskID
		}
		return a.After(b)
	})
}

func unixSeconds() int64 { return time.Now().Unix() }
