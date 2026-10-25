package provider

import (
	"bytes"
	"encoding/json"
	"testing"
)

type outboundTestMessage struct {
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	ToolCalls []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
	ToolCallID string `json:"tool_call_id"`
}

type outboundTestBody struct {
	Messages      []outboundTestMessage `json:"messages"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
}

func decodeOutboundTestBody(t *testing.T, body []byte) outboundTestBody {
	t.Helper()
	var decoded outboundTestBody
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode normalized body: %v\n%s", err, body)
	}
	return decoded
}

func TestNormalizeOutboundTextContentArrays(t *testing.T) {
	body := []byte(`{"model":"m","stream_options":{"include_usage":true},"messages":[` +
		`{"role":"system","content":"sys"},` +
		`{"role":"user","content":[{"type":"text","text":"hello"}]},` +
		`{"role":"assistant","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]},` +
		`{"role":"tool","content":"ok","tool_call_id":"c1"}]}`)

	normalized := normalizeOutboundBody(body)
	decoded := decodeOutboundTestBody(t, normalized)
	if len(decoded.Messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(decoded.Messages))
	}
	if got := string(decoded.Messages[0].Content); got != `"sys"` {
		t.Fatalf("system content = %s, want a string", got)
	}
	if got := string(decoded.Messages[1].Content); got != `"hello"` {
		t.Fatalf("user content = %s, want a string", got)
	}
	if got := string(decoded.Messages[2].Content); got != `"a\nb"` {
		t.Fatalf("assistant content = %s, want the joined string", got)
	}
	if got := string(decoded.Messages[3].Content); got != `"ok"` {
		t.Fatalf("tool content = %s, want it untouched", got)
	}
	if decoded.StreamOptions == nil || !decoded.StreamOptions.IncludeUsage {
		t.Fatalf("stream_options lost: %s", normalized)
	}
}

func TestNormalizeOutboundPreservesMixedContent(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[` +
		`{"type":"text","text":"look"},` +
		`{"type":"image_url","image_url":{"url":"https://example.test/a.png"}}]}]}`)

	normalized := normalizeOutboundBody(body)
	if !bytes.Equal(normalized, body) {
		t.Fatalf("mixed content changed:\n got %s\nwant %s", normalized, body)
	}
}

func TestNormalizeOutboundToolArguments(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":null,"tool_calls":[` +
		`{"id":"c1","type":"function","function":{"name":"echo","arguments":"{ \"a\" : 1 }"}},` +
		`{"id":"c2","type":"function","function":{"name":"echo","arguments":"{\"n\":12345678901234567890}"}},` +
		`{"id":"c3","type":"function","function":{"name":"echo","arguments":""}},` +
		`{"id":"c4","type":"function","function":{"name":"echo","arguments":"{\"broken\":"}}]}]}`)

	decoded := decodeOutboundTestBody(t, normalizeOutboundBody(body))
	calls := decoded.Messages[0].ToolCalls
	if len(calls) != 4 {
		t.Fatalf("tool calls = %d, want 4", len(calls))
	}
	if calls[0].Function.Arguments != `{"a":1}` {
		t.Fatalf("compact arguments = %s", calls[0].Function.Arguments)
	}
	if calls[1].Function.Arguments != `{"n":12345678901234567890}` {
		t.Fatalf("large integer lexeme = %s", calls[1].Function.Arguments)
	}
	if calls[2].Function.Arguments != `{}` || calls[3].Function.Arguments != `{}` {
		t.Fatalf("empty/invalid arguments = %q/%q, want {}", calls[2].Function.Arguments, calls[3].Function.Arguments)
	}
	if calls[0].ID != "c1" || calls[0].Function.Name != "echo" || calls[0].Type != "function" {
		t.Fatalf("tool call identity lost: %+v", calls[0])
	}
}

func TestNormalizeOutboundIsIdempotent(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":[{"type":"text","text":"a"}],` +
		`"tool_calls":[{"id":"c1","type":"function","function":{"name":"echo","arguments":"{ \"a\" : 1 }"}}]}]}`)
	once := normalizeOutboundBody(body)
	twice := normalizeOutboundBody(once)
	if !bytes.Equal(once, twice) {
		t.Fatalf("normalization is not idempotent:\n once %s\ntwice %s", once, twice)
	}
}

func TestNormalizeOutboundFailsOpenOnMalformedBody(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`not json`),
		[]byte(`[1,2,3]`),
		[]byte(`{"messages":"nope"}`),
		[]byte(`{"messages":[42]}`),
	} {
		if got := normalizeOutboundBody(body); !bytes.Equal(got, body) {
			t.Fatalf("malformed body was rewritten:\n got %s\nwant %s", got, body)
		}
	}
}
