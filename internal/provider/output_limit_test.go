package provider

import (
	"agent-vivy/internal/domain"
	"context"
	"encoding/json"
	"github.com/cloudwego/eino/schema"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIssue58RequestedOutputLimitActualHTTP(t *testing.T) {
	for _, protocol := range []string{AdapterOpenAICompletions, AdapterAnthropicMessages} {
		t.Run(protocol, func(t *testing.T) {
			var bodies []map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				var body map[string]any
				json.Unmarshal(raw, &body)
				bodies = append(bodies, body)
				w.Header().Set("Content-Type", "application/json")
				if protocol == AdapterAnthropicMessages {
					io.WriteString(w, anthropicTestReply)
				} else {
					io.WriteString(w, openAITestReply)
				}
			}))
			defer server.Close()
			vendor := testClaudeVendor(server.URL)
			modelID := "claude-sonnet-4-5"
			if protocol == AdapterOpenAICompletions {
				vendor = Vendor{Name: "fixture", EnvKey: "FIXTURE", Endpoints: []Endpoint{{Adapter: protocol, BaseURL: server.URL, DefaultModel: "fixture", Models: []Model{{ID: "fixture", SupportsThinking: true}}}}}
				modelID = "fixture"
			}
			catalog := NewCatalog(vendor)
			live := LiveSpec{Provider: vendor.Name, Model: modelID, APIKey: "fixture", BaseURL: server.URL, Ready: true}
			cm := NewResolvingChatModel(routedHost(t, testProfile(t, vendor)), catalog, staticSpecSource{live: live})
			input := []*schema.Message{schema.UserMessage("fixture")}
			ctx := domain.WithThinkingMode(context.Background(), domain.ThinkingLevelLow)
			if _, err := cm.Generate(domain.WithModelOutputLimit(ctx, 4096), input); err != nil {
				t.Fatal(err)
			}
			if bodies[0]["max_tokens"] != float64(4096) {
				t.Fatalf("cap overridden: %+v", bodies[0])
			}
			if protocol == AdapterAnthropicMessages {
				thinking, _ := bodies[0]["thinking"].(map[string]any)
				if thinking["budget_tokens"] != float64(claudeThinkingBudget(domain.ThinkingLevelLow)) {
					t.Fatalf("thinking changed: %+v", bodies[0])
				}
			}
			if protocol == AdapterOpenAICompletions && bodies[0]["reasoning_effort"] != "low" {
				t.Fatalf("thinking changed: %+v", bodies[0])
			}
			if _, err := cm.Generate(ctx, input); err != nil {
				t.Fatal(err)
			}
			if protocol == AdapterAnthropicMessages && bodies[1]["max_tokens"] != float64(claudeThinkingBudget(domain.ThinkingLevelLow)+claudeDefaultMaxTokens) {
				t.Fatal("ordinary request changed")
			}
			if protocol == AdapterOpenAICompletions && bodies[1]["max_tokens"] != nil {
				t.Fatal("ordinary request acquired cap")
			}
			if protocol == AdapterAnthropicMessages {
				before := len(bodies)
				_, err := cm.Generate(domain.WithModelOutputLimit(domain.WithThinkingMode(ctx, domain.ThinkingLevelMax), 4096), input)
				if err == nil || len(bodies) != before {
					t.Fatal("incompatible thinking transmitted")
				}
			}
			for _, body := range bodies {
				raw, _ := json.Marshal(body)
				t.Logf("HTTP_CAPTURE %s", raw)
			}
		})
	}
}
