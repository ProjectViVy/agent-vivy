package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
// and byte lengths only (D-010), so no transcript content is re-assembled
// here beyond what the durable stores already hold (user text, model
// completed content, tool results).
const (
	trajectoryTextBound   = 8 << 10
	defaultTrajectoryRuns = 20
	maxTrajectoryRuns     = 50
)

// TrajectoryTokens is per-model-call token usage (model.usage payload).
type TrajectoryTokens struct {
	Input      int `json:"input,omitempty"`
	Output     int `json:"output,omitempty"`
	Think      int `json:"think,omitempty"`
	CacheRead  int `json:"cache_read,omitempty"`
	CacheWrite int `json:"cache_write,omitempty"`
}

// TrajectoryRecord is one ledger/timeline/detail row (closed kind set:
// system, user, message, tool, compacted; the UI additionally renders the
// DSH context/subtool kinds which Vivy never emits).
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

// TrajectoryRequest is one model call (request → usage → completed).
type TrajectoryRequest struct {
	Number        int              `json:"number"`
	Turn          *int             `json:"turn"`
	Group         string           `json:"group"`
	Status        string           `json:"status"`
	StartedAt     int64            `json:"started_at"`
	CompletedAt   int64            `json:"completed_at"`
	Provider      string           `json:"provider,omitempty"`
	Model         string           `json:"model,omitempty"`
	Usage         TrajectoryTokens `json:"usage"`
	Retry         int              `json:"retry,omitempty"`
	Messages      int              `json:"messages,omitempty"`
	PreambleBytes int              `json:"preamble_bytes,omitempty"`
	Error         string           `json:"error,omitempty"`
}

// TrajectorySession is the trajectory/session projection result.
type TrajectorySession struct {
	SessionID string              `json:"session_id"`
	Turns     int                 `json:"turns"`
	Records   []TrajectoryRecord  `json:"records"`
	Requests  []TrajectoryRequest `json:"requests"`
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
	if len(runs) > limit {
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
	out := TrajectorySession{SessionID: string(sessionID), Records: []TrajectoryRecord{}, Requests: []TrajectoryRequest{}}
	for i, run := range runs {
		turn := i + 1
		var userRow *TrajectoryRecord
		if text, ok := userText[run.ID]; ok {
			userRow = &TrajectoryRecord{Turn: &turn, Group: "User", Kind: "user",
				Text: boundTrajectoryText(text), OpensTurn: true}
		}
		records, requests, failure := s.projectRunTrajectory(ctx, run, turn, len(out.Requests), i == 0)
		records = insertTrajectoryUserRow(records, userRow)
		out.Records = append(out.Records, records...)
		out.Requests = append(out.Requests, requests...)
		if failure != "" {
			out.Records = append(out.Records, TrajectoryRecord{Turn: &turn, Group: "Run",
				Kind: "message", Text: boundTrajectoryText(failure), IsError: true})
		}
	}
	for i := range out.Records {
		out.Records[i].Index = i + 1
		out.Records[i].ID = fmt.Sprintf("rec-%d", i+1)
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

// projectRunTrajectory folds one run's journal into records and requests.
// Only the first projected run emits the Session-section system row; later
// run.started events still update provider/model provenance. It returns a
// non-empty failure string when the run ended in failure.
func (s *Service) projectRunTrajectory(ctx context.Context, run domain.Run, turn, requestOffset int, firstRun bool) ([]TrajectoryRecord, []TrajectoryRequest, string) {
	it, err := s.deps.Journal.Replay(ctx, run.ID, 0)
	if err != nil {
		return []TrajectoryRecord{trajectoryFoldErrorRecord(turn, err)}, nil, ""
	}
	defer func() { _ = it.Close() }()

	var (
		records  []TrajectoryRecord
		requests []TrajectoryRequest
		open     *TrajectoryRequest
		usage    *TrajectoryTokens
		provider string
		modelID  string
		step     int
		failure  string
		toolArgs = map[string]string{}
		toolT0   = map[string]int64{}
	)
	appendRecord := func(record TrajectoryRecord) {
		if record.Turn == nil {
			turn := turn
			record.Turn = &turn
		}
		records = append(records, record)
	}
	closeOpen := func(completedAt int64, status, message string) {
		if open == nil {
			return
		}
		open.CompletedAt = completedAt
		open.Status = status
		open.Error = message
		if usage != nil {
			open.Usage = *usage
		}
		requests = append(requests, *open)
		open = nil
		usage = nil
	}
	for it.Next() {
		event := it.Value().Event
		switch event.Type {
		case domain.EventRunStarted:
			var payload payloadRunStarted
			if json.Unmarshal(event.Payload, &payload) == nil {
				provider, modelID = payload.Provider, payload.Model
				if firstRun {
					// Session-section row: turn stays null like the DSH view.
					records = append(records, TrajectoryRecord{Group: "Session", Kind: "system",
						Text: fmt.Sprintf("Run start · %s / %s · mode %s", payload.Provider, payload.Model, payload.Mode)})
				}
			}
		case domain.EventModelRequest:
			var payload payloadModelRequest
			_ = json.Unmarshal(event.Payload, &payload)
			closeOpen(event.CreatedAt, "error", "interrupted by a new model request")
			step++
			open = &TrajectoryRequest{Turn: &turn, Group: fmt.Sprintf("Step %d", step),
				Status: "complete", StartedAt: event.CreatedAt, Provider: provider, Model: modelID,
				Messages: len(payload.Messages), PreambleBytes: payload.PreambleBytes}
		case domain.EventModelUsage:
			var payload payloadModelUsage
			if json.Unmarshal(event.Payload, &payload) == nil {
				usage = &TrajectoryTokens{Input: payload.PromptTokens, Output: payload.CompletionTokens,
					Think: payload.ReasoningTokens, CacheRead: payload.CachedTokens}
			}
		case domain.EventProviderRetry:
			if open != nil {
				open.Retry++
			}
		case domain.EventModelCompleted:
			var payload payloadModelCompleted
			content := ""
			if json.Unmarshal(event.Payload, &payload) == nil {
				content = payload.Content
			}
			if open != nil {
				group := open.Group
				startedAt := open.StartedAt
				appendRecord(TrajectoryRecord{Group: group, Kind: "message", Text: boundTrajectoryText(content),
					TimeSeconds: trajectorySeconds(startedAt, event.CreatedAt), StartedAt: &startedAt,
					Tokens: usage, Provider: provider, Model: modelID})
				closeOpen(event.CreatedAt, "complete", "")
			}
		case domain.EventToolRequested:
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
			if open != nil {
				group = open.Group
			} else if step > 0 {
				group = fmt.Sprintf("Step %d", step)
			}
			record := TrajectoryRecord{Group: group, Kind: "tool", Text: payload.ToolName,
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
				records = append(records, TrajectoryRecord{Group: "Compaction", Kind: "compacted",
					Text: fmt.Sprintf("%s · %d → %d tokens", payload.Mode, payload.BeforeTokens, payload.AfterTokens)})
			}
		case domain.EventRunFailed:
			var payload payloadRunFailed
			if json.Unmarshal(event.Payload, &payload) == nil {
				failure = payload.Message
			}
		}
	}
	if err := it.Err(); err != nil {
		return append(records, trajectoryFoldErrorRecord(turn, err)), requests, failure
	}
	closeOpen(lastEventTime(records), "error", failureOrInterrupted(failure))
	return records, requests, failure
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
