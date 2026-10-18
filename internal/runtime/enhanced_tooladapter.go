package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// enhancedToolAdapter exposes the Eino EnhancedInvokableTool surface while
// delegating all Vivy validation, policy, HITL, hooks, budgets, and run
// identity work to the ordinary adapter.
type enhancedToolAdapter struct{ inner *toolAdapter }

var _ einotool.EnhancedInvokableTool = (*enhancedToolAdapter)(nil)

func newEnhancedToolAdapter(inner *toolAdapter) *enhancedToolAdapter {
	return &enhancedToolAdapter{inner: inner}
}

func (a *enhancedToolAdapter) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return a.inner.Info(ctx)
}

func (a *enhancedToolAdapter) InvokableRun(ctx context.Context, argument *schema.ToolArgument, _ ...einotool.Option) (*schema.ToolResult, error) {
	if argument == nil {
		return nil, fmt.Errorf("runtime: enhanced tool argument is nil")
	}
	result, err := a.inner.InvokableRun(ctx, argument.Text)
	if err != nil {
		return nil, err
	}
	trusted := false
	if coordinator, ok := toolOperationCoordinatorFromContext(ctx).(serviceToolOperationCoordinator); ok {
		keep, err := coordinator.retained(ctx)
		if err != nil {
			return nil, err
		}
		if keep {
			op, found, err := coordinator.Lookup(ctx, compose.GetToolCallID(ctx), a.inner.t.Spec().Name, []byte(argument.Text))
			if err != nil {
				return nil, err
			}
			if found && op.State == domain.ToolOperationCompleted {
				exact, err := coordinator.resolve(ctx, op)
				if err != nil {
					return nil, err
				}
				trusted = result == exact
			}
		}
	}
	return normalizeEnhancedResult(result, a.inner.maxResultBytes, trusted)
}

type rawToolOutput struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	URL        string `json:"url,omitempty"`
	Base64Data string `json:"base64data,omitempty"`
	MIMEType   string `json:"mime_type,omitempty"`
}

type rawToolEnvelope struct {
	ExactReference string          `json:"exact_reference,omitempty"`
	Parts          []rawToolOutput `json:"parts"`
}

func normalizeEnhancedResult(result string, budget int, retained ...bool) (*schema.ToolResult, error) {
	result = strings.TrimPrefix(result, untrustedToolResultHeader)
	var envelope rawToolEnvelope
	if err := json.Unmarshal([]byte(result), &envelope); err != nil || len(envelope.Parts) == 0 {
		text := untrustedToolResultHeader + result
		if len(retained) == 0 || !retained[0] {
			text = compactToolResult(text, budget)
		}
		return &schema.ToolResult{Parts: []schema.ToolOutputPart{{Type: schema.ToolPartTypeText, Text: text}}}, nil
	}
	parts := make([]schema.ToolOutputPart, 0, len(envelope.Parts))
	used := 0
	for _, raw := range envelope.Parts {
		part := schema.ToolOutputPart{}
		common := schema.MessagePartCommon{MIMEType: raw.MIMEType}
		if raw.URL != "" {
			value := raw.URL
			common.URL = &value
		}
		if raw.Base64Data != "" {
			value := raw.Base64Data
			common.Base64Data = &value
		}
		switch strings.ToLower(raw.Type) {
		case "text":
			part.Type, part.Text = schema.ToolPartTypeText, raw.Text
		case "image":
			part.Type, part.Image = schema.ToolPartTypeImage, &schema.ToolOutputImage{MessagePartCommon: common}
		case "audio":
			part.Type, part.Audio = schema.ToolPartTypeAudio, &schema.ToolOutputAudio{MessagePartCommon: common}
		case "video":
			part.Type, part.Video = schema.ToolPartTypeVideo, &schema.ToolOutputVideo{MessagePartCommon: common}
		case "file":
			part.Type, part.File = schema.ToolPartTypeFile, &schema.ToolOutputFile{MessagePartCommon: common}
		default:
			return nil, fmt.Errorf("runtime: unsupported enhanced tool part %q", raw.Type)
		}
		if budget > 0 && part.Type == schema.ToolPartTypeText {
			// Only text counts against the byte budget; media parts are
			// bounded where they are produced. An over-budget text part is
			// compacted head+tail instead of dropped.
			encoded, _ := json.Marshal(part)
			if used+len(encoded) > budget {
				if remaining := budget - used; remaining > 0 {
					part.Text = compactToolResult(part.Text, remaining)
				} else {
					part.Text = "[text part dropped: tool result budget exhausted]"
				}
				encoded, _ = json.Marshal(part)
			}
			used += len(encoded)
		}
		parts = append(parts, part)
	}
	if len(retained) > 0 && retained[0] && envelope.ExactReference != "" {
		parts = append(parts, schema.ToolOutputPart{Type: schema.ToolPartTypeText, Text: envelope.ExactReference})
	}
	return &schema.ToolResult{Parts: parts}, nil
}
