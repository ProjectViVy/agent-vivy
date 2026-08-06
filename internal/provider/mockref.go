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

func (mockRef) Model(_ context.Context, _ string) (model.ToolCallingChatModel, error) {
	return &mockEinoModel{m: NewMock()}, nil
}

// mockEinoModel bridges domain.ChatModel to Eino's ToolCallingChatModel.
// It mirrors runtime.modelAdapter but stays provider-local: the mock
// emits plain text and never issues tool calls, so the bridge stays
// minimal (text content and role only).
type mockEinoModel struct {
	m domain.ChatModel
}

func (a *mockEinoModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
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

// WithTools is a no-op: the mock never issues tool calls.
func (a *mockEinoModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return a, nil
}
