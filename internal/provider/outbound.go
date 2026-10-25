package provider

// Outbound wire normalizer. Eino's agentic projection emits text as content
// blocks, and the pinned ACL conversion serializes those as content arrays for
// both user and assistant turns. MiniMax's own harness sends text as a plain
// string, so this modifier canonicalizes the body after the ACL conversion and
// before the wire recorder: the summary then describes the bytes actually sent.
//
// It uses the ACL's RequestPayloadModifier seam instead of rewriting HTTP
// bodies: the option is applied inside the provider client, under the recorder,
// and needs no second request path. Normalization is best-effort — a body this
// file cannot parse is sent unchanged.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	openaiacl "github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// outboundNormalizedModel installs the payload modifier on every call. The
// option is appended last so it wins: model.GetImplSpecificOptions applies
// options in order.
type outboundNormalizedModel struct {
	inner model.AgenticModel
}

func newOutboundNormalizedModel(inner model.AgenticModel) model.AgenticModel {
	if inner == nil {
		return nil
	}
	return &outboundNormalizedModel{inner: inner}
}

func (m *outboundNormalizedModel) Generate(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	return m.inner.Generate(ctx, in, withOutboundNormalization(opts)...)
}

func (m *outboundNormalizedModel) Stream(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return m.inner.Stream(ctx, in, withOutboundNormalization(opts)...)
}

func withOutboundNormalization(opts []model.Option) []model.Option {
	normalized := make([]model.Option, 0, len(opts)+1)
	normalized = append(normalized, opts...)
	return append(normalized, openaiacl.WithRequestPayloadModifier(normalizeOutboundPayload))
}

func normalizeOutboundPayload(_ context.Context, _ []*schema.Message, rawBody []byte) ([]byte, error) {
	return normalizeOutboundBody(rawBody), nil
}

// normalizeOutboundBody returns rawBody unchanged when nothing needed to be
// rewritten, so a call the provider would accept is never altered in shape.
func normalizeOutboundBody(rawBody []byte) []byte {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		slog.Warn("outbound wire normalization skipped; body is not a JSON object", "err", err)
		return rawBody
	}
	rawMessages, ok := envelope["messages"]
	if !ok {
		return rawBody
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(rawMessages, &messages); err != nil {
		slog.Warn("outbound wire normalization skipped; messages is not an array", "err", err)
		return rawBody
	}
	changed := false
	for i, raw := range messages {
		normalized, didChange := normalizeOutboundMessage(raw)
		if didChange {
			messages[i] = normalized
			changed = true
		}
	}
	if !changed {
		return rawBody
	}
	encodedMessages, err := json.Marshal(messages)
	if err != nil {
		slog.Warn("outbound wire normalization skipped; re-encoding messages failed", "err", err)
		return rawBody
	}
	envelope["messages"] = encodedMessages
	encoded, err := json.Marshal(envelope)
	if err != nil {
		slog.Warn("outbound wire normalization skipped; re-encoding body failed", "err", err)
		return rawBody
	}
	return encoded
}

// normalizeOutboundMessage edits only the fields this file owns. Every other
// message field (name, tool_call_id, reasoning_content) is preserved.
func normalizeOutboundMessage(raw json.RawMessage) (json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return raw, false
	}
	changed := false
	role := outboundRole(fields["role"])
	if role == "user" || role == "assistant" {
		if content, ok := fields["content"]; ok {
			if normalized, didChange := collapseTextContent(content); didChange {
				fields["content"] = normalized
				changed = true
			}
		}
	}
	if calls, ok := fields["tool_calls"]; ok {
		if normalized, didChange := normalizeToolCalls(calls); didChange {
			fields["tool_calls"] = normalized
			changed = true
		}
	}
	if !changed {
		return raw, false
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return raw, false
	}
	return encoded, true
}

func outboundRole(raw json.RawMessage) string {
	var role string
	if len(raw) == 0 {
		return ""
	}
	if err := json.Unmarshal(raw, &role); err != nil {
		return ""
	}
	return role
}

// collapseTextContent turns a text-only content array into one string. Arrays
// carrying any non-text part keep their original form.
func collapseTextContent(raw json.RawMessage) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return raw, false
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(trimmed, &parts); err != nil || len(parts) == 0 {
		return raw, false
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type != "text" {
			return raw, false
		}
		texts = append(texts, part.Text)
	}
	encoded, err := json.Marshal(strings.Join(texts, "\n"))
	if err != nil {
		return raw, false
	}
	return encoded, true
}

func normalizeToolCalls(raw json.RawMessage) (json.RawMessage, bool) {
	var calls []json.RawMessage
	if err := json.Unmarshal(raw, &calls); err != nil {
		return raw, false
	}
	changed := false
	for i, rawCall := range calls {
		var call map[string]json.RawMessage
		if err := json.Unmarshal(rawCall, &call); err != nil {
			continue
		}
		var function map[string]json.RawMessage
		if err := json.Unmarshal(call["function"], &function); err != nil {
			continue
		}
		var arguments string
		if len(function["arguments"]) > 0 {
			if err := json.Unmarshal(function["arguments"], &arguments); err != nil {
				continue
			}
		}
		normalized, didChange := normalizeToolArguments(outboundToolCallID(call["id"]), arguments)
		if !didChange {
			continue
		}
		encodedArguments, err := json.Marshal(normalized)
		if err != nil {
			continue
		}
		function["arguments"] = encodedArguments
		encodedFunction, err := json.Marshal(function)
		if err != nil {
			continue
		}
		call["function"] = encodedFunction
		encodedCall, err := json.Marshal(call)
		if err != nil {
			continue
		}
		calls[i] = encodedCall
		changed = true
	}
	if !changed {
		return raw, false
	}
	encoded, err := json.Marshal(calls)
	if err != nil {
		return raw, false
	}
	return encoded, true
}

func outboundToolCallID(raw json.RawMessage) string {
	var id string
	if len(raw) == 0 {
		return ""
	}
	if err := json.Unmarshal(raw, &id); err != nil {
		return ""
	}
	return id
}

// normalizeToolArguments replays only syntactically valid JSON. A truncated or
// corrupt argument string (an aborted stream persisted mid-call) becomes an
// empty object: the call id and its tool result still pair, and the model never
// receives bytes the provider cannot parse. Values are compacted, never
// re-encoded through any, so key order and number lexemes survive.
func normalizeToolArguments(callID, arguments string) (string, bool) {
	if arguments == "" {
		return "{}", true
	}
	if !json.Valid([]byte(arguments)) {
		slog.Warn("outbound tool arguments were not valid JSON; replaying an empty object",
			"tool_call_id", callID, "argument_bytes", len(arguments))
		return "{}", true
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(arguments)); err != nil {
		return "{}", true
	}
	if compact.String() == arguments {
		return arguments, false
	}
	return compact.String(), true
}
