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
		if budget > 0 {
			encoded, _ := json.Marshal(part)
			if used+len(encoded) > budget {
				break
			}
			used += len(encoded)
		}
		parts = append(parts, part)
	}
	return &schema.ToolResult{Parts: parts}, nil
}
