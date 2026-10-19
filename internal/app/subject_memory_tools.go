package app

import (
	"context"
	"encoding/json"
	"errors"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	memorymodule "agent-vivy/internal/modules/memory"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
	"github.com/ProjectViVy/laputa/laputa/evolution"
)

// This private App route selects the already owned native sink using persisted
// run policy and the runtime's current tool grant. No caller chooses an owner.
func subjectMemoryTools(svc *runtime.Service, store storage.Engine, bundle cognitivecontract.Bundle) func(context.Context, string, json.RawMessage) (json.RawMessage, bool, error) {
	return func(ctx context.Context, name string, raw json.RawMessage) (json.RawMessage, bool, error) {
		rid := tools.RunIDFromContext(ctx)
		if rid == "" {
			return nil, false, nil
		}
		sources, ok := store.(storage.SubjectSourceStore)
		if !ok {
			return nil, false, nil
		}
		pin, err := sources.GetRunSubjectSource(ctx, rid)
		if errors.Is(err, storage.ErrNotFound) || (err == nil && pin.PolicyID != domain.SubjectResidentPolicy) {
			return nil, false, nil
		}
		fail := func(reason string) (json.RawMessage, bool, error) {
			out, _ := json.Marshal(map[string]any{"status": "failed", "reason": reason})
			return out, true, nil
		}
		if err != nil {
			return fail("recap_scope_denied")
		}
		binding, err := svc.AuthorizeRecap(ctx, tools.SessionIDFromContext(ctx), rid, name)
		if err != nil {
			return fail("recap_scope_denied")
		}
		owner, ok := bundle.(cognitivecontract.RecapProvider)
		if !ok {
			return fail("recap_scope_denied")
		}
		head, err := owner.Recap().ReadRecap(ctx)
		if err != nil || head.Binding != binding {
			return fail("recap_scope_denied")
		}
		selected, ok := bundle.(cognitivecontract.SubjectMemoryProvider)
		if !ok || selected.SubjectMemory() == nil {
			return fail("subject_memory_unavailable")
		}
		backend := selected.SubjectMemory()
		scope := evolution.Scope{SubjectID: binding.SubjectID, Kind: evolution.ScopePersonal}
		var result any
		switch name {
		case memorymodule.ToolSearch:
			var input struct {
				Query  string `json:"query"`
				Limit  int    `json:"limit"`
				Cursor string `json:"cursor"`
			}
			if json.Unmarshal(raw, &input) != nil || input.Query == "" {
				return fail("memory_invalid_request")
			}
			limit, ok := subjectSearchLimit(input.Limit)
			if !ok {
				return fail("memory_invalid_request")
			}
			input.Limit = limit
			page, readErr := backend.Search(ctx, gardenmemory.AuthorizedSearch{Scopes: []evolution.Scope{scope}, Query: input.Query, Limit: input.Limit, Cursor: input.Cursor})
			if readErr != nil {
				out, _ := json.Marshal(map[string]any{"status": "failed", "reason": "subject_memory_unavailable", "native_error": readErr.Error()})
				return out, true, nil
			}
			result = map[string]any{"status": "listed", "entries": page.Items, "next_cursor": page.NextCursor, "scope": "subject", "expansion": "memory_get requires id and expected_revision from this page"}
		case memorymodule.ToolGet:
			var input struct {
				ID       string `json:"id"`
				Revision uint64 `json:"expected_revision"`
			}
			if json.Unmarshal(raw, &input) != nil || input.ID == "" || input.Revision == 0 {
				return fail("memory_expected_revision_required")
			}
			page, readErr := backend.Expand(ctx, gardenmemory.AuthorizedExpansion{Scopes: []evolution.Scope{scope}, CardID: input.ID, ExpectedRevision: input.Revision, BudgetChars: 4000})
			if readErr != nil {
				out, _ := json.Marshal(map[string]any{"status": "failed", "reason": "subject_memory_unavailable", "native_error": readErr.Error()})
				return out, true, nil
			}
			result = map[string]any{"status": "listed", "entries": page.Items, "truncated": page.Truncated, "scope": "subject"}
		default:
			// Legacy machine-home mutation/list schemas do not confer subject authority.
			// Keep capability truth explicit rather than silently using unqualified rows.
			return fail("subject_memory_operation_unavailable")
		}
		out, err := json.Marshal(result)
		return out, true, err
	}
}

// subjectSearchLimit maps a requested page size onto what the subject backend
// serves. The shared memory_search schema accepts any positive limit, so a
// larger request is clamped to one full page rather than rejected.
func subjectSearchLimit(requested int) (int, bool) {
	switch {
	case requested < 0:
		return 0, false
	case requested == 0:
		return 20, true
	case requested > 100:
		return 100, true
	}
	return requested, true
}
