package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// usageProjectionEventTypes enumerates the Journal event families the
// usage projection folds: run.started context, observed request/usage/
// finish lifecycle records and legacy usage samples.
const usageProjectionEventTypes = `('run.started','model.request','model.usage','model.call.finished')`

// ListModelUsage returns the folded usage projection for the window:
// observed attempts selected by request start time, legacy records by
// sample time. Candidate runs are those with any usage-relevant event in
// the window; their full model.* prefix is folded so replacement samples
// and finishes resolve against their own attempt.
func (b *Backend) ListModelUsage(ctx context.Context, sinceUnixMilli int64) ([]storage.UsageRow, error) {
	const query = `SELECT e.run_id, r.session_id, COALESCE(s.title, ''), r.status,
			e.type, e.seq, e.created_at, e.payload
		 FROM run_events e
		  JOIN runs r ON r.id = e.run_id
		  LEFT JOIN sessions s ON s.id = r.session_id
		 WHERE e.type IN ` + usageProjectionEventTypes + `
		  AND e.run_id IN (
		    SELECT run_id FROM run_events
		    WHERE (type = 'model.request' AND created_at >= ?)
		       OR (type = 'model.usage' AND created_at >= ?)
		  )
		 ORDER BY e.run_id, e.seq`
	runs, err := b.listUsageRuns(ctx, query, sinceUnixMilli, sinceUnixMilli)
	if err != nil {
		return nil, err
	}
	var out []storage.UsageRow
	for _, run := range runs {
		out = append(out, storage.ProjectUsageRows(run, sinceUnixMilli)...)
	}
	return out, nil
}

// ListSessionModelUsage restricts the journal projection at the database
// boundary so active-session refreshes do not scan unrelated history.
// The session reader has no time window: every attempt and legacy record
// of the session's runs is projected.
func (b *Backend) ListSessionModelUsage(ctx context.Context, sessionID domain.SessionID) ([]storage.UsageRow, error) {
	const query = `SELECT e.run_id, r.session_id, COALESCE(s.title, ''), r.status,
			e.type, e.seq, e.created_at, e.payload
		 FROM run_events e
		  JOIN runs r ON r.id = e.run_id
		  LEFT JOIN sessions s ON s.id = r.session_id
		 WHERE e.type IN ` + usageProjectionEventTypes + `
		  AND r.session_id = ?
		 ORDER BY e.run_id, e.seq`
	runs, err := b.listUsageRuns(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	var out []storage.UsageRow
	for _, run := range runs {
		out = append(out, storage.ProjectUsageRows(run, 0)...)
	}
	return storage.AggregateUsageRows(out), nil
}

// listUsageRuns loads candidate runs' canonical events in (run_id, seq)
// order and groups them into projection inputs.
func (b *Backend) listUsageRuns(ctx context.Context, query string, args ...any) ([]storage.UsageRunInput, error) {
	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list model usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []storage.UsageRunInput
	byID := make(map[string]int)
	for rows.Next() {
		var (
			runID        string
			sessionID    string
			sessionTitle string
			runStatus    string
			eventType    string
			seq          int
			createdAt    int64
			payload      []byte
		)
		if err := rows.Scan(&runID, &sessionID, &sessionTitle, &runStatus, &eventType, &seq, &createdAt, &payload); err != nil {
			return nil, fmt.Errorf("storage: scan model usage row: %w", err)
		}
		idx, ok := byID[runID]
		if !ok {
			idx = len(out)
			byID[runID] = idx
			out = append(out, storage.UsageRunInput{
				RunID:        domain.RunID(runID),
				SessionID:    domain.SessionID(sessionID),
				SessionTitle: sessionTitle,
				RunStatus:    domain.RunStatus(runStatus),
			})
		}
		out[idx].Events = append(out[idx].Events, storage.CanonicalUsageEvent{
			Type:      domain.EventType(eventType),
			Seq:       seq,
			CreatedAt: createdAt,
			Payload:   json.RawMessage(payload),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate model usage: %w", err)
	}
	return out, nil
}

var _ storage.TokenUsageStore = (*Backend)(nil)
var _ storage.SessionTokenUsageStore = (*Backend)(nil)
