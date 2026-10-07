package runtime

import (
	"context"
	"encoding/json"
	"log/slog"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// sessionToolActivation returns the session's deferred-tool activation set.
// The tracker is folded once from every run journal of the session —
// tools.exposure_changed control events plus tool_search results — so
// activation survives restarts, and then updated live: tool_search
// completions feed it through the persist path, tools/activate through
// SetToolActivation. Replay failures degrade to a smaller set with a
// warning, matching the sessionMounts seed contract.
func (s *Service) sessionToolActivation(ctx context.Context, sessionID domain.SessionID) *tools.ToolActivation {
	s.toolActivationMu.Lock()
	defer s.toolActivationMu.Unlock()
	if activation := s.toolActivations[sessionID]; activation != nil {
		return activation
	}
	activation := tools.NewToolActivation()
	s.toolActivations[sessionID] = activation
	runs, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		slog.Warn("tool activation: run listing failed", "session", string(sessionID), "err", err)
		return activation
	}
	for _, run := range runs {
		s.foldToolActivationRun(ctx, activation, run.ID)
	}
	// The synthetic control run carries tools/activate events recorded
	// after the session's real runs have sealed.
	s.foldToolActivationRun(ctx, activation, toolExposureRunID(sessionID))
	return activation
}

func (s *Service) foldToolActivationRun(ctx context.Context, activation *tools.ToolActivation, runID domain.RunID) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		slog.Warn("tool activation: journal replay failed", "run", string(runID), "err", err)
		return
	}
	defer func() { _ = it.Close() }()
	for it.Next() {
		event := it.Value().Event
		switch event.Type {
		case domain.EventToolsExposureChanged:
			var p payloadToolsExposureChanged
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				slog.Warn("tool activation: unreadable tools.exposure_changed payload", "run", string(runID))
				continue
			}
			activation.Activate(p.Activated...)
			activation.Deactivate(p.Deactivated...)
		case domain.EventToolFinished:
			matches := toolSearchMatches(event.Payload)
			if len(matches) > 0 {
				activation.Activate(matches...)
			}
		}
	}
	if err := it.Err(); err != nil {
		slog.Warn("tool activation: journal replay failed", "run", string(runID), "err", err)
	}
}

// toolExposureRunID is the session's synthetic control run for exposure
// changes. It has no RunStore row and never receives a terminal event,
// so tools.exposure_changed appends stay legal after real runs seal.
func toolExposureRunID(sessionID domain.SessionID) domain.RunID {
	return domain.RunID("sessctl_toolx_" + string(sessionID))
}

// toolSearchMatches extracts the tool names a finished tool_search call
// matched — the same forward-selection vocabulary Eino's middleware
// scans in the conversation state.
func toolSearchMatches(payload json.RawMessage) []string {
	var finished payloadToolFinished
	if err := json.Unmarshal(payload, &finished); err != nil || finished.ToolName != officialToolSearchName || finished.Error != "" {
		return nil
	}
	var result struct {
		Matches []string `json:"matches"`
	}
	if err := json.Unmarshal([]byte(finished.Result), &result); err != nil {
		return nil
	}
	return result.Matches
}

// noteToolSearchMatches folds a journaled tool_search completion into the
// session activation set. It runs synchronously inside the persist path so
// the names are callable before the next model leg of the same run.
func (s *Service) noteToolSearchMatches(sessionID domain.SessionID, payload json.RawMessage) {
	matches := toolSearchMatches(payload)
	if len(matches) == 0 {
		return
	}
	s.sessionToolActivation(context.Background(), sessionID).Activate(matches...)
}

// SetToolActivation applies a session-scoped activation or deactivation
// from the tools/activate and tools/deactivate control verbs. The change is
// journaled as tools.exposure_changed on the session's synthetic control
// run — real runs seal at their terminal event, so a dedicated non-sealing
// run keeps the Journal durable regardless of when the flip arrives.
func (s *Service) SetToolActivation(ctx context.Context, sessionID domain.SessionID, names []string, activate bool) error {
	if len(names) == 0 {
		return nil
	}
	activation := s.sessionToolActivation(ctx, sessionID)
	payload := payloadToolsExposureChanged{}
	if activate {
		activation.Activate(names...)
		payload.Activated = append([]string(nil), names...)
	} else {
		activation.Deactivate(names...)
		payload.Deactivated = append([]string(nil), names...)
	}
	_, err := s.recordSyntheticSessionEvent(ctx, sessionID, toolExposureRunID(sessionID), domain.EventToolsExposureChanged, payload)
	return err
}

func mustMarshalToolExposure(payload payloadToolsExposureChanged) json.RawMessage {
	data, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}

// ToolActivation returns the session's currently activated deferred tool
// names (sorted) for the tools/list projection.
func (s *Service) ToolActivation(ctx context.Context, sessionID domain.SessionID) []string {
	return s.sessionToolActivation(ctx, sessionID).Snapshot()
}

// dropSessionToolActivation releases the in-memory tracker when the
// session is deleted.
func (s *Service) dropSessionToolActivation(sessionID domain.SessionID) {
	s.toolActivationMu.Lock()
	delete(s.toolActivations, sessionID)
	s.toolActivationMu.Unlock()
}
