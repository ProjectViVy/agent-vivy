package provider

import (
	"agent-vivy/internal/domain"
	"context"
	"encoding/json"
	"fmt"
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

func TestIssue58OutputRoutePinActualHTTP(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(raw, &body)
		requests = append(requests, body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, anthropicTestReply)
	}))
	defer server.Close()
	vendor := testClaudeVendor(server.URL)
	catalog := NewCatalog(vendor)
	live := LiveSpec{Provider: vendor.Name, Model: "claude-sonnet-4-5", APIKey: "fixture", BaseURL: server.URL, Ready: true, Endpoint: vendor.Endpoints[0]}
	cm := NewResolvingChatModel(routedHost(t, testProfile(t, vendor)), catalog, staticSpecSource{live: live})
	input := []*schema.Message{schema.UserMessage("fixture")}
	ctx := domain.WithModelOutputLimit(domain.WithThinkingMode(context.Background(), domain.ThinkingLevelMax), 4096)
	for _, pin := range []domain.ModelRequestScope{{Provider: "other", Model: live.Model}, {Provider: live.Provider, Model: "other"}} {
		if _, err := cm.Generate(domain.WithModelRequestScope(ctx, pin.Provider, pin.Model), input); err == nil || len(requests) != 0 {
			t.Fatal("mismatched admitted route reached HTTP")
		}
	}
	// Registered endpoint thinking stays under its existing metadata rule (no
	// implicit injected thinking); a limit cannot introduce or reinterpret it.
	if _, err := cm.Generate(domain.WithModelRequestScope(ctx, live.Provider, live.Model), input); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0]["max_tokens"] != float64(4096) || requests[0]["thinking"] != nil {
		t.Fatalf("custom endpoint options changed: %+v", requests)
	}
	for _, body := range requests {
		raw, _ := json.Marshal(body)
		t.Logf("HTTP_CAPTURE %s", raw)
	}
}

type issue58ChangingSpec struct {
	reads         int
	first, second LiveSpec
}

func (s *issue58ChangingSpec) Live() LiveSpec {
	s.reads++
	if s.reads == 1 {
		return s.first
	}
	return s.second
}
func TestIssue58OneLiveSpecPerInvocationActualHTTP(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(fmt.Sprint(bound), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, openAITestReply)
			}))
			defer server.Close()
			vendor := Vendor{Name: "fixture", EnvKey: "FIXTURE", Endpoints: []Endpoint{{Adapter: AdapterOpenAICompletions, BaseURL: server.URL, DefaultModel: "fixture", Models: []Model{{ID: "fixture"}}}}}
			source := &issue58ChangingSpec{first: LiveSpec{Provider: vendor.Name, Model: "fixture", APIKey: "fixture", BaseURL: server.URL, Ready: true}, second: LiveSpec{Provider: "changed", Model: "different", Ready: false}}
			cm := NewResolvingChatModel(routedHost(t, testProfile(t, vendor)), NewCatalog(vendor), source)
			if bound {
				var err error
				cm, err = cm.WithTools([]*schema.ToolInfo{{Name: "fixture_tool", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{"value": {Type: schema.String}})}})
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx := domain.WithModelRequestScope(domain.WithModelOutputLimit(context.Background(), 4096), vendor.Name, "fixture")
			if _, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("fixture")}); err != nil || calls != 1 || source.reads != 1 {
				t.Fatalf("incoherent live reads=%d HTTP=%d err=%v", source.reads, calls, err)
			}
		})
	}
}
