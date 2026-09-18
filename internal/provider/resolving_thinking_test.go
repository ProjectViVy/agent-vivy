package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

type staticSpecSource struct{ live LiveSpec }

func (s staticSpecSource) Live() LiveSpec { return s.live }

// thinkingBody asserts the outbound Anthropic request carries (or omits)
// the extended-thinking parameter for the given run preference.
func thinkingBody(t *testing.T, mode domain.ThinkingMode, modelID string) map[string]any {
	t.Helper()
	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, anthropicTestReply)
	}))
	defer srv.Close()

	vendor := testClaudeVendor(srv.URL)
	catalog := NewCatalog(vendor)
	live := LiveSpec{Provider: vendor.Name, Model: modelID, APIKey: "spec-key", BaseURL: srv.URL, Ready: true}
	cm := NewResolvingChatModel(routedHost(t, testProfile(t, vendor)), catalog, staticSpecSource{live: live})
	ctx := domain.WithThinkingMode(context.Background(), mode)
	if _, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode outbound body: %v", err)
	}
	return body
}

// TestResolvingModelInjectsClaudeThinking drives the resolving model
// against a local Anthropic-shaped server and asserts the D9-gated
// thinking option translation: "on" on a thinking-capable model sends the
// enabled parameter with the fixed budget; every other combination sends
// no thinking key at all.
func TestResolvingModelInjectsClaudeThinking(t *testing.T) {
	body := thinkingBody(t, domain.ThinkingModeOn, "claude-sonnet-4-5")
	thinking, ok := body["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("thinking param missing: %s", body["thinking"])
	}
	if thinking["type"] != "enabled" {
		t.Fatalf("thinking.type = %v, want enabled", thinking["type"])
	}
	if budget, ok := thinking["budget_tokens"].(float64); !ok || int(budget) != claudeThinkingBudgetTokens {
		t.Fatalf("thinking.budget_tokens = %v, want %d", thinking["budget_tokens"], claudeThinkingBudgetTokens)
	}
}

func TestResolvingModelOmitsThinkingWithoutRequest(t *testing.T) {
	for _, testCase := range []struct {
		name string
		mode domain.ThinkingMode
	}{
		{"auto", domain.ThinkingModeAuto},
		{"off", domain.ThinkingModeOff},
		{"absent", domain.ThinkingMode("")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if body := thinkingBody(t, testCase.mode, "claude-sonnet-4-5"); body["thinking"] != nil {
				t.Fatalf("thinking param sent for mode %q: %v", testCase.mode, body["thinking"])
			}
		})
	}
}

// TestDecideThinkingRuleTable pins the whole rule as data: adapter,
// capability, model metadata and the run preference decide the request shape,
// and no vendor name participates.
func TestDecideThinkingRuleTable(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		adapter      string
		deepSeek     bool
		supports     bool
		mode         domain.ThinkingMode
		wantClaude   bool
		wantDeepSeek string
		wantEffort   string
	}{
		{"reasoning model gets reasoning_effort", AdapterOpenAICompletions, false, true, domain.ThinkingModeAuto, false, "", "high"},
		{"reasoning model on explicit on", AdapterOpenAICompletions, false, true, domain.ThinkingModeOn, false, "", "high"},
		{"reasoning model on off sends nothing", AdapterOpenAICompletions, false, true, domain.ThinkingModeOff, false, "", ""},
		{"non-reasoning model sends nothing", AdapterOpenAICompletions, false, false, domain.ThinkingModeOn, false, "", ""},
		{"deepseek endpoint sends both fields", AdapterOpenAICompletions, true, true, domain.ThinkingModeOn, false, "enabled", "high"},
		{"deepseek endpoint on auto", AdapterOpenAICompletions, true, true, domain.ThinkingModeAuto, false, "enabled", "high"},
		{"deepseek endpoint on off disables", AdapterOpenAICompletions, true, true, domain.ThinkingModeOff, false, "disabled", ""},
		{"deepseek capability without metadata", AdapterOpenAICompletions, true, false, domain.ThinkingModeOn, false, "", ""},
		{"anthropic on sends the budget", AdapterAnthropicMessages, false, true, domain.ThinkingModeOn, true, "", ""},
		{"anthropic auto sends nothing", AdapterAnthropicMessages, false, true, domain.ThinkingModeAuto, false, "", ""},
		{"anthropic without metadata", AdapterAnthropicMessages, false, false, domain.ThinkingModeOn, false, "", ""},
		{"deferred adapter sends nothing", AdapterOpenAIResponses, false, true, domain.ThinkingModeOn, false, "", ""},
		{"unknown adapter sends nothing", "gemini-generate-content", false, true, domain.ThinkingModeOn, false, "", ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := decideThinking(testCase.adapter, testCase.deepSeek, testCase.supports, testCase.mode)
			if got.claudeThinking != testCase.wantClaude || got.deepSeekFlag != testCase.wantDeepSeek || got.reasoningEffort != testCase.wantEffort {
				t.Fatalf("decideThinking(%q, deepseek=%v, supports=%v, %q) = %+v, want claude=%v deepseek=%q effort=%q",
					testCase.adapter, testCase.deepSeek, testCase.supports, testCase.mode, got,
					testCase.wantClaude, testCase.wantDeepSeek, testCase.wantEffort)
			}
			options := got.options()
			want := 0
			for _, present := range []bool{got.claudeThinking, got.deepSeekFlag != "", got.reasoningEffort != ""} {
				if present {
					want++
				}
			}
			if len(options) != want {
				t.Fatalf("options() = %d options, want %d", len(options), want)
			}
		})
	}
}

// openAITestReply is the minimal OpenAI-compatible completion a fake upstream
// returns so the resolving model can be driven end to end.
const openAITestReply = `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

// openAIBody drives the resolving model against a local OpenAI-compatible
// server and returns the decoded outbound request body.
func openAIBody(t *testing.T, vendor Vendor, modelID string, mode domain.ThinkingMode) map[string]any {
	t.Helper()
	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, openAITestReply)
	}))
	defer srv.Close()

	proxied := vendor
	proxied.Endpoints[0].BaseURL = srv.URL
	live := LiveSpec{Provider: proxied.Name, Model: modelID, APIKey: "spec-key", BaseURL: srv.URL, Ready: true}
	cm := NewResolvingChatModel(routedHost(t, testProfile(t, proxied)), NewCatalog(proxied), staticSpecSource{live: live})
	ctx := domain.WithThinkingMode(context.Background(), mode)
	if _, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode outbound body: %v", err)
	}
	return body
}

// OpenAI reasoning models now receive reasoning_effort. They received nothing
// before: the per-model flag existed only for DeepSeek, and the resolver
// returned early for every other vendor, so an o1/gpt-5 request was sent with
// the provider default instead of the requested reasoning level.
func TestResolvingModelSendsReasoningEffortForOpenAIReasoningModels(t *testing.T) {
	vendor := testVendor("gateway", "Gateway", "GATEWAY_API_KEY", AdapterOpenAICompletions, "https://gateway.invalid/v1", "gpt-5", []Model{
		{ID: "gpt-5", ContextWindow: 128000, SupportsThinking: true},
		{ID: "gpt-4o", ContextWindow: 128000},
	})
	body := openAIBody(t, vendor, "gpt-5", domain.ThinkingModeAuto)
	if body["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort = %v, want high", body["reasoning_effort"])
	}
	if body["thinking"] != nil {
		t.Fatalf("a non-DeepSeek endpoint must not receive the thinking object: %v", body["thinking"])
	}
	if body["model"] != "gpt-5" {
		t.Fatalf("outbound model = %v, want the raw id", body["model"])
	}

	// The same endpoint's non-reasoning model is untouched.
	if plain := openAIBody(t, vendor, "gpt-4o", domain.ThinkingModeAuto); plain["reasoning_effort"] != nil {
		t.Fatalf("non-reasoning model received reasoning_effort: %v", plain["reasoning_effort"])
	}
}

// A DeepSeek endpoint keeps its documented canonical request: both fields, and
// an explicit disable on "off".
func TestResolvingModelKeepsDeepSeekCanonicalThinkingRequest(t *testing.T) {
	vendor := testDeepSeekVendor("https://api.deepseek.com")
	body := openAIBody(t, vendor, "deepseek-flash", domain.ThinkingModeOn)
	thinking, ok := body["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("thinking = %v, want enabled", body["thinking"])
	}
	if body["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort = %v, want high", body["reasoning_effort"])
	}

	off := openAIBody(t, vendor, "deepseek-flash", domain.ThinkingModeOff)
	disabled, ok := off["thinking"].(map[string]any)
	if !ok || disabled["type"] != "disabled" {
		t.Fatalf("thinking on off = %v, want disabled", off["thinking"])
	}
}

// TestResolvingModelThinkingGatedOnMetadata pins the D9 gate: a
// thinking-capable route only. claude-3-5-sonnet (pre-thinking) must never
// receive the parameter even on "on".
func TestResolvingModelThinkingGatedOnMetadata(t *testing.T) {
	if body := thinkingBody(t, domain.ThinkingModeOn, "claude-3-5-sonnet"); body["thinking"] != nil {
		t.Fatalf("thinking param sent to non-thinking model: %v", body["thinking"])
	}
}

// TestResolvingModelWithToolsInjectsThinking covers the bound path the
// agent loop uses: WithTools wrappers must carry the thinking option too.
func TestResolvingModelWithToolsInjectsThinking(t *testing.T) {
	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, anthropicTestReply)
	}))
	defer srv.Close()

	vendor := testClaudeVendor(srv.URL)
	live := LiveSpec{Provider: vendor.Name, Model: "claude-sonnet-4-5", APIKey: "spec-key", BaseURL: srv.URL, Ready: true}
	cm := NewResolvingChatModel(routedHost(t, testProfile(t, vendor)), NewCatalog(vendor), staticSpecSource{live: live})
	bound, err := cm.WithTools([]*schema.ToolInfo{{Name: "echo"}})
	if err != nil {
		t.Fatalf("with tools: %v", err)
	}
	ctx := domain.WithThinkingMode(context.Background(), domain.ThinkingModeOn)
	if _, err := bound.Generate(ctx, []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode outbound body: %v", err)
	}
	if body["thinking"] == nil {
		t.Fatal("thinking param missing on bound model path")
	}
}
