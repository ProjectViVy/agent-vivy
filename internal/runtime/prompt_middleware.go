package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

// promptMiddleware binds the immutable prompt captured for a run to Eino's
// per-execution instruction and checks the final projected model input. The
// middleware is deliberately run-local: the Engine's static instruction is
// retained as the legacy/no-snapshot fallback and is never mutated.
//
// It must be registered after every other user handler. Eino registers the
// first handler as the outermost WrapModel layer, so placing this handler last
// makes the budget check observe the final post-compaction/tool projection.
type promptMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	maxBytes int
}

func newPromptMiddleware(maxBytes int) adk.ChatModelAgentMiddleware {
	return &promptMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		maxBytes:                     maxBytes,
	}
}

// BeforeAgent replaces the shared/static instruction with the immutable
// instruction admitted for this run. Assignment is intentional: appending
// here would create a second persona on resume and on every tool iteration.
func (m *promptMiddleware) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext) (context.Context, *adk.ChatModelAgentContext, error) {
	if runCtx == nil {
		return ctx, runCtx, nil
	}
	snapshot, ok := runPrompt(ctx)
	if !ok {
		// Legacy runs have no prompt marker and retain the static engine
		// instruction. This is the only path that bypasses snapshot checks.
		return ctx, runCtx, nil
	}
	instruction, err := promptInstruction(snapshot)
	if err != nil {
		return ctx, nil, err
	}
	next := *runCtx
	// Earlier native handlers (for example Eino's skill middleware) append
	// transient tool instructions to the shared static instruction. Replace
	// only that static prefix so the admitted persona/mask is authoritative
	// while those scoped additions remain available to the run.
	static := composeStaticInstruction()
	addition := ""
	switch {
	case strings.HasPrefix(runCtx.Instruction, instruction):
		// A repeated BeforeAgent pass may receive the already authoritative
		// instruction. Keep only its transient suffix so persona/mask content
		// cannot duplicate across tool iterations or Resume.
		addition = strings.TrimPrefix(runCtx.Instruction, instruction)
	case strings.HasPrefix(runCtx.Instruction, static):
		addition = strings.TrimPrefix(runCtx.Instruction, static)
	case runCtx.Instruction != "":
		// A third-party handler may have replaced the prefix entirely. Preserve
		// its bounded addition rather than silently dropping the capability.
		addition = runCtx.Instruction
	}
	next.Instruction = instruction + addition
	return ctx, &next, nil
}

// WrapModel sees the exact []schema.Message produced by GenModelInput,
// including the authoritative system instruction and any transient native
// middleware injections. It therefore guards the actual call boundary rather
// than an earlier history approximation.
func (m *promptMiddleware) WrapModel(ctx context.Context, next model.BaseModel[*schema.Message], _ *adk.ModelContext) (model.BaseModel[*schema.Message], error) {
	if next == nil {
		return nil, errors.New("runtime: prompt middleware received nil model")
	}
	return &promptBudgetModel{inner: next, maxBytes: m.maxBytes}, nil
}

// literalGenModelInput is deliberately free of Eino's default FString
// formatting. SessionValues are useful to native middleware, but an admitted
// mask body is data and must preserve literal braces such as {system} and
// {{persona}} all the way to the provider.
func literalGenModelInput(_ context.Context, instruction string, input *adk.AgentInput) ([]*schema.Message, error) {
	capacity := 0
	if input != nil {
		capacity = len(input.Messages)
	}
	if instruction != "" {
		capacity++
	}
	msgs := make([]*schema.Message, 0, capacity)
	if instruction != "" {
		msgs = append(msgs, schema.SystemMessage(instruction))
	}
	if input != nil {
		msgs = append(msgs, input.Messages...)
	}
	return msgs, nil
}

type promptBudgetModel struct {
	inner    model.BaseModel[*schema.Message]
	maxBytes int
}

func (m *promptBudgetModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if err := m.check(ctx, input); err != nil {
		return nil, err
	}
	return m.inner.Generate(ctx, input, opts...)
}

func (m *promptBudgetModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if err := m.check(ctx, input); err != nil {
		return nil, err
	}
	return m.inner.Stream(ctx, input, opts...)
}

func (m *promptBudgetModel) check(ctx context.Context, input []*schema.Message) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if m.maxBytes <= 0 {
		return nil
	}
	used := projectedContextBytes(input)
	if used > m.maxBytes {
		return fmt.Errorf("%w: final model input requires %d bytes; budget is %d", ErrContextBudgetExceeded, used, m.maxBytes)
	}
	return nil
}

// promptInstruction validates the durable snapshot before it becomes an
// Eino instruction. The opaque checkpoint itself is never decoded here; the
// snapshot payload is host-owned JSON persisted beside the run admission.
func promptInstruction(snapshot storage.RunPromptSnapshot) (string, error) {
	if snapshot.SchemaVersion != promptSchemaVersion {
		return "", fmt.Errorf("runtime: prompt snapshot schema %d is unsupported", snapshot.SchemaVersion)
	}
	if snapshot.ComposerVersion != promptComposerVersion {
		return "", maskcontract.NewError(maskcontract.CodeIncompatiblePrompt, errors.New("runtime: prompt snapshot composer version is unsupported"))
	}
	if _, err := storage.ValidateRunPromptSnapshot(snapshot); err != nil {
		return "", err
	}
	var payload storage.RunPromptPayload
	if err := json.Unmarshal(snapshot.Payload, &payload); err != nil {
		return "", fmt.Errorf("runtime: decode prompt snapshot: %w", err)
	}
	if strings.TrimSpace(payload.Instruction) == "" {
		return "", errors.New("runtime: prompt snapshot instruction is empty")
	}
	return payload.Instruction, nil
}

func promptInstructionReservation(ctx context.Context) (int, error) {
	snapshot, ok := runPrompt(ctx)
	if !ok {
		// Legacy runs still receive the Engine's static instruction. Reserve
		// that fixed system message during history selection as well; otherwise
		// a full transcript can pass the early budget and be rejected only at
		// the final model boundary.
		instruction := composeStaticInstruction()
		if instruction == "" {
			return 0, nil
		}
		return projectedContextBytes([]*schema.Message{schema.SystemMessage(instruction)}), nil
	}
	instruction, err := promptInstruction(snapshot)
	if err != nil {
		return 0, err
	}
	return projectedContextBytes([]*schema.Message{schema.SystemMessage(instruction)}), nil
}
