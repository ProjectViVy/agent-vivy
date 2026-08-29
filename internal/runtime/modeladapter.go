package runtime

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// WrapModel bridges a domain.ChatModel to Eino's
// model.ToolCallingChatModel. Messages cross the D-007 firewall only here;
// the domain side never imports Eino. Tests wrap provider.NewMock() or a
// ScriptedModel; the product openai path passes the native eino-ext
// component straight through (no double wrap).
//
// V0 transports text content only (role + content). WithTools returns the
// adapter unchanged: the domain model contract does not emit tool calls in
// V0, which matches the spike's scriptedModel behavior
// (docs/eino-capability-verify.md).
func WrapModel(m domain.ChatModel) model.ToolCallingChatModel {
	return &modelAdapter{inner: m}
}

type modelAdapter struct {
	inner domain.ChatModel
}

var _ model.ToolCallingChatModel = (*modelAdapter)(nil)

func (a *modelAdapter) Generate(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	stream, err := a.Stream(ctx, input)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		sb.WriteString(chunk.Content)
	}
	return &schema.Message{Role: schema.Assistant, Content: sb.String()}, nil
}

func (a *modelAdapter) Stream(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ds, err := a.inner.Stream(ctx, fromEinoMessages(input))
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

func (a *modelAdapter) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return a, nil
}

func fromEinoMessages(ms []*schema.Message) []*domain.Message {
	out := make([]*domain.Message, 0, len(ms))
	for _, m := range ms {
		if m == nil {
			continue
		}
		row := &domain.Message{Role: fromEinoRole(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		if len(m.ToolCalls) > 0 {
			row.ToolCallID = m.ToolCalls[0].ID
			row.ToolName = m.ToolCalls[0].Function.Name
			row.ToolArgs = []byte(m.ToolCalls[0].Function.Arguments)
		}
		out = append(out, row)
	}
	return out
}

func fromEinoRole(r schema.RoleType) domain.Role {
	switch r {
	case schema.User:
		return domain.RoleUser
	case schema.Tool:
		return domain.RoleTool
	default:
		// System and any future roles collapse to assistant; the domain
		// vocabulary intentionally stays at three roles in V0.
		return domain.RoleAssistant
	}
}
