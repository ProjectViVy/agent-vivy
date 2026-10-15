package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// Version one LIFE policy owns the requested cap. It is neither a provider
// capability assertion nor an end-user setting. Native adapters enforce it.
const recapOutputTokens = 4096

var ErrContextCapacityRequired = fmt.Errorf("%w: context_capacity_required", ErrContextBudgetExceeded)

type recapRequestUsage struct {
	TextualTokenUpperBound, OutputReserve, Window int
	Estimated                                     bool
}

func (s *Service) recapOutputLimit(ctx context.Context) int {
	n := recapOutputTokens
	if _, summary := ctx.Value(recapSummaryKey{}).(*recapFold); summary {
		n = recapSummaryOutputTokens
	}
	if maximum := s.recapModelInfo(ctx).MaxOutputTokens; maximum > 0 && maximum < n {
		n = maximum
	}
	return n
}
func (s *Service) measureRecapRequest(ctx context.Context, messages []*schema.Message, tools []*schema.ToolInfo) (recapRequestUsage, error) {
	info := s.recapModelInfo(ctx)
	usage := recapRequestUsage{OutputReserve: s.recapOutputLimit(ctx), Window: info.ContextWindow, Estimated: true}
	if usage.Window <= 0 {
		return usage, fmt.Errorf("%w: model context window unavailable", ErrContextCapacityRequired)
	}
	add := func(n int) bool {
		if n < 0 || usage.TextualTokenUpperBound > math.MaxInt-n {
			return false
		}
		usage.TextualTokenUpperBound += n
		return true
	}
	for _, message := range messages {
		if message == nil {
			continue
		}
		// No pinned adapter declares a non-text token charge. Exact bytes remain
		// retained, but automatic resident transmission cannot guess that charge.
		if len(message.MultiContent) > 0 || len(message.AssistantGenMultiContent) > 0 {
			return usage, fmt.Errorf("%w: non-text charge unavailable", ErrContextCapacityRequired)
		}
		for _, part := range message.UserInputMultiContent {
			if part.Type != schema.ChatMessagePartTypeText {
				return usage, fmt.Errorf("%w: non-text charge unavailable", ErrContextCapacityRequired)
			}
		}
		copy := *message
		copy.Extra = nil
		copy.ResponseMeta = nil
		raw, err := json.Marshal(copy)
		if err != nil {
			return usage, fmt.Errorf("%w: message serialization", ErrContextCapacityRequired)
		}
		if !add(len(raw) + contextMessageOverhead) {
			return usage, ErrContextCapacityRequired
		}
	}
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		var parameters any
		if tool.ParamsOneOf != nil {
			value, err := tool.ParamsOneOf.ToJSONSchema()
			if err != nil {
				return usage, fmt.Errorf("%w: tool schema unavailable", ErrContextCapacityRequired)
			}
			parameters = value
		}
		raw, err := json.Marshal(struct {
			Name, Description string
			Parameters        any
		}{tool.Name, tool.Desc, parameters})
		if err != nil || !add(len(raw)+contextMessageOverhead) {
			return usage, ErrContextCapacityRequired
		}
	}
	// Every UTF-8 byte is charged as at most one textual token, including JSON
	// framing, names, full schemas, references and all runtime instructions.
	if usage.TextualTokenUpperBound > usage.Window-usage.OutputReserve {
		return usage, fmt.Errorf("%w: input upper bound=%d output reserve=%d window=%d", ErrContextCapacityRequired, usage.TextualTokenUpperBound, usage.OutputReserve, usage.Window)
	}
	eng := engineFromContext(ctx)
	if eng == nil {
		eng = s.engine
	}
	if eng != nil && eng.cfg.MaxContextBytes > 0 && usage.TextualTokenUpperBound > eng.cfg.MaxContextBytes {
		return usage, fmt.Errorf("%w: request exceeds configured input byte capacity", ErrContextCapacityRequired)
	}
	return usage, nil
}

type residentRecapKey struct{}

func residentRecap(ctx context.Context) bool {
	enabled, _ := ctx.Value(residentRecapKey{}).(bool)
	return enabled
}
func (s *Service) withRecapRunPolicy(ctx context.Context, rid domain.RunID, sid domain.SessionID, providerName, modelID string) context.Context {
	selected, err := s.residentRecapRun(ctx, rid, sid)
	if !selected && err == nil {
		return ctx
	}
	// Unavailable policy never silently selects legacy compression. The
	// final observer propagates the actual error before invoking the provider.
	ctx = context.WithValue(ctx, residentRecapKey{}, true)
	ctx = domain.WithModelRequestScope(ctx, providerName, modelID)
	return domain.WithModelOutputLimit(ctx, s.recapOutputLimit(ctx))
}

// Native legacy compression remains the ordinary Coding strategy. It cannot
// retire selected LIFE context before Task2 commits a C1 operation receipt.
type codingCompactionMiddleware struct{ adk.ChatModelAgentMiddleware }

func (m codingCompactionMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if residentRecap(ctx) {
		return ctx, state, nil
	}
	return m.ChatModelAgentMiddleware.BeforeModelRewriteState(ctx, state, mc)
}

func (s *Service) recapModelInfo(ctx context.Context) domain.ModelInfo {
	pin, ok := domain.ModelRequestScopeFromContext(ctx)
	if !ok {
		return s.GetModelInfo(ctx)
	}
	s.mu.Lock()
	catalog := s.catalog
	s.mu.Unlock()
	if catalog == nil || pin.Provider == "" || pin.Model == "" {
		return domain.ModelInfo{}
	}
	info, err := catalog.ResolveModelInfo(ctx, pin.Provider, pin.Model)
	if err != nil {
		return domain.ModelInfo{}
	}
	return info
}

// residentRecapRun distinguishes positively known ordinary/child execution
// from unavailable selected-policy pins. Session identity cannot grant a child
// resident input, and a missing primary pin cannot authorize a resident Run.
func (s *Service) residentRecapRun(ctx context.Context, rid domain.RunID, sid domain.SessionID) (bool, error) {
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		if s.deps.MainConversation != nil || s.deps.SubjectSources != nil {
			return false, storage.ErrSubjectBinding
		}
		return false, nil
	}
	pin, err := sources.GetRunSubjectSource(ctx, rid)
	if err == nil {
		return pin.ConversationID != "", nil
	}
	if err != storage.ErrNotFound {
		return false, err
	}
	run, err := s.deps.Runs.GetRun(ctx, rid)
	if err != nil {
		return false, err
	}
	if run.Kind == domain.RunKindChild || run.Kind == domain.RunKindWorkflow {
		return false, nil
	}
	current, err := s.resolveSubjectAdmission(ctx, sid)
	if err != nil {
		return false, err
	}
	if current != nil && current.ConversationID != "" {
		return false, storage.ErrSubjectBinding
	}
	return false, nil
}
