package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// payloadModelUsage mirrors runtime.payloadModelUsage for JSON decoding.
type payloadModelUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens"`
	CachedTokens     int `json:"cached_tokens"`
}

// payloadRunStarted mirrors runtime.payloadRunStarted for JSON decoding.
type payloadRunStarted struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// ListModelUsage returns every model.usage event with created_at >= since,
// joined with runs/sessions and the matching run.started payload. Missing
// run.started yields empty Model/Provider; the row still counts.
func (b *Backend) ListModelUsage(ctx context.Context, sinceUnixMilli int64) ([]storage.UsageRow, error) {
	const query = `SELECT e.run_id, e.created_at, e.payload,
			r.session_id, COALESCE(s.title, ''),
			rs.payload
		 FROM run_events e
		  JOIN runs r ON r.id = e.run_id
		  LEFT JOIN sessions s ON s.id = r.session_id
		  LEFT JOIN LATERAL (
		  	SELECT payload FROM run_events WHERE run_id = e.run_id AND type = 'run.started' LIMIT 1
		  ) rs ON true
		 WHERE e.type = 'model.usage' AND e.created_at >= $1
		 ORDER BY e.created_at ASC`
	rows, err := b.db.QueryContext(ctx, query, sinceUnixMilli)
	if err != nil {
		return nil, fmt.Errorf("storage: list model usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []storage.UsageRow
	for rows.Next() {
		var (
			runID        string
			createdAt    int64
			usageRaw     []byte
			sessionID    string
			sessionTitle string
			startedRaw   []byte
		)
		if err := rows.Scan(&runID, &createdAt, &usageRaw, &sessionID, &sessionTitle, &startedRaw); err != nil {
			return nil, fmt.Errorf("storage: scan model usage row: %w", err)
		}
		var usage payloadModelUsage
		if err := json.Unmarshal(usageRaw, &usage); err != nil {
			continue
		}
		row := storage.UsageRow{
			RunID:            domain.RunID(runID),
			SessionID:        domain.SessionID(sessionID),
			SessionTitle:     sessionTitle,
			CreatedAt:        createdAt,
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			ReasoningTokens:  usage.ReasoningTokens,
			CachedTokens:     usage.CachedTokens,
		}
		if len(startedRaw) > 0 {
			var started payloadRunStarted
			if err := json.Unmarshal(startedRaw, &started); err == nil {
				row.Model = started.Model
				row.Provider = started.Provider
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate model usage: %w", err)
	}
	return out, nil
}

var _ storage.TokenUsageStore = (*Backend)(nil)
