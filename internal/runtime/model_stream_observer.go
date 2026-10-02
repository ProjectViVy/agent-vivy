package runtime

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Model call lifecycle observation (OBS-02 / D2). Every runtime-owned
// model invocation — chat route Generate/Stream and the compaction
// summarizer's primary/fallback BaseModel calls — emits a journaled
// lifecycle: a v3 model.request inside Begin (before the provider is
// invoked), cumulative model.usage v2 samples, and exactly one
// model.call.finished at End. Begin failing is fail-closed: the provider
// is never invoked when the call cannot be journaled.

// modelCallInput is the exact invocation surface handed to the model.
type modelCallInput struct {
	Mode     string // "generate" | "stream"
	Messages []*schema.Message
	// Tools is the resolved bound-tool set the model will call with
	// (WithTools bindings plus per-call common options).
	Tools []*schema.ToolInfo
}

// modelCallMeta is the immutable identity one call reports across its
// lifecycle events.
type modelCallMeta struct {
	CallID        string
	Provider      string
	Model         string
	Source        string
	ContextViewID string
}

// modelCallResult carries the call outcome to End. Usage is the last
// provider-reported sample; ResponseComplete is true only when the model
// produced a complete response (normal stream EOF or Generate success).
type modelCallResult struct {
	Err              error
	Usage            *schema.TokenUsage
	ResponseComplete bool
}

// modelCallRoute names which model boundary a wrapper serves. Source ""
// is the chat route — the observer factory resolves it to "main" or
// "child" from the run's execution context. "summary" marks the
// compaction summarizer's calls; FallbackModel distinguishes the summary
// failover onto the main chat model from the primary summary model so
// the route's model field stays truthful.
type modelCallRoute struct {
	Source        string
	FallbackModel bool
}

const modelCallSourceSummary = "summary"

// modelCallObserver is the runtime-owned seam every observed model call
// reports through. Begin persists the v3 request before the provider is
// invoked; StreamOpened marks a materialized call (successful inner
// Stream setup, or a returned Generate message); Chunk observes each
// forwarded chunk (or the single Generate message); End settles the call
// exactly once.
type modelCallObserver interface {
	Begin(context.Context, modelCallInput) (modelCallMeta, error)
	StreamOpened(modelCallMeta)
	Chunk(context.Context, modelCallMeta, *schema.Message) error
	End(context.Context, modelCallMeta, modelCallResult) error
}

// modelCallObserverFactory is the per-run context value: it binds the
// owning run, route metadata, and emitter for one wrapper's calls.
type modelCallObserverFactory func(modelCallRoute) modelCallObserver

type modelCallObserverKey struct{}

func withModelCallObserverFactory(ctx context.Context, factory modelCallObserverFactory) context.Context {
	return context.WithValue(ctx, modelCallObserverKey{}, factory)
}

func modelCallObserverFactoryFrom(ctx context.Context) modelCallObserverFactory {
	factory, _ := ctx.Value(modelCallObserverKey{}).(modelCallObserverFactory)
	return factory
}

// observedModelCallCore carries the wrapper state shared by the
// ToolCallingChatModel and BaseModel observers.
type observedModelCallCore struct {
	route      modelCallRoute
	boundTools []*schema.ToolInfo
}

// begin resolves the observer and runs Begin. A nil observer means the
// call proceeds unobserved (legacy/test paths keep the old behavior); a
// Begin error is returned to the caller and the provider is not invoked.
func (c *observedModelCallCore) begin(ctx context.Context, mode string, input []*schema.Message, opts []model.Option) (modelCallObserver, modelCallMeta, error) {
	factory := modelCallObserverFactoryFrom(ctx)
	if factory == nil {
		return nil, modelCallMeta{}, nil
	}
	obs := factory(c.route)
	if obs == nil {
		return nil, modelCallMeta{}, nil
	}
	tools := c.boundTools
	if common := model.GetCommonOptions(nil, opts...); common != nil && len(common.Tools) > 0 {
		tools = mergeToolInfos(tools, common.Tools)
	}
	meta, err := obs.Begin(ctx, modelCallInput{Mode: mode, Messages: input, Tools: tools})
	if err != nil {
		return nil, meta, err
	}
	return obs, meta, nil
}

func mergeToolInfos(base, extra []*schema.ToolInfo) []*schema.ToolInfo {
	if len(base) == 0 {
		return append([]*schema.ToolInfo(nil), extra...)
	}
	seen := make(map[string]struct{}, len(base)+len(extra))
	out := make([]*schema.ToolInfo, 0, len(base)+len(extra))
	for _, info := range base {
		out = append(out, info)
		if info != nil {
			seen[info.Name] = struct{}{}
		}
	}
	for _, info := range extra {
		if info == nil {
			out = append(out, info)
			continue
		}
		if _, dup := seen[info.Name]; dup {
			continue
		}
		seen[info.Name] = struct{}{}
		out = append(out, info)
	}
	return out
}

func usageOfMessage(msg *schema.Message) *schema.TokenUsage {
	if msg == nil || msg.ResponseMeta == nil {
		return nil
	}
	return msg.ResponseMeta.Usage
}

// observeGenerate runs one observed non-streaming invocation: Begin,
// inner Generate, the returned message observed once, then End.
func observeGenerate(ctx context.Context, core *observedModelCallCore, inner model.BaseModel[*schema.Message], input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	obs, meta, err := core.begin(ctx, "generate", input, opts)
	if err != nil {
		return nil, err
	}
	if obs == nil {
		return inner.Generate(ctx, input, opts...)
	}
	msg, callErr := inner.Generate(ctx, input, opts...)
	if callErr != nil {
		_ = obs.End(ctx, meta, modelCallResult{Err: callErr})
		return nil, callErr
	}
	obs.StreamOpened(meta)
	if err := obs.Chunk(ctx, meta, msg); err != nil {
		_ = obs.End(ctx, meta, modelCallResult{Err: err, Usage: usageOfMessage(msg)})
		return nil, err
	}
	_ = obs.End(ctx, meta, modelCallResult{Usage: usageOfMessage(msg), ResponseComplete: true})
	return msg, nil
}

// observeStream runs one observed streaming invocation: Begin before the
// inner Stream setup (so setup errors are journaled), StreamOpened only
// after the upstream reader exists, then a bounded pipe pump that calls
// Chunk before each downstream Recv and End exactly once on EOF, read
// error, observer failure, downstream close, or cancellation.
//
// Inspected Eino v0.9.13 callbacks.OnEndWithStreamOutput /
// schema.StreamReader.Copy / compose.genericOnEndWithStreamOutput: those
// timings Copy the stream after inner Stream returns. A sync handler can
// run before compose Recv, but it is still a sibling copy — it cannot put
// persist on the producer Recv→Send path, apply Pipe(8) backpressure onto
// provider Recv, fail-close the graph copy from a persist error, or run
// the tool-settled barrier. Revisit if Eino adds a producer-path Recv hook
// with those semantics.
func observeStream(ctx context.Context, core *observedModelCallCore, inner model.BaseModel[*schema.Message], input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	obs, meta, err := core.begin(ctx, "stream", input, opts)
	if err != nil {
		return nil, err
	}
	if obs == nil {
		return inner.Stream(ctx, input, opts...)
	}
	upstream, err := inner.Stream(ctx, input, opts...)
	if err != nil {
		_ = obs.End(ctx, meta, modelCallResult{Err: err})
		return nil, err
	}
	// Mark the call before exposing the tee to Eino. Starting the marker
	// in the pump goroutine races Eino's eager stream forwarding: the
	// materialized event can otherwise enter the mapper first and emit
	// the same deltas again.
	obs.StreamOpened(meta)
	reader, writer := schema.Pipe[*schema.Message](8)
	go func() {
		defer upstream.Close()
		var lastUsage *schema.TokenUsage
		result := modelCallResult{}
		// End settles before the downstream sees the pipe close so a
		// run terminal can never overtake the call's finish record.
		finish := func() {
			result.Usage = lastUsage
			_ = obs.End(ctx, meta, result)
			writer.Close()
		}
		for {
			chunk, recvErr := upstream.Recv()
			if recvErr == io.EOF {
				result.ResponseComplete = true
				finish()
				return
			}
			if recvErr != nil {
				result.Err = recvErr
				writer.Send(nil, recvErr)
				finish()
				return
			}
			if chunk == nil {
				continue
			}
			if usage := usageOfMessage(chunk); usage != nil {
				lastUsage = usage
			}
			if err := obs.Chunk(ctx, meta, chunk); err != nil {
				result.Err = err
				writer.Send(nil, err)
				finish()
				return
			}
			if writer.Send(chunk, nil) {
				result.Err = context.Canceled
				finish()
				return
			}
		}
	}()
	return reader, nil
}

// observingChatModel wraps a ToolCallingChatModel for the run's chat
// route; WithTools returns an observed clone that keeps the bound-tool
// scope inside this call's request record.
type observingChatModel struct {
	inner model.ToolCallingChatModel
	core  observedModelCallCore
}

func observeChatModel(inner model.ToolCallingChatModel) model.ToolCallingChatModel {
	return &observingChatModel{inner: inner}
}

func (m *observingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return observeGenerate(ctx, &m.core, m.inner, input, opts...)
}

func (m *observingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return observeStream(ctx, &m.core, m.inner, input, opts...)
}

func (m *observingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &observingChatModel{
		inner: bound,
		core:  observedModelCallCore{route: m.core.route, boundTools: mergeToolInfos(m.core.boundTools, tools)},
	}, nil
}

// observingBaseModel wraps a plain BaseModel — the compaction
// summarizer's primary and failover routes, which never bind tools.
type observingBaseModel struct {
	inner model.BaseModel[*schema.Message]
	core  observedModelCallCore
}

func observeSummaryModel(inner model.BaseModel[*schema.Message], fallback bool) model.BaseModel[*schema.Message] {
	return &observingBaseModel{
		inner: inner,
		core:  observedModelCallCore{route: modelCallRoute{Source: modelCallSourceSummary, FallbackModel: fallback}},
	}
}

func (m *observingBaseModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return observeGenerate(ctx, &m.core, m.inner, input, opts...)
}

func (m *observingBaseModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return observeStream(ctx, &m.core, m.inner, input, opts...)
}

// usageAccumulator merges one call's cumulative usage samples through the
// pinned Eino merge (schema.ConcatMessages on usage-only messages):
// bounded state, no text accumulation. Contradictory (decreasing) samples
// and non-cumulative total conventions mark the call partial rather than
// silently rewriting evidence (D2).
type usageAccumulator struct {
	merged   *schema.Message
	latest   *schema.TokenUsage
	partial  bool
	reported bool
	lastEmit string
}

func (a *usageAccumulator) record(u *schema.TokenUsage) {
	if u == nil {
		return
	}
	a.reported = true
	a.latest = u
	if prev := a.mergedUsage(); prev != nil {
		if u.PromptTokens < prev.PromptTokens ||
			u.CompletionTokens < prev.CompletionTokens ||
			u.TotalTokens < prev.TotalTokens ||
			u.PromptTokenDetails.CachedTokens < prev.PromptTokenDetails.CachedTokens ||
			u.CompletionTokensDetails.ReasoningTokens < prev.CompletionTokensDetails.ReasoningTokens {
			a.partial = true
		}
	}
	if u.TotalTokens != u.PromptTokens+u.CompletionTokens {
		a.partial = true
	}
	merged, err := schema.ConcatMessages([]*schema.Message{
		a.baseMessage(),
		{ResponseMeta: &schema.ResponseMeta{Usage: u}},
	})
	if err != nil {
		a.partial = true
		return
	}
	a.merged = merged
}

func (a *usageAccumulator) baseMessage() *schema.Message {
	if a.merged != nil {
		return a.merged
	}
	return &schema.Message{}
}

func (a *usageAccumulator) mergedUsage() *schema.TokenUsage {
	if a.merged == nil || a.merged.ResponseMeta == nil {
		return nil
	}
	return a.merged.ResponseMeta.Usage
}

// normalizedUsageSample is the merged, VIVY-convention usage shape: total
// is input + output, and reasoning/cached keep a presence bit (positive
// is known, zero is unknown) so reported scalars are never synthesized.
type normalizedUsageSample struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	ReasoningTokens  *int
	CachedTokens     *int
	Partial          bool
}

func (a *usageAccumulator) sample() (normalizedUsageSample, bool) {
	u := a.mergedUsage()
	if u == nil {
		return normalizedUsageSample{}, false
	}
	s := normalizedUsageSample{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.PromptTokens + u.CompletionTokens,
		Partial:          a.partial,
	}
	if v := u.CompletionTokensDetails.ReasoningTokens; v > 0 {
		s.ReasoningTokens = &v
	}
	if v := u.PromptTokenDetails.CachedTokens; v > 0 {
		s.CachedTokens = &v
	}
	return s, true
}

func (s normalizedUsageSample) key() string {
	reasoning, cached := -1, -1
	if s.ReasoningTokens != nil {
		reasoning = *s.ReasoningTokens
	}
	if s.CachedTokens != nil {
		cached = *s.CachedTokens
	}
	return fmt.Sprintf("%d/%d/%d/%d/%d/%t", s.PromptTokens, s.CompletionTokens, s.TotalTokens, reasoning, cached, s.Partial)
}
