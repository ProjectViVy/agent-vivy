package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// payloadModelUsage mirrors runtime.payloadModelUsage for JSON decoding.
// Field names match schemas/events/payloads/model.usage.json v1.
type payloadModelUsage struct {
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	ReasoningTokens  int    `json:"reasoning_tokens"`
	CachedTokens     int    `json:"cached_tokens"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	Source           string `json:"source"`
}

// payloadRunStarted mirrors runtime.payloadRunStarted for JSON decoding.
// Field names match schemas/events/payloads/run.started.json v1.
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
		  LEFT JOIN run_events rs ON rs.run_id = e.run_id AND rs.type = 'run.started'
		   AND rs.seq = (SELECT MIN(first.seq) FROM run_events first WHERE first.run_id = e.run_id AND first.type = 'run.started')
		 WHERE e.type = 'model.usage' AND e.created_at >= ?
		 ORDER BY e.created_at ASC`
	return b.listModelUsage(ctx, query, false, sinceUnixMilli)
}

// ListSessionModelUsage restricts the journal projection at the database
// boundary so active-session refreshes do not scan unrelated history.
func (b *Backend) ListSessionModelUsage(ctx context.Context, sessionID domain.SessionID) ([]storage.UsageRow, error) {
	const query = `SELECT e.run_id, e.created_at, e.payload,
			r.session_id, COALESCE(s.title, ''),
			rs.payload
		 FROM run_events e
		  JOIN runs r ON r.id = e.run_id
		  LEFT JOIN sessions s ON s.id = r.session_id
		  LEFT JOIN run_events rs ON rs.run_id = e.run_id AND rs.type = 'run.started'
		   AND rs.seq = (SELECT MIN(first.seq) FROM run_events first WHERE first.run_id = e.run_id AND first.type = 'run.started')
		 WHERE e.type = 'model.usage' AND r.session_id = ?
		 ORDER BY e.created_at ASC`
	return b.listModelUsage(ctx, query, true, sessionID)
}

func (b *Backend) listModelUsage(ctx context.Context, query string, aggregateRoutes bool, args ...any) ([]storage.UsageRow, error) {
	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list model usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []storage.UsageRow
	aggregates := make(map[string]storage.UsageRow)
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
			// Skip corrupted payloads rather than failing the whole query.
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
			RequestCount:     1,
			Source:           usage.Source,
		}
		if len(startedRaw) > 0 {
			var started payloadRunStarted
			if err := json.Unmarshal(startedRaw, &started); err == nil {
				row.Model = started.Model
				row.Provider = started.Provider
			}
		}
		applyUsageAttribution(&row, usage.Provider, usage.Model, usage.Source)
		if !aggregateRoutes {
			out = append(out, row)
			continue
		}
		key := row.Provider + "\x00" + row.Model
		if _, exists := aggregates[key]; !exists && len(aggregates) >= storage.SessionUsageRouteMax {
			key = ""
			row.Provider, row.Model = "", ""
		}
		aggregateUsageRow(aggregates, key, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate model usage: %w", err)
	}
	if !aggregateRoutes {
		return out, nil
	}
	keys := make([]string, 0, len(aggregates))
	for key := range aggregates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, aggregates[key])
	}
	return out, nil
}

func applyUsageAttribution(row *storage.UsageRow, provider, model, source string) {
	// Summary middleware may use an override and fail over to the main model;
	// until the adapter reports the actual route, never inherit main-run
	// pricing for that auxiliary call.
	if source == "summary" {
		row.Provider, row.Model = "", ""
	}
	if provider != "" || model != "" {
		row.Provider, row.Model = "", ""
		if provider != "" && model != "" {
			row.Provider, row.Model = provider, model
		}
	}
}

func aggregateUsageRow(rows map[string]storage.UsageRow, key string, row storage.UsageRow) {
	current := rows[key]
	if current.SessionID == "" {
		current = row
		current.PromptTokens, current.CompletionTokens, current.TotalTokens = 0, 0, 0
		current.ReasoningTokens, current.CachedTokens, current.RequestCount = 0, 0, 0
	}
	current.PromptTokens += row.PromptTokens
	current.CompletionTokens += row.CompletionTokens
	current.TotalTokens += row.TotalTokens
	current.ReasoningTokens += row.ReasoningTokens
	current.CachedTokens += row.CachedTokens
	current.RequestCount += row.RequestCount
	if row.CreatedAt > current.CreatedAt {
		current.CreatedAt = row.CreatedAt
	}
	rows[key] = current
}

var _ storage.TokenUsageStore = (*Backend)(nil)
var _ storage.SessionTokenUsageStore = (*Backend)(nil)
