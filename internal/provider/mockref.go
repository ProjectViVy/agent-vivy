package provider

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// mockRef exposes the deterministic domain-level Mock (FR-3) behind the
// Ref seam so config.Runtime.Mock can select it through the same path as
// real bundles.
type mockRef struct{}

func newMockRef() Ref { return mockRef{} }

func (mockRef) Name() string { return "mock" }

func (mockRef) Model(_ context.Context, modelID string) (model.ToolCallingChatModel, error) {
	scenario := ""
	if strings.HasPrefix(modelID, "mock:") {
		scenario = strings.TrimPrefix(modelID, "mock:")
	}
	return &mockEinoModel{m: NewMock(), scenario: scenario}, nil
}

func (mockRef) ModelInfo(_ context.Context, modelID string) (domain.ModelInfo, error) {
	// Mock provider uses a conservative default context window suitable
	// for testing. In practice, tests don't need huge contexts.
	info := domain.ModelInfo{
		ID:              modelID,
		Provider:        "mock",
		ContextWindow:   128000, // 128K tokens as reasonable default
		MaxOutputTokens: 4096,
	}
	if info.ID == "" {
		info.ID = "mock-default"
	}
	return info, nil
}

// mockEinoModel bridges domain.ChatModel to Eino's ToolCallingChatModel.
// It mirrors runtime.modelAdapter but stays provider-local: normal mock mode
// emits plain text, while test-only scenario mode can emit fixed tool calls
// to exercise the real HITL path.
type mockEinoModel struct {
	m        domain.ChatModel
	scenario string
}

func (a *mockEinoModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if a.scenario != "" {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return a.scenarioMessage(in), nil
	}
	stream, err := a.Stream(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	var content strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		content.WriteString(chunk.Content)
	}
	return &schema.Message{Role: schema.Assistant, Content: content.String()}, nil
}

func (a *mockEinoModel) Stream(ctx context.Context, in []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a.scenario != "" {
		r, w := schema.Pipe[*schema.Message](1)
		go func() {
			defer w.Close()
			if w.Send(a.scenarioMessage(in), nil) {
				return
			}
		}()
		return r, nil
	}
	hist := make([]*domain.Message, 0, len(in))
	for _, msg := range in {
		if msg == nil {
			continue
		}
		role := domain.RoleAssistant
		if msg.Role == schema.User {
			role = domain.RoleUser
		}
		hist = append(hist, &domain.Message{Role: role, Content: msg.Content})
	}
	ds, err := a.m.Stream(ctx, hist)
	if err != nil {
		return nil, err
	}
	r, w := schema.Pipe[*schema.Message](8)
	go func() {
		defer w.Close()
		for {
			if err := ctx.Err(); err != nil {
				w.Send(nil, err)
				return
			}
			chunk, err := ds.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				w.Send(nil, err)
				return
			}
			// Send reports whether the reader side closed; stop pumping
			// when it does.
			if w.Send(&schema.Message{Role: schema.Assistant, Content: chunk.Content}, nil) {
				return
			}
		}
	}()
	return r, nil
}

// scenarioMessage is deliberately small and deterministic. It drives the
// real Eino tool-call, interrupt, ReviewItem, RPC, and UI paths in offline
// acceptance tests; it is never selected by the normal mock provider.
func (a *mockEinoModel) scenarioMessage(in []*schema.Message) *schema.Message {
	lastUser := ""
	lastUserIndex := -1
	for index, msg := range in {
		if msg != nil && msg.Role == schema.User {
			lastUser = strings.ToLower(strings.TrimSpace(msg.Content))
			lastUserIndex = index
		}
	}
	if lastUser == "" {
		return schema.AssistantMessage("mock scenario is waiting for a user message", nil)
	}
	toolResult := false
	for index := lastUserIndex + 1; index < len(in); index++ {
		if in[index] != nil && in[index].Role == schema.Tool {
			toolResult = true
			break
		}
	}
	if toolResult {
		return schema.AssistantMessage("mock scenario completed: "+a.scenario, nil)
	}

	toolName, args, ok := scenarioTool(a.scenario, lastUser)
	if !ok {
		return schema.AssistantMessage("mock reply to: "+lastUser, nil)
	}
	return schema.AssistantMessage("", []schema.ToolCall{{
		ID:       "call-e2e-1",
		Function: schema.FunctionCall{Name: toolName, Arguments: args},
	}})
}

func scenarioTool(scenario, user string) (string, string, bool) {
	if scenario == "hitl" {
		switch {
		case strings.Contains(user, "question"):
			scenario = "question"
		case strings.Contains(user, "stale"):
			scenario = "stale"
		case strings.Contains(user, "approval"), strings.Contains(user, "timeout"):
			scenario = "approval"
		default:
			return "", "", false
		}
	}
	switch scenario {
	case "approval", "timeout":
		return "write_note", `{"content":"e2e approval note"}`, true
	case "question":
		return "ask_user", `{"question":"Which color should I use?"}`, true
	case "stale":
		return "write_file", `{"path":"e2e/stale.txt","content":"new deterministic content"}`, true
	default:
		return "", "", false
	}
}

// WithTools remains a no-op: scenario mode owns its fixed tool call, while
// normal mock mode remains text-only.
func (a *mockEinoModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return a, nil
}
