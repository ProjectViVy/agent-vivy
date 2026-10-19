package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// Hand-written SSE in each vendor's published wire format.
const (
	ccChunk = `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,`

	ccReasoningReply = "data: " + ccChunk + `"delta":{"role":"assistant","reasoning_content":"think "}}]}` + "\n\n" +
		"data: " + ccChunk + `"delta":{"reasoning_content":"hard"}}]}` + "\n\n" +
		"data: " + ccChunk + `"delta":{"content":"Sunny "}}]}` + "\n\n" +
		"data: " + ccChunk + `"delta":{"content":"in Paris."}}]}` + "\n\n" +
		"data: " + ccChunk + `"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
		"data: [DONE]\n\n"

	ccToolCallReply = "data: " + ccChunk + `"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}]}}]}` + "\n\n" +
		"data: " + ccChunk + `"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n" +
		"data: [DONE]\n\n"

	anReply = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-5\",\"content\":[],\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":4,\"output_tokens\":1}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Sunny \"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"in Paris.\"}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":7}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
)

// cutBefore ends a fixture just before the first occurrence of marker, as a
// proxy or connection that closes cleanly mid-reply would.
func cutBefore(s, marker string) string { return s[:strings.Index(s, marker)] }

type streamResult struct {
	text, reasoning string
	err             error
}

func streamFrom(t *testing.T, vendor func(string) Vendor, model string, status int, body string) streamResult {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"error":{"type":"server_error","message":"fake failure"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	v := vendor(srv.URL)
	cm := NewResolvingChatModel(routedHost(t, testProfile(t, v)), NewCatalog(v), staticSpecSource{live: LiveSpec{
		Provider: v.Name, Model: model, APIKey: "secret", BaseURL: srv.URL, Ready: true,
	}})
	sr, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("weather?")})
	if err != nil {
		return streamResult{err: err}
	}
	defer sr.Close()
	var res streamResult
	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return res
		}
		if err != nil {
			res.err = err
			return res
		}
		res.text += msg.Content
		res.reasoning += msg.ReasoningContent
	}
}

func TestStreamCompleteReplyEndsCleanly(t *testing.T) {
	for name, tc := range map[string]struct {
		vendor func(string) Vendor
		model  string
		body   string
		text   string
	}{
		"openai-completions": {testDeepSeekVendor, "deepseek-flash", ccReasoningReply, "Sunny in Paris."},
		"openai tool call":   {testDeepSeekVendor, "deepseek-flash", ccToolCallReply, ""},
		"anthropic-messages": {testClaudeVendor, "claude-sonnet-4-5", anReply, "Sunny in Paris."},
	} {
		t.Run(name, func(t *testing.T) {
			res := streamFrom(t, tc.vendor, tc.model, http.StatusOK, tc.body)
			if res.err != nil || res.text != tc.text {
				t.Fatalf("stream = %q, err=%v; want %q, nil", res.text, res.err, tc.text)
			}
		})
	}
}

// KNOWN BUG — STREAM-TRUNCATION-SILENT-SUCCESS (docs/TODO.md §0.1). This test
// stays as a record of current behavior: a stream that ends before the
// model's finish signal is accepted as a short, successful reply. A guard was
// tried on 2026-10-18 and dropped, because OpenAI-compatible servers that
// never send finish_reason look exactly like a cut-off stream. When the bug is
// fixed, flip these assertions to expect an error.
func TestStreamCutOffIsAcceptedKnownBug(t *testing.T) {
	for name, tc := range map[string]struct {
		vendor func(string) Vendor
		model  string
		body   string
		text   string
	}{
		"openai-completions":         {testDeepSeekVendor, "deepseek-flash", cutBefore(ccReasoningReply, `"content":"in Paris."`), "Sunny "},
		"openai before finish chunk": {testDeepSeekVendor, "deepseek-flash", cutBefore(ccReasoningReply, `"finish_reason"`), "Sunny in Paris."},
		"anthropic-messages":         {testClaudeVendor, "claude-sonnet-4-5", cutBefore(anReply, "event: message_delta"), "Sunny in Paris."},
	} {
		t.Run(name, func(t *testing.T) {
			res := streamFrom(t, tc.vendor, tc.model, http.StatusOK, tc.body)
			if res.err != nil || res.text != tc.text {
				t.Fatalf("cut-off stream = %q, err=%v; current behavior is %q with no error (known bug)", res.text, res.err, tc.text)
			}
		})
	}
}

func TestStreamProviderErrorSurfaces(t *testing.T) {
	res := streamFrom(t, testDeepSeekVendor, "deepseek-flash", http.StatusInternalServerError, "")
	if res.err == nil {
		t.Fatalf("err = %v; want the provider's own error", res.err)
	}
}

// DEEPSEEK-REASONING-CONTENT: streamed reasoning_content reaches
// Message.ReasoningContent through the resolving model the runtime uses; the
// runtime mapper projects that field into model.reasoning_delta
// (internal/runtime/observability_test.go).
func TestDeepSeekStreamSurfacesReasoningContent(t *testing.T) {
	res := streamFrom(t, testDeepSeekVendor, "deepseek-flash", http.StatusOK, ccReasoningReply)
	if res.err != nil || res.reasoning != "think hard" || res.text != "Sunny in Paris." {
		t.Fatalf("reasoning=%q text=%q err=%v", res.reasoning, res.text, res.err)
	}
}
