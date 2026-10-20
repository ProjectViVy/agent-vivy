package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// enhancedToolAdapter exposes the Eino EnhancedInvokableTool surface while
// delegating all Vivy validation, policy, HITL, hooks, redaction, and run
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
	return normalizeEnhancedResult(result, a.inner.maxResultBytes)
}

type rawToolOutput struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	URL        string `json:"url,omitempty"`
	Base64Data string `json:"base64data,omitempty"`
	MIMEType   string `json:"mime_type,omitempty"`
}

type rawToolEnvelope struct {
	Parts []rawToolOutput `json:"parts"`
}

func normalizeEnhancedResult(result string, budget int) (*schema.ToolResult, error) {
	result = strings.TrimPrefix(result, untrustedToolResultHeader)
	var envelope rawToolEnvelope
	if err := json.Unmarshal([]byte(result), &envelope); err != nil || len(envelope.Parts) == 0 {
		return &schema.ToolResult{Parts: []schema.ToolOutputPart{{Type: schema.ToolPartTypeText, Text: compactToolResult(untrustedToolResultHeader+result, budget)}}}, nil
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
	return &schema.ToolResult{Parts: parts}, nil
}

// enhancedResultProjection reconstructs the ordinary mapper projection when
// native Eino reuses an executed result without another middleware event.
// Reuse the same normalization and native block types as the live adapter;
// Eino's ToolResult-to-agentic block converter is private to compose.
func enhancedResultProjection(result string, budget int) (string, []json.RawMessage, error) {
	normalized, err := normalizeEnhancedResult(result, budget)
	if err != nil {
		return "", nil, err
	}
	value := func(pointer *string) string {
		if pointer == nil {
			return ""
		}
		return *pointer
	}
	var text strings.Builder
	var parts []json.RawMessage
	for _, part := range normalized.Parts {
		block := schema.FunctionToolResultContentBlock{Type: schema.FunctionToolResultContentBlockType(part.Type)}
		switch part.Type {
		case schema.ToolPartTypeText:
			text.WriteString(part.Text)
			continue
		case schema.ToolPartTypeImage:
			block.Image = &schema.UserInputImage{URL: value(part.Image.URL), Base64Data: value(part.Image.Base64Data), MIMEType: part.Image.MIMEType}
		case schema.ToolPartTypeAudio:
			block.Audio = &schema.UserInputAudio{URL: value(part.Audio.URL), Base64Data: value(part.Audio.Base64Data), MIMEType: part.Audio.MIMEType}
		case schema.ToolPartTypeVideo:
			block.Video = &schema.UserInputVideo{URL: value(part.Video.URL), Base64Data: value(part.Video.Base64Data), MIMEType: part.Video.MIMEType}
		case schema.ToolPartTypeFile:
			block.File = &schema.UserInputFile{URL: value(part.File.URL), Base64Data: value(part.File.Base64Data), MIMEType: part.File.MIMEType}
		default:
			return "", nil, fmt.Errorf("runtime: unsupported enhanced projection part %q", part.Type)
		}
		encoded, err := json.Marshal(block)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, encoded)
	}
	return text.String(), parts, nil
}
