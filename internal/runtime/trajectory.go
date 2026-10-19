package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

// UI-TRAJ / UI-TRAJECTORY-DEMO kernel capability: project a session's
// journal (run_events) and message store into the turn-level trajectory
// structure the console panel renders. The projection is read-only and
// bounded: text and detail fields are clipped to trajectoryTextBound, and
// only the most recent `limit` runs of the session are folded.
//
// Turn semantics: one run == one user turn (a run is started by exactly one
// user text). Within a turn, every model call is a Step; tool rows attach to
// the step whose model call produced them. Token usage comes from
// model.usage, timing from event timestamps; request payloads carry hashes
// and byte lengths only (D-010). V2 assistant text is reassembled from the
// durable bounded model.delta sequence and verified at model.completed;
// legacy v1 journals keep their authoritative completion content.
//
// OBS-04 / D4: the projection is a live Journal view. Every model call gets
// a stable request_id and an honest call_status/usage_state; run_activity
// reports queued/active/waiting/terminal state derived from durable events;
// watermarks name the last folded seq per run so subscribers can dedup and
// detect gaps; has_older_runs exposes the window boundary. Legacy status/
// completed_at/usage fields stay populated for v1 clients.
const (
	trajectoryTextBound   = 8 << 10
	defaultTrajectoryRuns = 20
	maxTrajectoryRuns     = 50

	trajectoryProjectionVersion = 2
)

// Call lifecycle vocabulary (D4).
const (
	TrajCallActive      = "active"
	TrajCallCompleted   = "completed"
	TrajCallFailed      = "failed"
	TrajCallCancelled   = "cancelled"
	TrajCallInterrupted = "interrupted"
	TrajCallLegacy      = "legacy"
)

// Usage evidence vocabulary (D4).
const (
	TrajUsageMissing  = "missing"
	TrajUsageReported = "reported"
	TrajUsagePartial  = "partial"
	TrajUsageActive   = "active"
	TrajUsageLegacy   = "legacy"
)

// Run activity vocabulary (D4).
const (
	TrajActivityQueued    = "queued"
	TrajActivityActive    = "active"
	TrajActivityWaiting   = "waiting"
	TrajActivityCompleted = "completed"
	TrajActivityFailed    = "failed"
	TrajActivityCancelled = "cancelled"

	TrajWaitApproval = "approval"
	TrajWaitQuestion = "question"
	TrajWaitChild    = "child"
	TrajWaitWorkflow = "workflow"
)

// TrajectoryTokens is per-model-call token usage (model.usage payload).
type TrajectoryTokens struct {
	Input      int `json:"input,omitempty"`
	Output     int `json:"output,omitempty"`
	Think      int `json:"think,omitempty"`
	CacheRead  int `json:"cache_read,omitempty"`
	CacheWrite int `json:"cache_write,omitempty"`
}

// TrajectoryUsageEvidence is the nullable v2 usage block on a request row.
// Reasoning/Cached keep pointer presence: nil means the provider did not
// report the bucket, never zero (D2: no synthetic usage).
type TrajectoryUsageEvidence struct {
	PromptTokens     int  `json:"prompt_tokens"`
	CompletionTokens int  `json:"completion_tokens"`
	TotalTokens      int  `json:"total_tokens"`
	ReasoningTokens  *int `json:"reasoning_tokens,omitempty"`
	CachedTokens     *int `json:"cached_tokens,omitempty"`
	CacheWriteTokens *int `json:"cache_write_tokens,omitempty"`
	// Partial marks evidence the normalizer flagged as contradictory.
	Partial bool `json:"partial,omitempty"`
}

// TrajectoryRecord is one ledger/timeline/detail row (closed kind set:
// system, user, message, tool, compacted; the UI additionally renders the
// DSH context/subtool kinds which Vivy never emits). ID is stable and
// derived from the persisted (run_id, seq, record_kind) triple.
type TrajectoryRecord struct {
	Index        int               `json:"index"`
	ID           string            `json:"id"`
	Turn         *int              `json:"turn"`
	Group        string            `json:"group"`
	Kind         string            `json:"kind"`
	Text         string            `json:"text"`
	TimeSeconds  *float64          `json:"time_seconds"`
	StartedAt    *int64            `json:"started_at"`
	Tokens       *TrajectoryTokens `json:"tokens,omitempty"`
	Result       string            `json:"result,omitempty"`
	IsError      bool              `json:"is_error,omitempty"`
	InputDetail  string            `json:"input_detail,omitempty"`
	OutputDetail string            `json:"output_detail,omitempty"`
	CallID       string            `json:"call_id,omitempty"`
	Provider     string            `json:"provider,omitempty"`
	Model        string            `json:"model,omitempty"`
	OpensTurn    bool              `json:"opens_turn,omitempty"`
}

// TrajectoryRequest is one model call (request → usage → finished).
// Status/CompletedAt/Usage are the legacy v1 compatibility fields; v2
// clients read call_status/finished_at/usage_state/usage_evidence.
type TrajectoryRequest struct {
	Number        int                      `json:"number"`
	RequestID     string                   `json:"request_id"`
	RunID         string                   `json:"run_id"`
	CallID        string                   `json:"call_id,omitempty"`
	Turn          *int                     `json:"turn"`
	Group         string                   `json:"group"`
	Status        string                   `json:"status"`
	CallStatus    string                   `json:"call_status"`
	StartedAt     int64                    `json:"started_at"`
	CompletedAt   int64                    `json:"completed_at"`
	FinishedAt    *int64                   `json:"finished_at,omitempty"`
	Provider      string                   `json:"provider,omitempty"`
	Model         string                   `json:"model,omitempty"`
	Usage         TrajectoryTokens         `json:"usage"`
	UsageState    string                   `json:"usage_state"`
	UsageEvidence *TrajectoryUsageEvidence `json:"usage_evidence"`
	Retry         int                      `json:"retry,omitempty"`
	Messages      int                      `json:"messages,omitempty"`
	PreambleBytes int                      `json:"preamble_bytes,omitempty"`
	Error         string                   `json:"error,omitempty"`
}

// TrajectoryRunActivity is one run's authoritative status plus its
// activity state and — only when a durable event identifies it — the
// kind of wait the run is parked on.
type TrajectoryRunActivity struct {
	RunID         string   `json:"run_id"`
	Status        string   `json:"status"`
	ActivityState string   `json:"activity_state"`
	WaitKind      string   `json:"wait_kind,omitempty"`
	ParentRunID   string   `json:"parent_run_id,omitempty"`
	ChildRunIDs   []string `json:"child_run_ids,omitempty"`
	WorkflowID    string   `json:"workflow_id,omitempty"`
}

// TrajectorySession is the trajectory/session projection result.
type TrajectorySession struct {
	SessionID         string                  `json:"session_id"`
	ProjectionVersion int                     `json:"projection_version"`
	Turns             int                     `json:"turns"`
	Records           []TrajectoryRecord      `json:"records"`
	Requests          []TrajectoryRequest     `json:"requests"`
	RunActivity       []TrajectoryRunActivity `json:"run_activity"`
	// Watermarks is the last folded journal seq per run, from the same read
	// prefix the snapshot used; subscribers resume/dedup against it.
	Watermarks   map[string]int64 `json:"watermarks"`
	HasOlderRuns bool             `json:"has_older_runs"`
}

// SessionTrajectory projects the session's most recent `limit` runs
// (1..50, default 20) into the trajectory structure.
func (s *Service) SessionTrajectory(ctx context.Context, sessionID domain.SessionID, limit int) (TrajectorySession, error) {
	if s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Messages == nil {
		return TrajectorySession{}, errors.New("runtime: trajectory requires journal, runs, and messages stores")
	}
	if limit <= 0 {
		limit = defaultTrajectoryRuns
	}
	if limit > maxTrajectoryRuns {
		limit = maxTrajectoryRuns
	}
	runs, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		return TrajectorySession{}, err
	}
	hasOlder := len(runs) > limit
	if hasOlder {
		runs = runs[len(runs)-limit:]
	}
	messages, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return TrajectorySession{}, err
	}
	messages, err = s.effectiveSessionMessages(ctx, sessionID, messages)
	if err != nil {
		return TrajectorySession{}, err
	}
	userText := make(map[domain.RunID]string)
	for _, message := range messages {
		if message.Role != domain.RoleUser || message.RunID == "" {
			continue
		}
		if _, ok := userText[message.RunID]; !ok {
			userText[message.RunID] = message.Content
		}
	}
	out := TrajectorySession{
		SessionID:         string(sessionID),
		ProjectionVersion: trajectoryProjectionVersion,
		Records:           []TrajectoryRecord{},
		Requests:          []TrajectoryRequest{},
		RunActivity:       []TrajectoryRunActivity{},
		Watermarks:        map[string]int64{},
		HasOlderRuns:      hasOlder,
	}
	for i, run := range runs {
		turn := i + 1
		var userRow *TrajectoryRecord
		if text, ok := userText[run.ID]; ok {
			userRow = &TrajectoryRecord{ID: string(run.ID) + ":user", Turn: &turn, Group: "User", Kind: "user",
				Text: boundTrajectoryText(text), OpensTurn: true}
		}
		records, requests, activity, watermark, failure := s.projectRunTrajectory(ctx, run, turn, i == 0)
		records = insertTrajectoryUserRow(records, userRow)
		out.Records = append(out.Records, records...)
		out.Requests = append(out.Requests, requests...)
		out.RunActivity = append(out.RunActivity, activity)
		out.Watermarks[string(run.ID)] = watermark
		if failure != "" {
			out.Records = append(out.Records, TrajectoryRecord{ID: string(run.ID) + ":failed", Turn: &turn, Group: "Run",
				Kind: "message", Text: boundTrajectoryText(failure), IsError: true})
		}
	}
	for i := range out.Records {
		out.Records[i].Index = i + 1
		if out.Records[i].ID == "" {
			out.Records[i].ID = fmt.Sprintf("rec-%d", i+1)
		}
	}
	for i := range out.Requests {
		out.Requests[i].Number = i + 1
	}
	out.Turns = len(runs)
	return out, nil
}

// insertTrajectoryUserRow places the user row after the session-section
// records (Session group) so turn 1 opens with the user message.
func insertTrajectoryUserRow(records []TrajectoryRecord, userRow *TrajectoryRecord) []TrajectoryRecord {
	if userRow == nil {
		return records
	}
	cut := 0
	for cut < len(records) && records[cut].Group == "Session" {
		cut++
	}
	out := make([]TrajectoryRecord, 0, len(records)+1)
	out = append(out, records[:cut]...)
	out = append(out, *userRow)
	return append(out, records[cut:]...)
}

// trajectoryCall is the fold state of one model-call attempt.
type trajectoryCall struct {
	request     *TrajectoryRequest
	evidence    *TrajectoryUsageEvidence
	partial     bool
	legacy      bool // no call_id: pre-OBS-02 journal row
	legacyUsage *TrajectoryTokens
	sawComplete bool // legacy model.completed observed
	finished    bool
}

// projectRunTrajectory folds one run's journal into records, requests, run
// activity and the folded watermark. Only the first projected run emits the
// Session-section system row; later run.started events still update
// provider/model provenance. It returns a non-empty failure string when the
// run ended in failure.
func (s *Service) projectRunTrajectory(ctx context.Context, run domain.Run, turn int, firstRun bool) ([]TrajectoryRecord, []TrajectoryRequest, TrajectoryRunActivity, int64, string) {
	activity := TrajectoryRunActivity{
		RunID:       string(run.ID),
		Status:      string(run.Status),
		ParentRunID: string(run.ParentID),
	}
	if run.Kind == domain.RunKindWorkflow {
		activity.WorkflowID = string(run.ID)
	}
	if children, err := s.deps.Runs.ListChildRuns(ctx, run.ID); err == nil && len(children) > 0 {
		activity.ChildRunIDs = make([]string, 0, len(children))
		for _, child := range children {
			activity.ChildRunIDs = append(activity.ChildRunIDs, string(child.ID))
		}
	}
	it, err := s.deps.Journal.Replay(ctx, run.ID, 0)
	if err != nil {
		activity.ActivityState = trajectoryActivityState(run, "")
		return []TrajectoryRecord{trajectoryFoldErrorRecord(turn, err)}, nil, activity, 0, ""
	}
	defer func() { _ = it.Close() }()

	var (
		records   []TrajectoryRecord
		requests  []TrajectoryRequest
		calls     = map[string]*trajectoryCall{}
		callOrder []string
		open      *trajectoryCall // most recently started, unfinished call
		lastCall  *trajectoryCall // most recently started, finished or not
		provider  string
		modelID   string
		step      int
		failure   string
		toolArgs  = map[string]string{}
		toolT0    = map[string]int64{}
		deltas    strings.Builder
		watermark int64
		waitKind  string
		pendAppr  = map[string]bool{}
		pendQuest = map[string]bool{}
		childWait bool
		wfWait    bool
	)
	appendRecord := func(record TrajectoryRecord) {
		if record.Turn == nil {
			t := turn
			record.Turn = &t
		}
		records = append(records, record)
	}
	startCall := func(key string, req *TrajectoryRequest) *trajectoryCall {
		call := &trajectoryCall{request: req}
		calls[key] = call
		callOrder = append(callOrder, key)
		return call
	}
	// interruptOpen marks a still-open attempt interrupted: a new request was
	// journaled before the call closed. It never terminalizes the run.
	interruptOpen := func(at int64) {
		if open == nil {
			return
		}
		finishTrajectoryCall(open, TrajCallInterrupted, at, "interrupted by a new model request")
		requests = append(requests, *open.request)
		open = nil
	}
	for it.Next() {
		event := it.Value().Event
		if int64(event.Seq) > watermark {
			watermark = int64(event.Seq)
		}
		switch event.Type {
		case domain.EventRunStarted:
			var payload payloadRunStarted
			if json.Unmarshal(event.Payload, &payload) == nil {
				provider, modelID = payload.Provider, payload.Model
				if firstRun {
					// Session-section row: turn stays null like the DSH view.
					records = append(records, TrajectoryRecord{
						ID:    trajectoryRecordID(run.ID, event.Seq, "system"),
						Group: "Session", Kind: "system",
						Text: fmt.Sprintf("Run start · %s / %s · mode %s", payload.Provider, payload.Model, payload.Mode)})
				}
			}
		case domain.EventModelRequest:
			var v3 payloadModelRequestV3
			_ = json.Unmarshal(event.Payload, &v3)
			if v3.Source == modelCallSourceMaintenance {
				startCall("call/"+v3.CallID, &TrajectoryRequest{RunID: string(run.ID), Turn: &turn, Group: "Maintenance", Status: "active", CallStatus: TrajCallActive, StartedAt: event.CreatedAt, UsageState: TrajUsageActive, CallID: v3.CallID, RequestID: trajectoryRequestID(run.ID, v3.CallID), Provider: v3.Provider, Model: v3.Model, Messages: len(v3.Messages), PreambleBytes: v3.PreambleBytes})
				continue
			}
			interruptOpen(event.CreatedAt)
			step++
			req := &TrajectoryRequest{
				RunID:      string(run.ID),
				Turn:       &turn,
				Group:      fmt.Sprintf("Step %d", step),
				Status:     "active",
				CallStatus: TrajCallActive,
				StartedAt:  event.CreatedAt,
				UsageState: TrajUsageActive,
				Messages:   len(v3.Messages), PreambleBytes: v3.PreambleBytes,
			}
			var key string
			if v3.CallID != "" {
				req.CallID = v3.CallID
				req.RequestID = trajectoryRequestID(run.ID, v3.CallID)
				req.Provider = firstNonEmpty(v3.Provider, provider)
				req.Model = firstNonEmpty(v3.Model, modelID)
				key = "call/" + v3.CallID
			} else {
				var v1 payloadModelRequest
				_ = json.Unmarshal(event.Payload, &v1)
				req.RequestID = fmt.Sprintf("%s:req-%d", run.ID, event.Seq)
				req.Provider = provider
				req.Model = modelID
				req.CallStatus = TrajCallLegacy
				req.UsageState = TrajUsageLegacy
				key = fmt.Sprintf("req/%d", event.Seq)
			}
			open = startCall(key, req)
			lastCall = open
			open.legacy = v3.CallID == ""
			deltas.Reset()
		case domain.EventModelDelta:
			var payload payloadModelDelta
			if json.Unmarshal(event.Payload, &payload) == nil {
				deltas.WriteString(payload.Delta)
			}
		case domain.EventModelUsage:
			var v2 payloadModelUsageV2
			_ = json.Unmarshal(event.Payload, &v2)
			if v2.CallID != "" {
				call := calls["call/"+v2.CallID]
				if call == nil {
					// Orphan sample: create an untracked attempt so evidence
					// still lands on a stable row.
					req := &TrajectoryRequest{
						RunID:      string(run.ID),
						Turn:       &turn,
						Group:      fmt.Sprintf("Step %d", step+1),
						Status:     "active",
						CallStatus: TrajCallActive,
						StartedAt:  event.CreatedAt,
						UsageState: TrajUsageActive,
						RequestID:  trajectoryRequestID(run.ID, v2.CallID),
						CallID:     v2.CallID,
						Provider:   v2.Provider,
						Model:      v2.Model,
					}
					call = startCall("call/"+v2.CallID, req)
					lastCall = call
				}
				call.evidence = &TrajectoryUsageEvidence{
					PromptTokens: v2.PromptTokens, CompletionTokens: v2.CompletionTokens,
					TotalTokens: v2.TotalTokens, ReasoningTokens: v2.ReasoningTokens,
					CachedTokens:     v2.CachedTokens,
					CacheWriteTokens: v2.CacheWriteTokens,
				}
				if v2.NormalizationPartial != nil && *v2.NormalizationPartial {
					call.partial = true
				}
			} else {
				var payload payloadModelUsage
				if json.Unmarshal(event.Payload, &payload) == nil && open != nil {
					open.legacyUsage = &TrajectoryTokens{Input: payload.PromptTokens, Output: payload.CompletionTokens,
						Think: payload.ReasoningTokens, CacheRead: payload.CachedTokens}
				}
			}
		case domain.EventProviderRetry:
			if open != nil {
				open.request.Retry++
			}
		case domain.EventModelCallFinished:
			var payload payloadModelCallFinished
			if json.Unmarshal(event.Payload, &payload) != nil {
				continue
			}
			call := calls["call/"+payload.CallID]
			if call == nil {
				// Orphan finish: keep the row rather than dropping evidence.
				req := &TrajectoryRequest{
					RunID:     string(run.ID),
					Turn:      &turn,
					Group:     fmt.Sprintf("Step %d", step+1),
					StartedAt: event.CreatedAt,
					RequestID: trajectoryRequestID(run.ID, payload.CallID),
					CallID:    payload.CallID,
					Provider:  payload.Provider,
					Model:     payload.Model,
				}
				call = startCall("call/"+payload.CallID, req)
				lastCall = call
			}
			status := payload.Status
			switch status {
			case "completed":
				status = TrajCallCompleted
			case "cancelled":
				status = TrajCallCancelled
			default:
				status = TrajCallFailed
			}
			if payload.Usage != nil && call.evidence == nil {
				call.evidence = &TrajectoryUsageEvidence{
					PromptTokens: payload.Usage.PromptTokens, CompletionTokens: payload.Usage.CompletionTokens,
					TotalTokens: payload.Usage.TotalTokens, ReasoningTokens: payload.Usage.ReasoningTokens,
					CachedTokens:     payload.Usage.CachedTokens,
					CacheWriteTokens: payload.Usage.CacheWriteTokens,
				}
				if payload.Usage.NormalizationPartial != nil && *payload.Usage.NormalizationPartial {
					call.partial = true
				}
			}
			message := ""
			if payload.Error != nil {
				message = payload.Error.Message
			}
			finishTrajectoryCall(call, status, event.CreatedAt, message)
			if call == open {
				open = nil
			}
			requests = append(requests, *call.request)
		case domain.EventModelCompleted:
			content, projectionErr := completedProjectionContent(event, deltas.String())
			deltas.Reset()
			if projectionErr != nil {
				appendRecord(trajectoryFoldErrorRecord(turn, projectionErr))
				interruptOpen(event.CreatedAt)
				continue
			}
			// call.finished may precede model.completed (OBS-02 lifecycle
			// closes at stream End); the message still belongs to the latest
			// call's step, finished or not.
			if lastCall != nil {
				lastCall.sawComplete = true
				group := lastCall.request.Group
				startedAt := lastCall.request.StartedAt
				appendRecord(TrajectoryRecord{
					ID:    trajectoryRecordID(run.ID, event.Seq, "message"),
					Group: group, Kind: "message", Text: boundTrajectoryText(content),
					TimeSeconds: trajectorySeconds(startedAt, event.CreatedAt), StartedAt: &startedAt,
					Tokens: lastCall.legacyUsage, Provider: provider, Model: modelID})
				if lastCall.legacy && !lastCall.finished {
					finishTrajectoryCall(lastCall, TrajCallCompleted, event.CreatedAt, "")
					requests = append(requests, *lastCall.request)
					if open == lastCall {
						open = nil
					}
				}
			}
		case domain.EventToolRequested:
			if deltas.Len() > 0 && lastCall != nil {
				startedAt := lastCall.request.StartedAt
				appendRecord(TrajectoryRecord{
					ID:    trajectoryRecordID(run.ID, event.Seq, "message"),
					Group: lastCall.request.Group, Kind: "message", Text: boundTrajectoryText(deltas.String()),
					TimeSeconds: trajectorySeconds(startedAt, event.CreatedAt), StartedAt: &startedAt,
					Provider: provider, Model: modelID})
			}
			deltas.Reset()
			var payload payloadToolRequested
			if json.Unmarshal(event.Payload, &payload) == nil {
				toolArgs[payload.ToolCallID] = boundTrajectoryJSON(payload.Args)
			}
		case domain.EventToolStarted:
			var payload payloadToolStarted
			if json.Unmarshal(event.Payload, &payload) == nil {
				toolT0[payload.ToolCallID] = event.CreatedAt
			}
		case domain.EventToolFinished:
			var payload payloadToolFinished
			if json.Unmarshal(event.Payload, &payload) != nil {
				continue
			}
			startedAt := event.CreatedAt
			if t0, ok := toolT0[payload.ToolCallID]; ok {
				startedAt = t0
			}
			group := "Step 1"
			if lastCall != nil {
				group = lastCall.request.Group
			} else if step > 0 {
				group = fmt.Sprintf("Step %d", step)
			}
			record := TrajectoryRecord{
				ID:    trajectoryRecordID(run.ID, event.Seq, "tool"),
				Group: group, Kind: "tool", Text: payload.ToolName,
				CallID: payload.ToolCallID, TimeSeconds: trajectorySeconds(startedAt, event.CreatedAt),
				StartedAt: &startedAt, IsError: payload.Error != ""}
			if payload.Error != "" {
				record.OutputDetail = boundTrajectoryText(payload.Error)
			} else {
				record.Result = boundTrajectoryText(payload.Result)
				record.OutputDetail = boundTrajectoryText(payload.Result)
			}
			if args, ok := toolArgs[payload.ToolCallID]; ok {
				record.InputDetail = args
			}
			appendRecord(record)
			delete(toolArgs, payload.ToolCallID)
			delete(toolT0, payload.ToolCallID)
		case domain.EventContextCompacted:
			var payload payloadContextCompacted
			if json.Unmarshal(event.Payload, &payload) == nil {
				// Session-section row: turn stays null like the DSH view.
				records = append(records, TrajectoryRecord{
					ID:    trajectoryRecordID(run.ID, event.Seq, "compacted"),
					Group: "Compaction", Kind: "compacted",
					Text: fmt.Sprintf("%s · %d → %d tokens", payload.Mode, payload.BeforeTokens, payload.AfterTokens)})
			}
		case domain.EventToolApprovalRequired:
			var payload payloadToolApprovalRequired
			if json.Unmarshal(event.Payload, &payload) == nil {
				pendAppr[payload.ApprovalID] = true
			}
		case domain.EventToolApprovalDecided, domain.EventToolProposalStale:
			var payload payloadApprovalDecided
			if json.Unmarshal(event.Payload, &payload) == nil {
				delete(pendAppr, payload.ApprovalID)
			}
			var stale payloadProposalStale
			if json.Unmarshal(event.Payload, &stale) == nil {
				delete(pendAppr, stale.ApprovalID)
			}
		case domain.EventToolApprovalCancelled:
			var payload payloadApprovalCancelled
			if json.Unmarshal(event.Payload, &payload) == nil {
				delete(pendAppr, payload.ApprovalID)
			}
		case domain.EventUserQuestionRequired:
			var payload payloadUserQuestionRequired
			if json.Unmarshal(event.Payload, &payload) == nil {
				pendQuest[payload.QuestionID] = true
			}
		case domain.EventUserQuestionAnswered:
			var payload payloadUserQuestionAnswered
			if json.Unmarshal(event.Payload, &payload) == nil {
				delete(pendQuest, payload.QuestionID)
			}
		case domain.EventUserQuestionCancelled:
			var payload payloadQuestionCancelled
			if json.Unmarshal(event.Payload, &payload) == nil {
				delete(pendQuest, payload.QuestionID)
			}
		case domain.EventToolApprovalExpired, domain.EventUserQuestionExpired:
			var payload payloadInteractionExpired
			if json.Unmarshal(event.Payload, &payload) == nil {
				delete(pendAppr, payload.ReviewID)
				delete(pendQuest, payload.ReviewID)
			}
		case domain.EventChildSuspended:
			childWait = true
		case domain.EventChildResumed, domain.EventChildCompleted, domain.EventChildFailed, domain.EventChildCancelled:
			childWait = false
		case domain.EventWorkflowWaiting:
			wfWait = true
		case domain.EventWorkflowResumed:
			wfWait = false
		case domain.EventRunFailed:
			var payload payloadRunFailed
			if json.Unmarshal(event.Payload, &payload) == nil {
				failure = payload.Message
			}
		}
	}
	if err := it.Err(); err != nil {
		appendRecord(trajectoryFoldErrorRecord(turn, err))
	}
	runLive := run.Status == domain.RunActive || run.Status == domain.RunQueued || run.Status == domain.RunAccepted
	for _, key := range callOrder {
		call := calls[key]
		if call.finished {
			continue
		}
		if runLive {
			// Still running: the call is active, never a fake completion.
			call.request.CallStatus = TrajCallActive
			call.request.Status = "active"
			applyUsageState(call)
		} else {
			finishTrajectoryCall(call, TrajCallInterrupted, lastEventTime(records), failureOrInterrupted(failure))
		}
		requests = append(requests, *call.request)
	}
	if len(pendAppr) > 0 {
		waitKind = TrajWaitApproval
	} else if len(pendQuest) > 0 {
		waitKind = TrajWaitQuestion
	} else if childWait {
		waitKind = TrajWaitChild
	} else if wfWait {
		waitKind = TrajWaitWorkflow
	}
	activity.ActivityState = trajectoryActivityState(run, waitKind)
	if waitKind != "" && activity.ActivityState == TrajActivityWaiting {
		activity.WaitKind = waitKind
	}
	return records, requests, activity, watermark, failure
}

// finishTrajectoryCall writes the terminal lifecycle fields on an attempt
// and mirrors them into the legacy status/completed_at/usage fields.
func finishTrajectoryCall(call *trajectoryCall, status string, at int64, message string) {
	call.finished = true
	call.request.CallStatus = status
	call.request.FinishedAt = &at
	call.request.CompletedAt = at
	call.request.Error = message
	switch status {
	case TrajCallCompleted:
		call.request.Status = "complete"
	case TrajCallCancelled:
		call.request.Status = "cancelled"
	case TrajCallInterrupted:
		call.request.Status = "error"
	default:
		call.request.Status = "error"
	}
	applyUsageState(call)
	if call.legacyUsage != nil {
		call.request.Usage = *call.legacyUsage
	} else if call.evidence != nil {
		call.request.Usage = TrajectoryTokens{
			Input: call.evidence.PromptTokens, Output: call.evidence.CompletionTokens,
		}
		if call.evidence.ReasoningTokens != nil {
			call.request.Usage.Think = *call.evidence.ReasoningTokens
		}
		if call.evidence.CachedTokens != nil {
			call.request.Usage.CacheRead = *call.evidence.CachedTokens
		}
		if call.evidence.CacheWriteTokens != nil {
			call.request.Usage.CacheWrite = *call.evidence.CacheWriteTokens
		}
	}
}

// applyUsageState derives usage_state from the attempt's evidence so nil
// usage never reads as reported zero.
func applyUsageState(call *trajectoryCall) {
	if call.legacy {
		call.request.UsageState = TrajUsageLegacy
		if call.legacyUsage != nil {
			call.request.UsageEvidence = &TrajectoryUsageEvidence{
				PromptTokens:     call.legacyUsage.Input,
				CompletionTokens: call.legacyUsage.Output,
				TotalTokens:      call.legacyUsage.Input + call.legacyUsage.Output,
			}
			if call.legacyUsage.Think != 0 {
				v := call.legacyUsage.Think
				call.request.UsageEvidence.ReasoningTokens = &v
			}
			if call.legacyUsage.CacheRead != 0 {
				v := call.legacyUsage.CacheRead
				call.request.UsageEvidence.CachedTokens = &v
			}
		}
		return
	}
	if !call.finished && call.request.CallStatus == TrajCallActive {
		call.request.UsageState = TrajUsageActive
	} else if call.evidence == nil {
		call.request.UsageState = TrajUsageMissing
	} else if call.partial {
		call.request.UsageState = TrajUsagePartial
	} else {
		call.request.UsageState = TrajUsageReported
	}
	call.request.UsageEvidence = call.evidence
}

func trajectoryActivityState(run domain.Run, waitKind string) string {
	switch run.Status {
	case domain.RunCompleted:
		return TrajActivityCompleted
	case domain.RunFailed:
		return TrajActivityFailed
	case domain.RunCancelled:
		return TrajActivityCancelled
	case domain.RunQueued, domain.RunAccepted:
		return TrajActivityQueued
	default:
		if waitKind != "" {
			return TrajActivityWaiting
		}
		return TrajActivityActive
	}
}

func trajectoryRecordID(runID domain.RunID, seq domain.EventSeq, kind string) string {
	return fmt.Sprintf("%s:%d:%s", runID, seq, kind)
}

func trajectoryRequestID(runID domain.RunID, callID string) string {
	return fmt.Sprintf("%s:%s", runID, callID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func failureOrInterrupted(failure string) string {
	if failure != "" {
		return failure
	}
	return "run ended before the model call completed"
}

func lastEventTime(records []TrajectoryRecord) int64 {
	var latest int64
	for _, record := range records {
		if record.StartedAt != nil && *record.StartedAt > latest {
			latest = *record.StartedAt
		}
	}
	return latest
}

func trajectoryFoldErrorRecord(turn int, err error) TrajectoryRecord {
	return TrajectoryRecord{Turn: &turn, Group: "Run", Kind: "message",
		Text: fmt.Sprintf("trajectory replay failed: %v", err), IsError: true}
}

func trajectorySeconds(startedAt, completedAt int64) *float64 {
	if startedAt <= 0 || completedAt < startedAt {
		return nil
	}
	seconds := float64(completedAt-startedAt) / 1000.0
	return &seconds
}

func boundTrajectoryText(text string) string {
	if len(text) <= trajectoryTextBound {
		return text
	}
	cut := safeUTF8Prefix(text, trajectoryTextBound)
	return text[:cut] + "\n[truncated]"
}

// boundTrajectoryJSON renders tool arguments as bounded JSON text.
func boundTrajectoryJSON(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return boundTrajectoryText(string(encoded))
}
