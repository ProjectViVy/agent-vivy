package rpc

import (
	"context"
	"errors"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

// sidebarResult is deliberately a narrow active-session projection. The
// session collection remains session/list (Ctrl+S in terminal faces); this
// route contains only facts needed by the right rail.
type sidebarResult struct {
	SessionID domain.SessionID `json:"session_id"`
	Session   sessionResult    `json:"session"`
	CWD       string           `json:"cwd,omitempty"`
	Model     string           `json:"model,omitempty"`
	Provider  string           `json:"provider,omitempty"`
	// ReasoningKnown distinguishes an unsupported model from an unknown route.
	// The bool itself is meaningful only when this flag is true.
	ReasoningKnown     bool                         `json:"reasoning_known"`
	ReasoningSupported bool                         `json:"reasoning_supported"`
	Context            *runtime.ContextStatusResult `json:"context,omitempty"`
	Usage              *sidebarUsageResult          `json:"usage,omitempty"`
	ModifiedFilesKnown bool                         `json:"modified_files_known"`
	ModifiedFiles      []sidebarModifiedFileResult  `json:"modified_files"`
}

type sidebarUsageResult struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	ReasoningTokens  int     `json:"reasoning_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	RequestCount     int     `json:"request_count"`
	CostUSD          float64 `json:"cost_usd"`
	// CostKnown is false when no row had reference pricing. A false value is
	// never rendered as a free ($0) session.
	CostKnown bool `json:"cost_known"`
}

type sidebarDiffResult struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

type sidebarModifiedFileResult struct {
	Path      string            `json:"path"`
	Diff      sidebarDiffResult `json:"diff"`
	UpdatedAt int64             `json:"updated_at"`
}

// sessionSidebar serves active-session truth from each authoritative owner:
// session storage, ControlDeps.ProjectRoot, runtime.Service and the two
// journal-derived read projections. Missing optional owners leave that
// section absent instead of guessing from process state.
func (h *controlHandler) sessionSidebar(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	session, err := h.deps.Sessions.GetSession(ctx, domain.SessionID(params.SessionID))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "session not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	result := sidebarResult{
		SessionID: session.ID,
		Session:   toSessionResult(session),
		CWD:       strings.TrimSpace(h.deps.ProjectRoot),
	}

	if h.deps.Service != nil {
		providerName, modelID := h.deps.Service.CurrentModel()
		info := h.deps.Service.GetModelInfo(ctx)
		if providerName == "" {
			providerName = info.Provider
		}
		if modelID == "" {
			modelID = info.ID
		}
		result.Provider = strings.TrimSpace(providerName)
		result.Model = strings.TrimSpace(modelID)
		// ContextWindow is the same metadata-known guard used by
		// session/context for image/thinking capability flags.
		result.ReasoningKnown = info.ContextWindow > 0
		result.ReasoningSupported = info.SupportsThinking
		if status, statusErr := h.deps.Service.ContextStatus(ctx, session.ID); statusErr == nil {
			result.Context = &status
		} else if !errors.Is(statusErr, storage.ErrNotFound) {
			return nil, internalError(statusErr)
		}
	}

	if h.deps.TokenUsage != nil {
		var rows []storage.UsageRow
		var usageErr error
		if sessionUsage, ok := h.deps.TokenUsage.(storage.SessionTokenUsageStore); ok {
			rows, usageErr = sessionUsage.ListSessionModelUsage(ctx, session.ID)
		} else {
			// Do not turn an optional embedder seam into an unbounded full-store
			// scan. Absence means the session usage section is unavailable.
			rows = nil
		}
		if usageErr != nil {
			return nil, internalError(usageErr)
		}
		usage := buildSidebarUsage(ctx, rows, session.ID, h.deps.ModelMeta)
		if usage.RequestCount > 0 {
			result.Usage = &usage
		}
	}

	fileStore := h.deps.FileVersions
	if fileStore == nil {
		if candidate, ok := h.deps.Sessions.(storage.ModifiedFileStore); ok {
			fileStore = candidate
		}
	}
	if fileStore != nil {
		files, filesErr := fileStore.ListModifiedFiles(ctx, session.ID, storage.ModifiedFileMax)
		if filesErr != nil {
			return nil, internalError(filesErr)
		}
		result.ModifiedFilesKnown = true
		result.ModifiedFiles = make([]sidebarModifiedFileResult, 0, len(files))
		for _, file := range files {
			result.ModifiedFiles = append(result.ModifiedFiles, sidebarModifiedFileResult{
				Path: file.Path,
				Diff: sidebarDiffResult{
					Additions: file.Diff.Additions,
					Deletions: file.Diff.Deletions,
				},
				UpdatedAt: file.UpdatedAt,
			})
		}
	}
	return result, nil
}

func buildSidebarUsage(ctx context.Context, rows []storage.UsageRow, sessionID domain.SessionID, meta ModelMeta) sidebarUsageResult {
	var out sidebarUsageResult
	pricedRequests := 0
	for _, row := range rows {
		if row.SessionID != sessionID {
			continue
		}
		out.PromptTokens += row.PromptTokens
		out.CompletionTokens += row.CompletionTokens
		out.TotalTokens += row.TotalTokens
		out.ReasoningTokens += row.ReasoningTokens
		out.CachedTokens += row.CachedTokens
		requests := row.RequestCount
		if requests <= 0 {
			requests = 1
		}
		out.RequestCount += requests
		if cost, known := rowCostUSD(ctx, meta, row); known {
			out.CostUSD += cost
			pricedRequests += requests
		}
	}
	// A session total is known only when every request is priced. Reporting a
	// partial sum as the whole session cost would make unknown models look free.
	out.CostKnown = out.RequestCount > 0 && pricedRequests == out.RequestCount
	// rowCostUSD already rounds each row to the tokenstats precision. The
	// aggregate follows the same four-decimal contract as stats/tokens.
	out.CostUSD = roundSidebarCost(out.CostUSD)
	return out
}

func roundSidebarCost(value float64) float64 {
	// Keep this in the rpc package so the sidebar and stats projections share
	// the same four-decimal presentation without exporting tokenstats helpers.
	const scale = 10000.0
	if value < 0 {
		return -roundSidebarCost(-value)
	}
	return float64(int64(value*scale+0.5)) / scale
}
