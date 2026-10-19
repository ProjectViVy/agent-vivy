package runtime

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// WrapModel bridges a domain.ChatModel to the agentic Eino model seam. Domain
// messages cross the D-007 firewall only here; the domain side never imports
// Eino. Tests use ScriptedModel or a test-only domain model; the product
// provider path passes the native agentic eino-ext component straight through
// (no wrap, no second generation).
//
// This is the domain↔Eino projection, not a text-generation bridge: the
// adapter still speaks schema.AgenticMessage, and a domain model that emits
// only text produces assistant text blocks. Tool calls are bound at request
// time by the agent, so the adapter carries no WithTools method.
func WrapModel(m domain.ChatModel) model.AgenticModel {
	return &modelAdapter{inner: m}
}

type modelAdapter struct {
	inner domain.ChatModel
}

var _ model.AgenticModel = (*modelAdapter)(nil)

func (a *modelAdapter) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	stream, err := a.Stream(ctx, input, opts...)
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
		sb.WriteString(agenticAssistantText(chunk))
	}
	return agenticAssistantTextMessage(sb.String()), nil
}

func (a *modelAdapter) Stream(ctx context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ds, err := a.inner.Stream(ctx, fromAgenticMessages(input))
	if err != nil {
		return nil, err
	}
	r, w := schema.Pipe[*schema.AgenticMessage](8)
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
			if w.Send(agenticAssistantTextMessage(chunk.Content), nil) {
				return
			}
		}
	}()
	return r, nil
}

// fromAgenticMessages projects the agentic feed onto the three-role domain
// vocabulary one domain model consumes. Tool results carry their call id and
// name; tool calls carry the first call's identity, matching the previous
// domain projection.
func fromAgenticMessages(ms []*schema.AgenticMessage) []*domain.Message {
	out := make([]*domain.Message, 0, len(ms))
	for _, m := range ms {
		if m == nil {
			continue
		}
		row := &domain.Message{Role: agenticRole(m), Content: agenticMessageText(m)}
		if results := agenticToolResults(m); len(results) > 0 {
			row.ToolCallID = results[0].CallID
			row.ToolName = results[0].Name
		}
		if calls := agenticToolCalls(m); len(calls) > 0 {
			row.ToolCallID = calls[0].CallID
			row.ToolName = calls[0].Name
			row.ToolArgs = []byte(calls[0].Arguments)
		}
		out = append(out, row)
	}
	return out
}
