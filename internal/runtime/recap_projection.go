package runtime

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"fmt"
)

// This disposable projection never edits retained evidence. Availability is
// checked before exclusion, including every sibling in grouped tool messages.
func projectAutomaticRecapOriginals(rows []domain.Message) ([]domain.Message, []string, error) {
	if _, available := AutomaticInputProvenance(rows); !available {
		return nil, nil, fmt.Errorf("%w: original input provenance unavailable", ErrRecapUnavailable)
	}
	out := make([]domain.Message, 0, len(rows))
	notices := []string{}
	for start := 0; start < len(rows); {
		end := start + 1
		if rows[start].Role == domain.RoleAssistant && rows[start].ToolCallID != "" {
			calls := map[string]bool{rows[start].ToolCallID: true}
			for end < len(rows) && rows[end].Role == domain.RoleAssistant && rows[end].ToolCallID != "" {
				calls[rows[end].ToolCallID] = true
				end++
			}
			for end < len(rows) && rows[end].Role == domain.RoleTool && calls[rows[end].ToolCallID] {
				end++
			}
		}
		excluded, available := AutomaticInputProvenance(rows[start:end])
		if !available {
			return nil, nil, ErrRecapUnavailable
		}
		if excluded {
			for _, row := range rows[start:end] {
				notices = append(notices, fmt.Sprintf("[Automatic context notice: retained source excluded; session=%q run=%q message=%q]", row.SessionID, row.RunID, row.ID))
			}
		} else {
			out = append(out, rows[start:end]...)
		}
		start = end
	}
	return out, notices, nil
}

func recapDispositionNotices(snapshot storage.SubjectContextSnapshot, sequence uint64) []string {
	notices := []string{}
	for _, d := range snapshot.Dispositions {
		for _, target := range d.Targets {
			if target.Sequence == sequence {
				notices = append(notices, fmt.Sprintf("[Non-authoritative source disposition: superseded in view=%q marker=%d interaction=%q; historical occurrence and actual outcome remain retained.]", d.SessionID, d.MarkerID, target.InteractionID))
			}
		}
	}
	return notices
}
