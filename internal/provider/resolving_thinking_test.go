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

	bundle := newClaudeTestBundle(srv.URL)
	catalog := NewCatalog(bundle)
	live := LiveSpec{Provider: bundle.Name, Model: modelID, APIKey: "spec-key", BaseURL: srv.URL, Ready: true}
	cm := NewResolvingChatModel(routedHost(t, ProfileFromBundle(bundle)), catalog, staticSpecSource{live: live})
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

	bundle := newClaudeTestBundle(srv.URL)
	live := LiveSpec{Provider: bundle.Name, Model: "claude-sonnet-4-5", APIKey: "spec-key", BaseURL: srv.URL, Ready: true}
	cm := NewResolvingChatModel(routedHost(t, ProfileFromBundle(bundle)), NewCatalog(bundle), staticSpecSource{live: live})
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
