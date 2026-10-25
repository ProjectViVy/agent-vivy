package provider

// Wire-shape golden tests. These pin the exact JSON the pinned ACL conversion
// (eino-ext/libs/acl/openai agenticMessagesToMessages) emits, because that
// mapping is unexported and is the only place the provider wire contract is
// decided. They are characterization tests: a failure means the wire changed,
// not that the new shape is wrong. The assistant content-array case is named
// as the suspect shape from the 2026-10-25 MiniMax (2013) failure.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type wireCaptureServer struct {
	mu     sync.Mutex
	body   []byte
	path   string
	status int
	reply  string
}

func newWireCaptureServer(t *testing.T, status int, reply string) (*httptest.Server, *wireCaptureServer) {
	t.Helper()
	capture := &wireCaptureServer{status: status, reply: reply}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read captured request: %v", err)
		}
		capture.mu.Lock()
		capture.body = body
		capture.path = r.URL.Path
		capture.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if capture.status != 0 {
			w.WriteHeader(capture.status)
		}
		_, _ = io.WriteString(w, capture.reply)
	}))
	t.Cleanup(server.Close)
	return server, capture
}

func (c *wireCaptureServer) captured(t *testing.T) []byte {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.body == nil {
		t.Fatal("server captured no request body")
	}
	return append([]byte(nil), c.body...)
}

// wireTestToolResult builds the user-role tool result turn the runtime feeds
// back to the model.
func wireTestToolResult(callID, name, text string) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeUser,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolResult{
				CallID: callID,
				Name:   name,
				Content: []*schema.FunctionToolResultContentBlock{
					{Type: schema.FunctionToolResultContentBlockTypeText, Text: &schema.UserInputText{Text: text}},
				},
			}),
		},
	}
}

func wireTestModel(t *testing.T, baseURL string) model.AgenticModel {
	t.Helper()
	ref := newOpenAIRef(Vendor{Name: "wire-test"}, Endpoint{
		Adapter:      AdapterOpenAICompletions,
		BaseURL:      baseURL + "/v1",
		DefaultModel: "wire-model",
	})
	chatModel, err := ref.Model(context.Background(), ModelSpec{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("construct wire model: %v", err)
	}
	return chatModel
}

type wireJSONMessage struct {
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

type wireJSONBody struct {
	Model         string            `json:"model"`
	Messages      []wireJSONMessage `json:"messages"`
	Stream        bool              `json:"stream"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
	Tools []struct {
		Type     string `json:"type"`
		Function struct {
			Name       string          `json:"name"`
			Parameters json.RawMessage `json:"parameters"`
		} `json:"function"`
	} `json:"tools"`
	ToolChoice json.RawMessage `json:"tool_choice"`
}

func decodeWireBody(t *testing.T, body []byte) wireJSONBody {
	t.Helper()
	var decoded wireJSONBody
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode captured wire body: %v\n%s", err, body)
	}
	return decoded
}

func TestWireGoldenSystemStringUserContentArray(t *testing.T) {
	server, capture := newWireCaptureServer(t, 0, openAITestReply)
	chatModel := wireTestModel(t, server.URL)

	if _, err := chatModel.Generate(context.Background(), []*schema.AgenticMessage{
		testSystem("sys"), testUser("hello"),
	}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if capture.path != "/v1/chat/completions" {
		t.Fatalf("path = %q, want /v1/chat/completions", capture.path)
	}
	body := decodeWireBody(t, capture.captured(t))
	if body.Model != "wire-model" {
		t.Fatalf("model = %q, want wire-model", body.Model)
	}
	if len(body.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(body.Messages))
	}
	if got := string(body.Messages[0].Content); got != `"sys"` {
		t.Fatalf("system content = %s, want a JSON string", got)
	}
	if got := string(body.Messages[1].Content); got != `[{"type":"text","text":"hello"}]` {
		t.Fatalf("user content = %s, want the content array form", got)
	}
}

// TestWireGoldenAssistantTextIsContentArray characterizes the current fork's
// assistant wire shape. AssistantGenText is projected onto
// AssistantGenMultiContent and therefore serializes as a content array. This is
// the prime suspect for the MiniMax (2013) rejection, but it is recorded as
// observed behavior, not blessed here.
func TestWireGoldenAssistantTextIsContentArray(t *testing.T) {
	server, capture := newWireCaptureServer(t, 0, openAITestReply)
	chatModel := wireTestModel(t, server.URL)

	if _, err := chatModel.Generate(context.Background(), []*schema.AgenticMessage{
		testUser("hi"), testAssistant("answer"), testUser("next"),
	}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body := decodeWireBody(t, capture.captured(t))
	if len(body.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(body.Messages))
	}
	assistant := body.Messages[1]
	if assistant.Role != "assistant" {
		t.Fatalf("assistant role = %q", assistant.Role)
	}
	if got := string(assistant.Content); got != `[{"type":"text","text":"answer"}]` {
		t.Fatalf("assistant content = %s, want the content array form", got)
	}
}

func TestWireGoldenAssistantToolCallsAndToolResult(t *testing.T) {
	server, capture := newWireCaptureServer(t, 0, openAITestReply)
	chatModel := wireTestModel(t, server.URL)

	assistant := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolCall{
				CallID: "call_1", Name: "echo_info", Arguments: `{"text":"x"}`,
			}),
		},
	}
	if _, err := chatModel.Generate(context.Background(), []*schema.AgenticMessage{
		testUser("hi"), assistant, wireTestToolResult("call_1", "echo_info", "ok"),
	}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body := decodeWireBody(t, capture.captured(t))
	if len(body.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(body.Messages))
	}
	call := body.Messages[1]
	if call.Role != "assistant" || len(call.ToolCalls) != 1 {
		t.Fatalf("assistant tool turn = %+v", call)
	}
	if call.ToolCalls[0].ID != "call_1" || call.ToolCalls[0].Type != "function" ||
		call.ToolCalls[0].Function.Name != "echo_info" ||
		call.ToolCalls[0].Function.Arguments != `{"text":"x"}` {
		t.Fatalf("tool call = %+v", call.ToolCalls[0])
	}
	if len(call.Content) != 0 {
		t.Fatalf("tool-call-only assistant carried content %s", call.Content)
	}
	result := body.Messages[2]
	if result.Role != "tool" || result.ToolCallID != "call_1" {
		t.Fatalf("tool result turn = %+v", result)
	}
	if got := string(result.Content); got != `"ok"` {
		t.Fatalf("tool result content = %s, want a JSON string", got)
	}
}

func TestWireGoldenToolDeclarations(t *testing.T) {
	server, capture := newWireCaptureServer(t, 0, openAITestReply)
	chatModel := wireTestModel(t, server.URL)

	tools := []*schema.ToolInfo{{
		Name: "echo_info",
		Desc: "Echoes text.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"text": {Type: schema.String, Desc: "Text to echo.", Required: true},
		}),
	}}
	if _, err := chatModel.Generate(context.Background(), []*schema.AgenticMessage{testUser("hi")},
		model.WithTools(tools)); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body := decodeWireBody(t, capture.captured(t))
	if len(body.Tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(body.Tools))
	}
	tool := body.Tools[0]
	if tool.Type != "function" || tool.Function.Name != "echo_info" {
		t.Fatalf("tool declaration = %+v", tool)
	}
	var parameters struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(tool.Function.Parameters, &parameters); err != nil {
		t.Fatalf("decode tool parameters: %v", err)
	}
	if parameters.Type != "object" || len(parameters.Properties) != 1 {
		t.Fatalf("parameters = %s", tool.Function.Parameters)
	}
	if _, ok := parameters.Properties["text"]; !ok {
		t.Fatalf("parameters lost the text property: %s", tool.Function.Parameters)
	}
	if len(parameters.Required) != 1 || parameters.Required[0] != "text" {
		t.Fatalf("required = %v, want [text]", parameters.Required)
	}
}

func TestWireGoldenStreamingRequest(t *testing.T) {
	server, capture := newWireCaptureServer(t, http.StatusBadRequest,
		`{"error":{"message":"invalid params, 400 (2013)","type":"invalid_request_error","code":2013}}`)
	chatModel := wireTestModel(t, server.URL)

	if _, err := chatModel.Stream(context.Background(), []*schema.AgenticMessage{testUser("hi")}); err == nil {
		t.Fatal("Stream unexpectedly succeeded against a 400 response")
	}
	body := decodeWireBody(t, capture.captured(t))
	if !body.Stream {
		t.Fatal("stream request did not set stream:true")
	}
	if body.StreamOptions == nil || !body.StreamOptions.IncludeUsage {
		t.Fatalf("stream_options = %+v, want include_usage:true", body.StreamOptions)
	}
}

func TestWireSummaryMatchesCapturedRequestWithoutContent(t *testing.T) {
	const nonce = "nonce-7f4c9a-do-not-leak"
	server, capture := newWireCaptureServer(t, http.StatusBadRequest,
		`{"error":{"message":"invalid params, 400 (2013)","type":"invalid_request_error","code":2013}}`)
	chatModel := wireTestModel(t, server.URL)

	var summary WireSummary
	ctx := WithWireSink(context.Background(), func(s WireSummary) { summary = s })
	assistant := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolCall{
				CallID: "call_nonce", Name: "echo_info", Arguments: fmt.Sprintf(`{"text":%q}`, nonce),
			}),
		},
	}
	tools := []*schema.ToolInfo{{
		Name: "echo_info",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"text": {Type: schema.String, Required: true},
		}),
	}}
	_, err := chatModel.Generate(ctx, []*schema.AgenticMessage{
		testSystem("sys"),
		testUser(nonce),
		assistant,
		wireTestToolResult("call_nonce", "echo_info", nonce),
	}, model.WithTools(tools))
	if err == nil {
		t.Fatal("Generate unexpectedly succeeded against a 400 response")
	}

	want := decodeWireBody(t, capture.captured(t))
	if summary.Status != http.StatusBadRequest || summary.Model != want.Model {
		t.Fatalf("summary identity = %d/%q, want 400/%q", summary.Status, summary.Model, want.Model)
	}
	if summary.Messages != len(want.Messages) {
		t.Fatalf("summary messages = %d, want %d", summary.Messages, len(want.Messages))
	}
	for i, message := range want.Messages {
		if summary.Roles[i] != message.Role {
			t.Fatalf("summary role[%d] = %q, want %q", i, summary.Roles[i], message.Role)
		}
	}
	if len(summary.CallIDs) != 1 || summary.CallIDs[0] != "call_nonce" {
		t.Fatalf("summary call ids = %v", summary.CallIDs)
	}
	if len(summary.ResultIDs) != 1 || summary.ResultIDs[0] != "call_nonce" {
		t.Fatalf("summary result ids = %v", summary.ResultIDs)
	}
	if summary.ToolCount != 1 || len(summary.ToolNames) != 1 || summary.ToolNames[0] != "echo_info" {
		t.Fatalf("summary tools = %d/%v", summary.ToolCount, summary.ToolNames)
	}
	if summary.Truncated || summary.BodyBytes != len(capture.captured(t)) {
		t.Fatalf("summary body = %d bytes truncated=%t, want %d bytes", summary.BodyBytes, summary.Truncated, len(capture.captured(t)))
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), nonce) {
		t.Fatalf("wire summary leaked message content: %s", encoded)
	}
}
